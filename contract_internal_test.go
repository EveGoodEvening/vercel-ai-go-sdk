package gateway

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
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

func TestProviderOptionsOmitNilAndEmpty(t *testing.T) {
	if got := encodeProviderOptions(nil); got != nil {
		t.Fatalf("nil options encoded as %#v", got)
	}
	if got := encodeProviderOptions([]ProviderOption{}); got != nil {
		t.Fatalf("empty options encoded as %#v", got)
	}
}

func TestModalityTransportExactRoutesAndHeaders(t *testing.T) {
	clearCredentialEnvironment(t)
	tests := []struct {
		route  string
		header string
	}{
		{providerRouteEmbedding, headerEmbeddingModelSpecificationVersion},
		{providerRouteReranking, headerRerankingModelSpecificationVersion},
		{providerRouteImage, headerImageModelSpecificationVersion},
		{providerRouteSpeech, headerSpeechModelSpecificationVersion},
		{providerRouteTranscription, headerTranscriptionModelSpecificationVersion},
		{providerRouteLanguage, headerLanguageModelSpecificationVersion},
	}
	for _, test := range tests {
		t.Run(test.route, func(t *testing.T) {
			var request *http.Request
			client, err := NewClient(WithAPIKey("secret"), WithBaseURL("https://example.test/v4/ai"), WithTeam("team"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(got *http.Request) (*http.Response, error) {
				request = got
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), ContentLength: 2}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := client.executeProviderRequest(context.Background(), test.route, "provider/model", func() ([]byte, error) { return []byte(`{"input":1}`), nil }, 32)
			if err != nil || string(outcome.body) != "{}" {
				t.Fatalf("outcome=%#v err=%v", outcome, err)
			}
			if request.URL.String() != "https://example.test/v4/ai"+test.route || request.Method != http.MethodPost {
				t.Fatalf("request = %s %s", request.Method, request.URL)
			}
			for name, want := range map[string]string{
				headerAuthorization: "Bearer secret", headerContentType: "application/json", headerGatewayProtocolVersion: gatewayProtocolVersion,
				headerGatewayAuthMethod: "api-key", headerModelID: "provider/model", headerTeam: "team", test.header: providerModelSpecificationVersion,
			} {
				if got := request.Header.Get(name); got != want {
					t.Fatalf("%s=%q want %q", name, got, want)
				}
			}
			if request.Header.Get(headerEvaluationModelSpecificationVersion) != "" {
				t.Fatal("modality request included evaluation specification header")
			}
		})
	}
}

func TestModalityTransportValidationBeforeCredentialAndOneAttempt(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	attempts := 0
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("failure"))}, nil
	})}), WithRetryPolicy(RetryPolicy{MaxAttempts: 10}))
	if err != nil {
		t.Fatal(err)
	}
	want := validationError(`$["value"]`, "invalid")
	_, err = client.executeProviderRequest(context.Background(), providerRouteEmbedding, "model", func() ([]byte, error) { return nil, want }, 32)
	if err != want || source.callCount() != 0 || attempts != 0 {
		t.Fatalf("err=%v credentials=%d attempts=%d", err, source.callCount(), attempts)
	}
	outcome, err := client.executeProviderRequest(context.Background(), providerRouteEmbedding, "model", func() ([]byte, error) { return []byte("{}"), nil }, 32)
	if err != nil || outcome.statusCode != http.StatusInternalServerError || attempts != 1 || source.callCount() != 1 {
		t.Fatalf("outcome=%#v err=%v credentials=%d attempts=%d", outcome, err, source.callCount(), attempts)
	}
}

func TestModalityTransportDistinctBodyLimitsAndSafeError(t *testing.T) {
	clearCredentialEnvironment(t)
	responses := []*http.Response{
		{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("12345")), ContentLength: 5},
		{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxDiagnosticBodyBytes+1))), ContentLength: maxDiagnosticBodyBytes + 1},
	}
	client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		response := responses[0]
		responses = responses[1:]
		return response, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.executeProviderRequest(context.Background(), providerRouteEmbedding, "model", func() ([]byte, error) { return []byte("{}"), nil }, 4)
	var transportErr *TransportError
	if !errors.As(err, &transportErr) || transportErr.Operation() != "read response body" || !errors.Is(err, httpx.ErrResponseBodyTooLarge) || strings.Contains(err.Error(), "12345") {
		t.Fatalf("unsafe or untyped success overflow error: %T %v", err, err)
	}
	outcome, err := client.executeProviderRequest(context.Background(), providerRouteEmbedding, "model", func() ([]byte, error) { return []byte("{}"), nil }, 4)
	if err != nil || len(outcome.body) != maxDiagnosticBodyBytes || !outcome.bodyTruncated || !errors.Is(outcome.bodyErr, httpx.ErrResponseBodyTooLarge) {
		t.Fatalf("diagnostic outcome=%#v err=%v", outcome, err)
	}
}

type cancellationBody struct {
	ctx    context.Context
	once   sync.Once
	closed chan struct{}
	mu     sync.Mutex
	closes int
}

func (body *cancellationBody) Read([]byte) (int, error) {
	<-body.ctx.Done()
	return 0, body.ctx.Err()
}

func (body *cancellationBody) Close() error {
	body.once.Do(func() { close(body.closed) })
	body.mu.Lock()
	body.closes++
	body.mu.Unlock()
	return nil
}

func TestModalityTransportCancellationClosesExactlyOnce(t *testing.T) {
	clearCredentialEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	body := &cancellationBody{ctx: ctx, closed: make(chan struct{})}
	client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body, ContentLength: -1}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, callErr := client.executeProviderRequest(ctx, providerRouteEmbedding, "model", func() ([]byte, error) { return []byte("{}"), nil }, 32)
		done <- callErr
	}()
	cancel()
	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not reachable: %v", err)
	}
	body.mu.Lock()
	closes := body.closes
	body.mu.Unlock()
	if closes != 1 {
		t.Fatalf("close calls=%d want 1", closes)
	}
}

func TestModalityTransportRefusesRedirectAndOwnsResponse(t *testing.T) {
	clearCredentialEnvironment(t)
	attempts := 0
	responseHeaders := http.Header{"Location": {"https://other.test/stolen"}, "X-Test": {"original"}}
	client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		if attempts > 1 {
			t.Fatal("redirect was followed")
		}
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     responseHeaders,
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Request:    request,
		}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := client.executeProviderRequest(context.Background(), providerRouteImage, "model", func() ([]byte, error) { return []byte("{}"), nil }, 32)
	if err != nil || attempts != 1 || outcome.statusCode != http.StatusTemporaryRedirect || string(outcome.body) != "redirect" {
		t.Fatalf("outcome=%#v err=%v attempts=%d", outcome, err, attempts)
	}
	responseHeaders.Set("X-Test", "changed")
	if outcome.headers.Get("X-Test") != "original" {
		t.Fatalf("response headers were not owned: %#v", outcome.headers)
	}
	outcome.body[0] = 'R'
	if string(outcome.body) != "Redirect" {
		t.Fatal("response body was not independently mutable")
	}
}

func TestModalityTransportSpecificationHeadersAreProtected(t *testing.T) {
	clearCredentialEnvironment(t)
	for name := range providerProtectedHeaderNames {
		_, err := NewClient(WithAPIKey("secret"), WithHeaders(http.Header{strings.ToLower(name): nil}))
		var configurationErr *ConfigurationError
		if !errors.As(err, &configurationErr) || configurationErr.Option() != "WithHeaders" || configurationErr.Reason() != "contains protected header" {
			t.Fatalf("header %q error = %T %v", name, err, err)
		}
	}
}
