package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

// ResponsesRequest is a non-streaming Responses API request.
type ResponsesRequest struct {
	Model              string
	Input              ResponseInput
	MaxOutputTokens    *int
	Temperature        *float64
	TopP               *float64
	PresencePenalty    *float64
	FrequencyPenalty   *float64
	Instructions       *string
	Tools              []ResponseTool
	ToolChoice         ResponseToolChoice
	ParallelToolCalls  *bool
	AllowedTools       []string
	Reasoning          *ResponseReasoning
	Text               *ResponseText
	Truncation         *string
	PreviousResponseID *string
	Store              *bool
	Metadata           map[string]string
	Caching            *string
	CacheAnchorItems   *int
	CacheTTL           *string
	PromptCacheKey     *string
}

type ResponseInput interface{ responseInput() }
type ResponseTextInput string

func (ResponseTextInput) responseInput() {}

type ResponseItemsInput []ResponseInputItem

func (ResponseItemsInput) responseInput() {}

type ResponseInputItem interface{ responseInputItem() }
type ResponseMessage struct {
	Role    string
	Content string
}

func (ResponseMessage) responseInputItem() {}

type ResponseFunctionCall struct {
	ID        string
	CallID    string
	Name      string
	Arguments string
}

func (ResponseFunctionCall) responseInputItem() {}

type ResponseFunctionCallOutput struct {
	CallID string
	Output string
}

func (ResponseFunctionCallOutput) responseInputItem() {}

type ResponseTool struct {
	Name        string
	Description *string
	Parameters  any
	Strict      *bool
}

type ResponseToolChoice interface{ responseToolChoice() }
type ResponseToolChoiceMode string

func (ResponseToolChoiceMode) responseToolChoice() {}

const (
	ResponseToolChoiceAuto     ResponseToolChoiceMode = "auto"
	ResponseToolChoiceRequired ResponseToolChoiceMode = "required"
	ResponseToolChoiceNone     ResponseToolChoiceMode = "none"
)

type ResponseSpecificToolChoice struct{ Name string }

func (ResponseSpecificToolChoice) responseToolChoice() {}

type ResponseReasoning struct {
	Effort  string
	Summary *string
}
type ResponseText struct{ Format ResponseTextFormat }
type ResponseTextFormat interface{ responseTextFormat() }
type ResponseTextFormatType string

func (ResponseTextFormatType) responseTextFormat() {}

const (
	ResponseTextFormatText       ResponseTextFormatType = "text"
	ResponseTextFormatJSONObject ResponseTextFormatType = "json_object"
)

type ResponseJSONSchemaFormat struct {
	Name        string
	Description *string
	Schema      any
	Strict      *bool
}

func (ResponseJSONSchemaFormat) responseTextFormat() {}

// ResponseResult retains the complete bounded status-200 JSON object.
type ResponseResult struct{ rawJSON []byte }

func (r *ResponseResult) RawJSON() []byte {
	if r == nil {
		return nil
	}
	out := make([]byte, len(r.rawJSON))
	copy(out, r.rawJSON)
	return out
}

// CreateResponse validates and submits one non-streaming Responses API request.
func (client *Client) CreateResponse(ctx context.Context, request ResponsesRequest) (*ResponseResult, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "required")
	}
	if err := validateResponsesRequest(request); err != nil {
		return nil, err
	}
	payload, err := encodeResponsesRequest(request)
	if err != nil {
		return nil, &TransportError{operation: "encode request", cause: err}
	}
	for attempt := 1; ; attempt++ {
		raw, err := client.executeResponsesRequest(ctx, payload)
		if err != nil {
			return nil, err
		}
		if raw.statusCode == http.StatusOK {
			if raw.bodyErr != nil {
				return nil, &TransportError{operation: "read response body", cause: raw.bodyErr}
			}
			return decodeRawResponseResult(raw)
		}
		now := time.Now()
		if client.config.retryHooks.now != nil {
			now = client.config.retryHooks.now()
		}
		responseErr := composeResponseError(raw, now)
		if !client.canRetry(ctx, attempt, responseErr) {
			return nil, responseErr
		}
		if err := client.waitForRetry(ctx, attempt, responseErr); err != nil {
			return nil, err
		}
	}
}

