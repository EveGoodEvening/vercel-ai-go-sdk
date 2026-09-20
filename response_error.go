package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

type responseErrorEnvelope struct {
	Error        responseErrorDetails
	GenerationID string
	RequestID    string
	ResponseID   string
}

type responseErrorDetails struct {
	Message string
	Type    string
	Code    string
	Param   any
}

func composeResponseError(raw rawEvaluationResponse, now time.Time) *ResponseError {
	retryAfter, hasRetryAfter := httpx.ParseRetryAfter(raw.headers.Get("Retry-After"), now)
	responseErr := &ResponseError{
		statusCode:      raw.statusCode,
		retryAfter:      retryAfter,
		hasRetryAfter:   hasRetryAfter,
		retryable:       retryableStatus(raw.statusCode),
		bodyTruncated:   raw.bodyTruncated,
		rawResponseBody: append([]byte(nil), raw.body...),
	}
	if raw.bodyErr != nil {
		responseErr.cause = &TransportError{operation: "read response body", cause: raw.bodyErr}
		return responseErr
	}
	if envelope, ok := parseResponseErrorEnvelope(raw.body); ok {
		responseErr.message = envelope.Error.Message
		responseErr.typeName = envelope.Error.Type
		responseErr.code = envelope.Error.Code
		responseErr.param = envelope.Error.Param
		responseErr.generationID = envelope.GenerationID
		responseErr.requestID = envelope.RequestID
		responseErr.responseID = envelope.ResponseID
	}
	return responseErr
}

func retryableStatus(status int) bool {
	return status == 408 || status == 409 || status == 429 || status >= 500 && status <= 599
}

func parseResponseErrorEnvelope(body []byte) (responseErrorEnvelope, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return responseErrorEnvelope{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return responseErrorEnvelope{}, false
	}
	errorObject, ok := root["error"].(map[string]any)
	if !ok {
		return responseErrorEnvelope{}, false
	}
	envelope := responseErrorEnvelope{
		GenerationID: jsonString(root["generationId"]),
		RequestID:    jsonString(root["requestId"]),
		ResponseID:   jsonString(root["responseId"]),
	}
	envelope.Error.Message = jsonString(errorObject["message"])
	envelope.Error.Type = jsonString(errorObject["type"])
	switch code := errorObject["code"].(type) {
	case string:
		envelope.Error.Code = code
	case json.Number:
		envelope.Error.Code = code.String()
	}
	if param, exists := errorObject["param"]; exists && param != nil {
		envelope.Error.Param = param
	}
	return envelope, true
}

func jsonString(value any) string {
	text, _ := value.(string)
	return text
}
