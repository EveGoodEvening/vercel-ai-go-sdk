package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

func TestResponseErrorStatusClassification(t *testing.T) {
	bodyVariants := []struct {
		name string
		body []byte
	}{
		{name: "known string type and code", body: []byte(`{"error":{"type":"known","code":"known"}}`)},
		{name: "unknown numeric type and code", body: []byte(`{"error":{"type":17,"code":42.5}}`)},
		{name: "malformed", body: []byte(`{"error":`)},
		{name: "empty"},
		{name: "provider shaped", body: []byte(`{"type":"provider","code":"provider"}`)},
	}
	for status := 0; status <= 600; status++ {
		want := status == 408 || status == 409 || status == 429 || status >= 500 && status <= 599
		for variantIndex, variant := range bodyVariants {
			headers := make(http.Header)
			if variantIndex%2 == 0 {
				headers.Set("Retry-After", "23")
			}
			err := composeResponseError(rawEvaluationResponse{statusCode: status, headers: headers, body: variant.body}, time.Time{})
			if err.Retryable() != want {
				t.Fatalf("status %d with %s body: Retryable() = %v, want %v", status, variant.name, err.Retryable(), want)
			}
		}
	}
}

func TestResponseErrorEnvelopeAndDiagnostics(t *testing.T) {
	rawBody := []byte(`{"error":{"message":"safe message","type":"gateway","code":12.50,"param":{"nested":[1,true]}},"generationId":"gen","requestId":"req","responseId":"res"}`)
	err := composeResponseError(rawEvaluationResponse{
		statusCode: 429,
		headers:    http.Header{"Retry-After": {"0"}},
		body:       rawBody,
	}, time.Time{})
	if err.StatusCode() != 429 || err.Message() != "safe message" || err.Type() != "gateway" || err.Code() != "12.50" {
		t.Fatalf("unexpected envelope accessors: %#v", err)
	}
	if err.GenerationID() != "gen" || err.RequestID() != "req" || err.ResponseID() != "res" {
		t.Fatalf("unexpected IDs: %q %q %q", err.GenerationID(), err.RequestID(), err.ResponseID())
	}
	if delay, ok := err.RetryAfter(); !ok || delay != 0 {
		t.Fatalf("RetryAfter() = %v, %v", delay, ok)
	}
	param := err.Param().(map[string]any)
	param["nested"].([]any)[0] = "changed"
	if err.Param().(map[string]any)["nested"].([]any)[0] == "changed" {
		t.Fatal("Param did not return a defensive copy")
	}
	first := err.RawResponseBody()
	first[0] = 'x'
	if err.RawResponseBody()[0] == 'x' {
		t.Fatal("RawResponseBody did not return a defensive copy")
	}
	assertResponseErrorFormattingExcludes(t, err, "safe message", "nested", string(rawBody))
}

func TestResponseErrorUnknownAndReadFailure(t *testing.T) {
	cause := errors.New("secret read failure")
	err := composeResponseError(rawEvaluationResponse{statusCode: 500, body: []byte("bounded secret"), bodyTruncated: true, bodyErr: cause}, time.Time{})
	if err.Message() != "" || err.Type() != "" || err.Code() != "" || err.Param() != nil {
		t.Fatal("read failure invented envelope fields")
	}
	if err.StatusCode() != 500 || !err.Retryable() || !err.BodyTruncated() || string(err.RawResponseBody()) != "bounded secret" {
		t.Fatal("read failure lost status-only classification or response diagnostics")
	}
	var transportErr *TransportError
	if !errors.As(err, &transportErr) || transportErr.Operation() != "read response body" || !errors.Is(err, cause) {
		t.Fatalf("read failure chain = %v", err)
	}
	assertResponseErrorFormattingExcludes(t, err, "secret read failure", "bounded secret")
}

func TestEvaluateNon200BodyFailuresPreserveStatusAndCauseWithoutRetry(t *testing.T) {
	readCause := errors.New("read failure marker")
	closeCause := errors.New("close failure marker")
	for _, test := range []struct {
		name      string
		newBody   func() io.ReadCloser
		wantCause error
		wantRaw   []byte
		truncated bool
	}{
		{name: "read", newBody: func() io.ReadCloser { return &faultBody{data: []byte("read prefix"), readErr: readCause} }, wantCause: readCause, wantRaw: []byte("read prefix")},
		{name: "overflow", newBody: func() io.ReadCloser { return io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("o"), (1<<20)+1))) }, wantCause: httpx.ErrResponseBodyTooLarge, wantRaw: bytes.Repeat([]byte("o"), 1<<20), truncated: true},
		{name: "close", newBody: func() io.ReadCloser { return &faultBody{data: []byte("close prefix"), closeErr: closeCause} }, wantCause: closeCause, wantRaw: []byte("close prefix")},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client, err := NewClient(WithAPIKey("credential marker"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{"Retry-After": {"7"}, "X-Preserved": {"yes"}}, Body: test.newBody(), Request: request}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Evaluate(context.Background(), "provider/model", validRequest())
			var responseErr *ResponseError
			var transportErr *TransportError
			if !errors.As(err, &responseErr) || responseErr.StatusCode() != http.StatusServiceUnavailable || !responseErr.Retryable() {
				t.Fatalf("response error = %T %v", err, err)
			}
			if !errors.As(err, &transportErr) || transportErr.Operation() != "read response body" || !errors.Is(err, test.wantCause) {
				t.Fatalf("read failure chain = %v", err)
			}
			delay, hasRetryAfter := responseErr.RetryAfter()
			if requests != 1 || delay != 7*time.Second || !hasRetryAfter || responseErr.BodyTruncated() != test.truncated || !bytes.Equal(responseErr.RawResponseBody(), test.wantRaw) {
				t.Fatalf("requests=%d RetryAfter=%v,%v truncated=%v raw length=%d", requests, delay, hasRetryAfter, responseErr.BodyTruncated(), len(responseErr.RawResponseBody()))
			}
		})
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{"12", 12 * time.Second, true},
		{"-1", 0, false},
		{"invalid", 0, false},
		{now.Add(3 * time.Second).Format(http.TimeFormat), 3 * time.Second, true},
		{now.Add(-time.Second).Format(http.TimeFormat), 0, true},
	} {
		got, ok := httpx.ParseRetryAfter(test.value, now)
		if got != test.want || ok != test.ok {
			t.Errorf("ParseRetryAfter(%q) = %v, %v; want %v, %v", test.value, got, ok, test.want, test.ok)
		}
	}
}