func (client *Client) executeResponsesRequest(ctx context.Context, payload []byte) (rawEvaluationResponse, error) {
	authorization, _, err := client.config.credential.authorization(ctx)
	if err != nil {
		return rawEvaluationResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.publicEndpoint("/responses"), bytes.NewReader(payload))
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "create request", cause: err}
	}
	req.Header = client.config.buildPublicHeaders(authorization)
	httpClient := &http.Client{Transport: client.config.httpClient.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Jar: client.config.httpClient.Jar, Timeout: client.config.httpClient.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}
	capture := readAndCloseResponse(ctx, resp.Body)
	return rawEvaluationResponse{statusCode: resp.StatusCode, headers: resp.Header.Clone(), body: capture.Body, bodyTruncated: capture.Truncated, bodyErr: capture.Err}, nil
}

func decodeRawResponseResult(raw rawEvaluationResponse) (*ResponseResult, error) {
	if raw.bodyTruncated {
		return nil, &ResponseValidationError{statusCode: http.StatusOK, path: "$", reason: "response body exceeds 1 MiB", bodyTruncated: true, rawResponseBody: append([]byte(nil), raw.body...)}
	}
	return decodeResponseResult(raw.body)
}

func decodeResponseResult(body []byte) (*ResponseResult, error) {
	if err := validateResponseJSON(body); err != nil {
		return nil, &ResponseValidationError{cause: err, statusCode: http.StatusOK, path: "$", reason: err.Error(), rawResponseBody: append([]byte(nil), body...)}
	}
	return &ResponseResult{rawJSON: append([]byte(nil), body...)}, nil
}

func validateResponseJSON(body []byte) error {
	if !utf8.Valid(body) {
		return fmtError("invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil {
		if err == io.EOF {
			return fmtError("empty response")
		}
		return err
	}
	if delimiter, ok := first.(json.Delim); !ok || delimiter != '{' {
		return fmtError("top-level value must be a non-null object")
	}
	if err := validateJSONTokenValue(decoder, first, 1); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmtError("trailing JSON value")
		}
		return err
	}
	return nil
}

func validateJSONTokenValue(decoder *json.Decoder, token json.Token, depth int) error {
	if depth > maxResponseJSONDepth {
		return fmtError("maximum depth exceeded")
	}
	switch value := token.(type) {
	case string:
		if len(value) > maxResponseValueBytes {
			return fmtError("string exceeds 1 MiB")
		}
		return nil
	case json.Delim:
		switch value {
		case '{':
			keys := make(map[string]struct{})
			count := 0
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmtError("object key must be a string")
				}
				if len(key) > maxResponseValueBytes {
					return fmtError("string exceeds 1 MiB")
				}
				if _, exists := keys[key]; exists {
					return fmtError("duplicate object key")
				}
				keys[key] = struct{}{}
				count++
				if count > maxResponseMembers {
					return fmtError("object exceeds 10000 members")
				}
				child, err := decoder.Token()
				if err != nil {
					return err
				}
				if err := validateJSONTokenValue(decoder, child, depth+1); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil {
				return err
			}
			if closing != json.Delim('}') {
				return fmtError("malformed object")
			}
			return nil
		case '[':
			count := 0
			for decoder.More() {
				count++
				if count > maxResponseMembers {
					return fmtError("array exceeds 10000 members")
				}
				child, err := decoder.Token()
				if err != nil {
					return err
				}
				if err := validateJSONTokenValue(decoder, child, depth+1); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil {
				return err
			}
			if closing != json.Delim(']') {
				return fmtError("malformed array")
			}
			return nil
		default:
			return fmtError("unexpected closing delimiter")
		}
	default:
		return nil
	}
}

type responseJSONError string

func (e responseJSONError) Error() string { return string(e) }
func fmtError(s string) error             { return responseJSONError(s) }
