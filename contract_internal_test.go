package gateway

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestErrorNilAndZeroAccessors(t *testing.T) {
	var configuration *ConfigurationError
	if configuration.Error() != "gateway configuration error" || configuration.Option() != "" || configuration.Reason() != "" {
		t.Fatal("ConfigurationError nil receiver returned non-zero accessor")
	}
	var validation *ValidationError
	if validation.Error() != "gateway validation error" || validation.Path() != "" || validation.Reason() != "" {
		t.Fatal("ValidationError nil receiver returned non-zero accessor")
	}
	var transport *TransportError
	if transport.Error() != "gateway transport error" || transport.Operation() != "" || transport.Unwrap() != nil {
		t.Fatal("TransportError nil receiver returned non-zero accessor")
	}
	var response *ResponseError
	if response.Error() != "gateway response error" || response.Unwrap() != nil || response.StatusCode() != 0 || response.Message() != "" || response.Type() != "" || response.Code() != "" || response.Param() != nil || response.GenerationID() != "" || response.RequestID() != "" || response.ResponseID() != "" || response.Retryable() || response.BodyTruncated() || response.RawResponseBody() != nil {
		t.Fatal("ResponseError nil receiver returned non-zero accessor")
	}
	if delay, ok := response.RetryAfter(); delay != 0 || ok {
		t.Fatal("ResponseError nil RetryAfter returned presence")
	}
	var responseValidation *ResponseValidationError
	if responseValidation.Error() != "gateway response validation error" || responseValidation.Unwrap() != nil || responseValidation.StatusCode() != 0 || responseValidation.Path() != "" || responseValidation.Reason() != "" || responseValidation.RequestID() != "" || responseValidation.ResponseID() != "" || responseValidation.BodyTruncated() || responseValidation.RawResponseBody() != nil {
		t.Fatal("ResponseValidationError nil receiver returned non-zero accessor")
	}
}

func TestConfigurationErrorClosedValues(t *testing.T) {
	cases := []struct {
		option string
		reason string
	}{
		{"WithBaseURL", "must not be empty or whitespace"},
		{"WithBaseURL", "must be an absolute http or https URL"},
		{"WithBaseURL", "must not contain userinfo, query, or fragment"},
		{"WithHTTPClient", "must not be nil"},
		{"WithAPIKey", "must not be empty or whitespace"},
		{"WithOIDCToken", "must not be empty or whitespace"},
		{"WithOIDCTokenSource", "must not be nil"},
		{"WithTeam", "must not be empty or whitespace"},
		{"WithHeaders", "contains protected header"},
		{"WithRetryPolicy", "invalid MaxAttempts"},
		{"WithRetryPolicy", "invalid InitialDelay"},
		{"WithRetryPolicy", "invalid MaxDelay"},
		{"WithRetryPolicy", "MaxDelay is less than InitialDelay"},
		{"WithRetryPolicy", "invalid Multiplier"},
		{"WithRetryPolicy", "invalid Jitter"},
		{"credentials", "no credential configured"},
	}
	for _, test := range cases {
		err := newConfigurationError(test.option, test.reason)
		if err.Option() != test.option || err.Reason() != test.reason {
			t.Fatalf("unexpected configuration accessor pair: %q, %q", err.Option(), err.Reason())
		}
	}
}

func TestTransportErrorClosedValues(t *testing.T) {
	cause := errors.New("cause")
	for _, operation := range []string{"resolve OIDC token", "encode request", "create request", "send request", "read response body"} {
		err := &TransportError{operation: operation, cause: cause}
		if err.Operation() != operation || !errors.Is(err, cause) {
			t.Fatalf("unexpected transport error for %q", operation)
		}
	}
}

func TestBaseURLRejectsBoundaryUnicodeWhitespace(t *testing.T) {
	for _, baseURL := range []string{
		"\u00a0https://example.com/v4/ai",
		"https://example.com/v4/ai\u00a0",
	} {
		_, err := NewClient(WithBaseURL(baseURL))
		var configurationError *ConfigurationError
		if !errors.As(err, &configurationError) {
			t.Fatalf("expected ConfigurationError for %q, got %v", baseURL, err)
		}
		if configurationError.Option() != "WithBaseURL" || configurationError.Reason() != "must not be empty or whitespace" {
			t.Fatalf("unexpected base URL error for %q: %q, %q", baseURL, configurationError.Option(), configurationError.Reason())
		}
	}
}

func TestRetryPolicyResolution(t *testing.T) {
	resolved, err := resolveRetryPolicy(RetryPolicy{MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.MaxAttempts != 2 || resolved.InitialDelay != 100*time.Millisecond || resolved.MaxDelay != 2*time.Second || resolved.Multiplier != 2 || resolved.Jitter != 0 {
		t.Fatalf("unexpected retry defaults: %#v", resolved)
	}

	invalid := []RetryPolicy{
		{MaxAttempts: -1},
		{MaxAttempts: 11},
		{MaxAttempts: 2, InitialDelay: time.Nanosecond},
		{MaxAttempts: 2, MaxDelay: time.Nanosecond},
		{MaxAttempts: 2, InitialDelay: time.Second, MaxDelay: time.Millisecond},
		{MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Millisecond},
		{MaxAttempts: 2, Multiplier: math.NaN()},
		{MaxAttempts: 2, Jitter: math.Inf(1)},
	}
	for _, policy := range invalid {
		if _, err := resolveRetryPolicy(policy); err == nil {
			t.Fatalf("expected invalid retry policy: %#v", policy)
		}
	}
}

func TestRetryHooksRemainPrivateShape(t *testing.T) {
	hooks := retryHooks{}
	_ = withRetryHooks(hooks)
}