func TestEvaluateDispatchAndValidationOrdering(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret authorization" || request.Header.Get("X-Caller") != "caller header secret" {
			t.Errorf("unexpected request credential or caller header")
		}
		requests++
		response.Header().Set("Retry-After", "1")
		response.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(response, `{"error":{"message":"upstream body marker","param":"secret state"},"requestId":"request-id","echo":{"authorization":"secret authorization","caller":"caller header secret","provider":"provider secret"}}`)
	}))
	defer server.Close()
	client, err := NewClient(WithAPIKey("secret authorization"), WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithHeaders(http.Header{"X-Caller": {"caller header secret"}}))
	if err != nil {
		t.Fatal(err)
	}
	request := EvaluationRequest{State: "secret state", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "answer"}}, ProviderOptions: map[string]map[string]any{"p": {"secret": "provider secret"}}}
	_, err = client.Evaluate(context.Background(), "provider/model", request)
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode() != http.StatusBadGateway || responseErr.RequestID() != "request-id" {
		t.Fatalf("Evaluate error = %T %v", err, err)
	}
	secrets := []string{"secret authorization", "caller header secret", "secret state", "provider secret", "upstream body marker"}
	assertResponseErrorFormattingExcludes(t, responseErr, secrets...)
	raw := string(responseErr.RawResponseBody())
	for _, secret := range secrets {
		if !strings.Contains(raw, secret) {
			t.Fatalf("RawResponseBody omitted bounded diagnostic %q", secret)
		}
	}
	_, err = client.Evaluate(nil, "", EvaluationRequest{})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Path() != `$["context"]` || requests != 1 {
		t.Fatalf("nil context ordering: %v; requests=%d", err, requests)
	}
	_, err = client.Evaluate(context.Background(), "", EvaluationRequest{})
	if !errors.As(err, &validationErr) || requests != 1 {
		t.Fatalf("invalid request ordering: %v; requests=%d", err, requests)
	}
}

func TestResponseErrorOIDCFormattingSecrecy(t *testing.T) {
	const oidcSecret = "oidc credential secret"
	body := []byte(`{"error":{"message":"oidc envelope secret","param":{"state":"oidc state secret","provider":"oidc provider secret"}},"echo":{"authorization":"Bearer oidc credential secret","caller":"oidc caller secret"}}`)
	client, err := NewClient(WithOIDCToken(oidcSecret), WithHeaders(http.Header{"X-Caller": {"oidc caller secret"}}), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer "+oidcSecret || request.Header.Get("X-Caller") != "oidc caller secret" {
			t.Errorf("unexpected OIDC credential or caller header")
		}
		return &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), Request: request}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Evaluate(context.Background(), "provider/model", EvaluationRequest{State: "oidc state secret", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "answer"}}, ProviderOptions: map[string]map[string]any{"p": {"secret": "oidc provider secret"}}})
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) || !bytes.Equal(responseErr.RawResponseBody(), body) {
		t.Fatalf("Evaluate error = %T %v", err, err)
	}
	assertResponseErrorFormattingExcludes(t, responseErr, oidcSecret, "Bearer "+oidcSecret, "oidc caller secret", "oidc state secret", "oidc provider secret", "oidc envelope secret", string(body))
}

func assertResponseErrorFormattingExcludes(t *testing.T, err error, secrets ...string) {
	t.Helper()
	wrapped := fmt.Errorf("wrapped response: %w", err)
	surfaces := map[string]string{
		"Error()":     err.Error(),
		"%s":          fmt.Sprintf("%s", err),
		"%v":          fmt.Sprintf("%v", err),
		"%+v":         fmt.Sprintf("%+v", err),
		"%q":          fmt.Sprintf("%q", err),
		"wrapped %v":  fmt.Sprintf("%v", wrapped),
		"wrapped %+v": fmt.Sprintf("%+v", wrapped),
	}
	for surface, formatted := range surfaces {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(formatted, secret) {
				t.Fatalf("%s disclosed %q: %q", surface, secret, formatted)
			}
		}
	}
}

func TestResponseErrorZeroValues(t *testing.T) {
	for _, err := range []*ResponseError{nil, {}} {
		if err.StatusCode() != 0 || err.Message() != "" || err.Type() != "" || err.Code() != "" || err.Param() != nil || err.GenerationID() != "" || err.RequestID() != "" || err.ResponseID() != "" || err.Retryable() || err.BodyTruncated() || err.RawResponseBody() != nil || err.Unwrap() != nil {
			t.Fatalf("non-zero accessors for %#v", err)
		}
		if delay, ok := err.RetryAfter(); delay != 0 || ok {
			t.Fatalf("non-zero RetryAfter for %#v", err)
		}
	}
}
