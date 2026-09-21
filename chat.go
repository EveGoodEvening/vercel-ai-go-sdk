package gateway

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

// ChatCompletionRequest is a non-streaming Chat Completions request.
type ChatCompletionRequest struct {
	Model            string
	Messages         []ChatMessage
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	Stop             ChatStop
	SafetyIdentifier *string
	Tools            []ChatTool
	ToolChoice       ChatToolChoice
	ResponseFormat   ChatResponseFormat
	Models           []string
	ProviderOptions  *ChatProviderOptions
	Provider         *ChatProvider
}

// ChatMessage is one input message. Content must be ChatTextContent or ChatPartsContent.
type ChatMessage struct {
	Role    string
	Content ChatMessageContent
}

type ChatMessageContent interface{ chatMessageContent() }
type ChatTextContent string

func (ChatTextContent) chatMessageContent() {}

type ChatPartsContent []ChatContentPart

func (ChatPartsContent) chatMessageContent() {}

type ChatContentPart interface{ chatContentPart() }
type ChatTextPart struct{ Text string }

func (ChatTextPart) chatContentPart() {}

type ChatImageURLPart struct {
	URL    string
	Detail *string
}

func (ChatImageURLPart) chatContentPart() {}

type ChatFilePart struct {
	Data      string
	MediaType string
	Filename  string
}

func (ChatFilePart) chatContentPart() {}

type ChatStop interface{ chatStop() }
type ChatStopString string

func (ChatStopString) chatStop() {}

type ChatStopStrings []string

func (ChatStopStrings) chatStop() {}

type ChatTool struct {
	Name        string
	Description *string
	Parameters  any
}

type ChatToolChoice interface{ chatToolChoice() }
type ChatToolChoiceMode string

func (ChatToolChoiceMode) chatToolChoice() {}

const (
	ChatToolChoiceAuto ChatToolChoiceMode = "auto"
	ChatToolChoiceNone ChatToolChoiceMode = "none"
)

type ChatSpecificToolChoice struct{ Name string }

func (ChatSpecificToolChoice) chatToolChoice() {}

type ChatResponseFormat interface{ chatResponseFormat() }
type ChatResponseFormatType string

func (ChatResponseFormatType) chatResponseFormat() {}

const (
	ChatResponseFormatText ChatResponseFormatType = "text"
	ChatResponseFormatJSON ChatResponseFormatType = "json"
)

type ChatJSONSchemaResponseFormat struct {
	Name        string
	Description *string
	Schema      any
	Strict      *bool
}

func (ChatJSONSchemaResponseFormat) chatResponseFormat() {}

type ChatLegacyJSONResponseFormat struct {
	Schema      any
	Name        *string
	Description *string
}

func (ChatLegacyJSONResponseFormat) chatResponseFormat() {}

// ChatProviderOptions contains documented AI Gateway routing options.
type ChatProviderOptions struct{ Gateway ChatGatewayOptions }
type ChatGatewayOptions struct {
	Order            []string
	Models           []string
	Sort             *string
	ProviderTimeouts *ChatProviderTimeouts
}
type ChatProviderTimeouts struct{ BYOK map[string]int }
type ChatProvider struct{ Sort string }

// JSONField preserves absence, explicit null, and a non-null JSON value.
type JSONField[T any] struct {
	Present bool
	Null    bool
	Value   T
}

type ChatCompletionResult struct {
	ID      JSONField[string]
	Object  JSONField[string]
	Created JSONField[int64]
	Model   JSONField[string]
	Choices JSONField[[]ChatChoice]
	Usage   JSONField[ChatUsage]
	rawJSON []byte
}

func (r *ChatCompletionResult) RawJSON() []byte {
	if r == nil {
		return nil
	}
	out := make([]byte, len(r.rawJSON))
	copy(out, r.rawJSON)
	return out
}

type ChatChoice struct {
	Index        JSONField[int]
	Message      JSONField[ChatAssistantMessage]
	FinishReason JSONField[string]
}
type ChatAssistantMessage struct {
	Role      JSONField[string]
	Content   JSONField[string]
	ToolCalls JSONField[[]ChatToolCall]
}
type ChatToolCall struct {
	ID       JSONField[string]
	Type     JSONField[string]
	Function JSONField[ChatFunctionCall]
}
type ChatFunctionCall struct {
	Name      JSONField[string]
	Arguments JSONField[string]
}
type ChatUsage struct {
	PromptTokens     JSONField[int64]
	CompletionTokens JSONField[int64]
	TotalTokens      JSONField[int64]
}

// CreateChatCompletion validates and submits one non-streaming Chat Completions request.
func (client *Client) CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResult, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "required")
	}
	if err := validateChatCompletionRequest(request); err != nil {
		return nil, err
	}
	payload, err := encodeChatCompletionRequest(request)
	if err != nil {
		return nil, &TransportError{operation: "encode request", cause: err}
	}
	for attempt := 1; ; attempt++ {
		raw, err := client.executeChatCompletionRequest(ctx, payload)
		if err != nil {
			return nil, err
		}
		if raw.statusCode == http.StatusOK {
			if raw.bodyTruncated {
				return nil, &ResponseValidationError{cause: raw.bodyErr, statusCode: http.StatusOK, path: "$", reason: "response body exceeds 1 MiB", bodyTruncated: true, rawResponseBody: append([]byte(nil), raw.body...)}
			}
			if raw.bodyErr != nil {
				return nil, &TransportError{operation: "read response body", cause: raw.bodyErr}
			}
			return decodeChatCompletionResult(raw.body)
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

func (client *Client) executeChatCompletionRequest(ctx context.Context, payload []byte) (rawEvaluationResponse, error) {
	authorization, _, err := client.config.credential.authorization(ctx)
	if err != nil {
		return rawEvaluationResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.publicEndpoint("/chat/completions"), bytes.NewReader(payload))
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "create request", cause: err}
	}
	req.Header = client.config.buildPublicHeaders(authorization)
	httpClient := &http.Client{Transport: client.config.httpClient.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Jar: client.config.httpClient.Jar, Timeout: client.config.httpClient.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}
	capture := httpx.ReadAndClose(resp.Body)
	return rawEvaluationResponse{statusCode: resp.StatusCode, headers: resp.Header.Clone(), body: capture.Body, bodyTruncated: capture.Truncated, bodyErr: capture.Err}, nil
}
