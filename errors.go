package gateway

import (
	"fmt"
	"time"
)

// ConfigurationError reports invalid client construction options.
type ConfigurationError struct {
	option string
	reason string
}

func newConfigurationError(option, reason string) *ConfigurationError {
	return &ConfigurationError{option: option, reason: reason}
}

// Error returns a safe diagnostic that never contains credentials or payloads.
func (e *ConfigurationError) Error() string {
	if e == nil {
		return "gateway configuration error"
	}
	if e.option == "" && e.reason == "" {
		return "gateway configuration error"
	}
	return fmt.Sprintf("gateway configuration error: %s: %s", e.option, e.reason)
}

// Option returns the option that failed, or an empty string when unavailable.
func (e *ConfigurationError) Option() string {
	if e == nil {
		return ""
	}
	return e.option
}

// Reason returns the stable failure reason, or an empty string when unavailable.
func (e *ConfigurationError) Reason() string {
	if e == nil {
		return ""
	}
	return e.reason
}

// ValidationError reports an invalid evaluation request.
type ValidationError struct {
	path   string
	reason string
}

// Error returns a safe diagnostic that never contains request values.
func (e *ValidationError) Error() string {
	if e == nil {
		return "gateway validation error"
	}
	if e.path == "" && e.reason == "" {
		return "gateway validation error"
	}
	return fmt.Sprintf("gateway validation error: %s: %s", e.path, e.reason)
}

// Path returns the canonical request path, or an empty string when unavailable.
func (e *ValidationError) Path() string {
	if e == nil {
		return ""
	}
	return e.path
}

// Reason returns the stable failure reason, or an empty string when unavailable.
func (e *ValidationError) Reason() string {
	if e == nil {
		return ""
	}
	return e.reason
}

// TransportError reports a failure before a usable response body was obtained.
type TransportError struct {
	operation string
	cause     error
}

// Error returns a safe diagnostic that never includes the wrapped error text.
func (e *TransportError) Error() string {
	if e == nil || e.operation == "" {
		return "gateway transport error"
	}
	return fmt.Sprintf("gateway transport error: %s", e.operation)
}

// Operation returns the stable transport operation, or an empty string.
func (e *TransportError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Unwrap returns the underlying transport cause.
func (e *TransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// ResponseError reports a non-200 Gateway HTTP response.
type ResponseError struct {
	cause           error
	statusCode      int
	message         string
	typeName        string
	code            string
	param           any
	generationID    string
	requestID       string
	responseID      string
	retryAfter      time.Duration
	hasRetryAfter   bool
	retryable       bool
	bodyTruncated   bool
	rawResponseBody []byte
}

// Error returns a safe diagnostic that never contains raw response bytes,
// request data, headers, or credentials.
func (e *ResponseError) Error() string {
	if e == nil || e.statusCode == 0 {
		return "gateway response error"
	}
	return fmt.Sprintf("gateway response error: status %d", e.statusCode)
}

// Unwrap returns the underlying cause, if any.
func (e *ResponseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// StatusCode returns the HTTP status, or zero when unavailable.
func (e *ResponseError) StatusCode() int {
	if e == nil {
		return 0
	}
	return e.statusCode
}

// Message returns the known Gateway error message.
func (e *ResponseError) Message() string {
	if e == nil {
		return ""
	}
	return e.message
}

// Type returns the known Gateway error type.
func (e *ResponseError) Type() string {
	if e == nil {
		return ""
	}
	return e.typeName
}

// Code returns a string code or a numeric code's original JSON lexical form.
func (e *ResponseError) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

// Param returns a recursively defensive-copied JSON-compatible parameter.
func (e *ResponseError) Param() any {
	if e == nil {
		return nil
	}
	return cloneJSONValue(e.param)
}

// GenerationID returns the top-level Gateway generation ID.
func (e *ResponseError) GenerationID() string {
	if e == nil {
		return ""
	}
	return e.generationID
}

// RequestID returns the top-level Gateway request ID.
func (e *ResponseError) RequestID() string {
	if e == nil {
		return ""
	}
	return e.requestID
}

// ResponseID returns the top-level Gateway response ID.
func (e *ResponseError) ResponseID() string {
	if e == nil {
		return ""
	}
	return e.responseID
}

// RetryAfter returns the parsed Retry-After delay and whether it was valid.
func (e *ResponseError) RetryAfter() (time.Duration, bool) {
	if e == nil {
		return 0, false
	}
	return e.retryAfter, e.hasRetryAfter
}

// Retryable reports the final status-only retry classification.
func (e *ResponseError) Retryable() bool {
	if e == nil {
		return false
	}
	return e.retryable
}

// BodyTruncated reports whether retained response bytes omit a suffix.
func (e *ResponseError) BodyTruncated() bool {
	if e == nil {
		return false
	}
	return e.bodyTruncated
}

// RawResponseBody returns a fresh copy of retained response bytes. An upstream
// or intermediary may have echoed sensitive request state or provider options;
// callers must sanitize the bytes before logging or persistence.
func (e *ResponseError) RawResponseBody() []byte {
	if e == nil || e.rawResponseBody == nil {
		return nil
	}
	return append([]byte(nil), e.rawResponseBody...)
}

// ResponseValidationError reports malformed or contract-invalid status-200
// provider output.
type ResponseValidationError struct {
	cause           error
	statusCode      int
	path            string
	reason          string
	requestID       string
	responseID      string
	bodyTruncated   bool
	rawResponseBody []byte
}

// Error returns a safe diagnostic that never contains raw response bytes,
// request data, headers, or credentials.
func (e *ResponseValidationError) Error() string {
	if e == nil {
		return "gateway response validation error"
	}
	if e.path == "" && e.reason == "" {
		return "gateway response validation error"
	}
	return fmt.Sprintf("gateway response validation error: %s: %s", e.path, e.reason)
}

// Unwrap returns the underlying decoding or validation cause, if any.
func (e *ResponseValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// StatusCode returns 200 for a response validation failure, or zero when unavailable.
func (e *ResponseValidationError) StatusCode() int {
	if e == nil {
		return 0
	}
	return e.statusCode
}

// Path returns the canonical wire-response path.
func (e *ResponseValidationError) Path() string {
	if e == nil {
		return ""
	}
	return e.path
}

// Reason returns the stable response-validation reason.
func (e *ResponseValidationError) Reason() string {
	if e == nil {
		return ""
	}
	return e.reason
}

// RequestID returns the top-level Gateway request ID.
func (e *ResponseValidationError) RequestID() string {
	if e == nil {
		return ""
	}
	return e.requestID
}

// ResponseID returns the top-level Gateway response ID.
func (e *ResponseValidationError) ResponseID() string {
	if e == nil {
		return ""
	}
	return e.responseID
}

// BodyTruncated reports whether retained response bytes omit a suffix.
func (e *ResponseValidationError) BodyTruncated() bool {
	if e == nil {
		return false
	}
	return e.bodyTruncated
}

// RawResponseBody returns a fresh copy of retained response bytes. An upstream
// or intermediary may have echoed sensitive request state or provider options;
// callers must sanitize the bytes before logging or persistence.
func (e *ResponseValidationError) RawResponseBody() []byte {
	if e == nil || e.rawResponseBody == nil {
		return nil
	}
	return append([]byte(nil), e.rawResponseBody...)
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(typed))
		for key, child := range typed {
			clone[key] = cloneJSONValue(child)
		}
		return clone
	case []any:
		clone := make([]any, len(typed))
		for index, child := range typed {
			clone[index] = cloneJSONValue(child)
		}
		return clone
	default:
		return value
	}
}
