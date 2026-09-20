package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/EveGoodEvening/vercel-ai-gateway-go-sdk/internal/httpx"
	"github.com/EveGoodEvening/vercel-ai-gateway-go-sdk/internal/testserver"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type transportTokenSource struct {
	mu    sync.Mutex
	calls int
	token string
	err   error
}

func (source *transportTokenSource) Token(context.Context) (string, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.calls++
	return source.token, source.err
}

func (source *transportTokenSource) callCount() int {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.calls
}

func TestExecuteEvaluationRequestExactWireRequest(t *testing.T) {
	clearCredentialEnvironment(t)
	fixture := testserver.New(testserver.Response{Status: http.StatusAccepted, Header: http.Header{"X-Response": {"yes"}}, Body: []byte("response")})
	defer fixture.Close()
	callerHeaders := http.Header{"X-Custom": {"first", "second"}}
	client, err := NewClient(
		WithAPIKey("secret"),
		WithBaseURL(fixture.URL+"/v4/ai///"),
		WithHeaders(callerHeaders),
		WithTeam("team"),
	)
	if err != nil {
		t.Fatal(err)
	}
	requestBody := EvaluationRequest{
		State: map[string]any{"input": "value"},
		Questions: map[string]Question{
			"quality": BooleanQuestion{Instructions: "Assess quality"},
		},
	}
	payload, err := encodeEvaluationRequest(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	const modelID = "arbitrary/provider:model?revision=one"
	outcome, err := client.executeEvaluationRequest(context.Background(), modelID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.statusCode != http.StatusAccepted || outcome.headers.Get("X-Response") != "yes" || string(outcome.body) != "response" || outcome.bodyTruncated || outcome.bodyErr != nil {
		t.Fatalf("unexpected outcome: %#v", outcome)
	}
	requests := fixture.Requests()
	if len(requests) != 1 {
		t.Fatalf("captured %d requests", len(requests))
	}
	request := requests[0]
	if request.Method != http.MethodPost || request.URL != "/v4/ai/evaluation-model" || !bytes.Equal(request.Body, payload) {
		t.Fatalf("request = %#v", request)
	}
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &encoded); err != nil {
		t.Fatal(err)
	}
	if _, present := encoded["model"]; present {
		t.Fatalf("encoded body contains model: %s", request.Body)
	}
	wantProtectedHeaders := map[string][]string{
		headerAuthorization:                       {"Bearer secret"},
		headerContentType:                         {"application/json"},
		headerGatewayProtocolVersion:              {gatewayProtocolVersion},
		headerGatewayAuthMethod:                   {"api-key"},
		headerEvaluationModelSpecificationVersion: {evaluationModelSpecificationVersion},
		headerModelID:                             {modelID},
		headerTeam:                                {"team"},
	}
	if len(wantProtectedHeaders) != len(protectedHeaderNames) {
		t.Fatalf("test covers %d protected headers, contract defines %d", len(wantProtectedHeaders), len(protectedHeaderNames))
	}
	for name := range protectedHeaderNames {
		want, covered := wantProtectedHeaders[name]
		if !covered {
			t.Fatalf("protected header %q lacks an outbound assertion", name)
		}
		if got := request.Header.Values(name); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %#v, want %#v", name, got, want)
		}
	}
	if got := request.Header.Values("X-Custom"); !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatalf("X-Custom = %#v", got)
	}
	if !reflect.DeepEqual(callerHeaders, http.Header{"X-Custom": {"first", "second"}}) {
		t.Fatalf("caller headers mutated: %#v", callerHeaders)
	}
}

func TestExecuteEvaluationRequestBaseURLNormalization(t *testing.T) {
	clearCredentialEnvironment(t)
	type observedRequest struct {
		host       string
		requestURI string
		path       string
	}
	observed := make(chan observedRequest, 1)
	fixture := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		observed <- observedRequest{
			host:       request.Host,
			requestURI: request.URL.RequestURI(),
			path:       request.URL.Path,
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer fixture.Close()

	for _, test := range []struct {
		name           string
		baseURL        string
		wantRequestURI string
		wantPath       string
	}{
		{
			name:           "origin empty path after trailing slash trim",
			baseURL:        fixture.URL + "/",
			wantRequestURI: "/evaluation-model",
			wantPath:       "/evaluation-model",
		},
		{
			name:           "port and escaping preserved",
			baseURL:        fixture.URL + "/v4%2Fai/",
			wantRequestURI: "/v4%2Fai/evaluation-model",
			wantPath:       "/v4/ai/evaluation-model",
		},
		{
			name:           "existing nontrailing path",
			baseURL:        fixture.URL + "/v4/ai",
			wantRequestURI: "/v4/ai/evaluation-model",
			wantPath:       "/v4/ai/evaluation-model",
		},
		{
			name:           "repeated trailing slashes",
			baseURL:        fixture.URL + "/v4/ai///",
			wantRequestURI: "/v4/ai/evaluation-model",
			wantPath:       "/v4/ai/evaluation-model",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(WithAPIKey("key"), WithBaseURL(test.baseURL))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.executeEvaluationRequest(context.Background(), "model", nil); err != nil {
				t.Fatal(err)
			}
			request := <-observed
			if request.host != fixture.Listener.Addr().String() || request.requestURI != test.wantRequestURI || request.path != test.wantPath {
				t.Fatalf("request host=%q URI=%q path=%q; want host=%q URI=%q path=%q", request.host, request.requestURI, request.path, fixture.Listener.Addr().String(), test.wantRequestURI, test.wantPath)
			}
		})
	}
}

func TestExecuteEvaluationRequestTokenSourceFailuresPrecedeTransport(t *testing.T) {
	for _, test := range []struct {
		name  string
		token string
		err   error
	}{
		{name: "source error", err: errors.New("token failure")},
		{name: "blank token", token: " \t"},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearCredentialEnvironment(t)
			source := &transportTokenSource{token: test.token, err: test.err}
			observed := false
			client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				observed = true
				return nil, errors.New("must not run")
			})}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.executeEvaluationRequest(context.Background(), "model", []byte("{}"))
			var transportError *TransportError
			if !errors.As(err, &transportError) || transportError.Operation() != "resolve OIDC token" || observed || source.callCount() != 1 {
				t.Fatalf("err=%v operation=%q observed=%v calls=%d", err, transportError.Operation(), observed, source.callCount())
			}
		})
	}
}

func TestExecuteEvaluationRequestAPIKeyServerFailureDoesNotInvokeOIDCSource(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "fallback-token"}
	fixture := testserver.New(testserver.Response{Status: http.StatusBadGateway, Body: []byte("upstream failure")})
	defer fixture.Close()
	client, err := NewClient(
		WithOIDCTokenSource(source),
		WithAPIKey("key"),
		WithBaseURL(fixture.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := client.executeEvaluationRequest(context.Background(), "model", nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.statusCode != http.StatusBadGateway || string(outcome.body) != "upstream failure" {
		t.Fatalf("unexpected outcome: %#v", outcome)
	}
	if source.callCount() != 0 {
		t.Fatalf("OIDC source called %d times after API-key server failure", source.callCount())
	}
}

func TestExecuteEvaluationRequestPreResponseErrors(t *testing.T) {
	clearCredentialEnvironment(t)
	client, err := NewClient(WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	client.config.baseURL = "http://[::1"
	_, err = client.executeEvaluationRequest(context.Background(), "model", nil)
	assertTransportOperation(t, err, "create request")

	cause := errors.New("send failed")
	client.config.baseURL = "http://localhost"
	client.config.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })}
	_, err = client.executeEvaluationRequest(context.Background(), "model", nil)
	assertTransportOperation(t, err, "send request")
	if !errors.Is(err, cause) {
		t.Fatal("send cause is not reachable")
	}
	client.config.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.executeEvaluationRequest(ctx, "model", nil)
	assertTransportOperation(t, err, "send request")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not reachable: %v", err)
	}
}

func TestExecuteEvaluationRequestPreservesResponseOnBodyFailures(t *testing.T) {
	readCause := errors.New("read failed")
	closeCause := errors.New("close failed")
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		for _, test := range []struct {
			name string
			body io.ReadCloser
			want error
		}{
			{name: "read", body: &faultBody{readErr: readCause}, want: readCause},
			{name: "close", body: &faultBody{data: []byte("body"), closeErr: closeCause}, want: closeCause},
			{name: "overflow", body: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), (1<<20)+1))), want: httpx.ErrResponseBodyTooLarge},
		} {
			t.Run(test.name, func(t *testing.T) {
				clearCredentialEnvironment(t)
				client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Header: http.Header{"X-Preserved": {"yes"}}, Body: test.body, Request: request}, nil
				})}))
				if err != nil {
					t.Fatal(err)
				}
				outcome, err := client.executeEvaluationRequest(context.Background(), "model", nil)
				if err != nil || outcome.statusCode != status || outcome.headers.Get("X-Preserved") != "yes" || !errors.Is(outcome.bodyErr, test.want) {
					t.Fatalf("outcome=%#v err=%v", outcome, err)
				}
				if test.name == "overflow" && (!outcome.bodyTruncated || len(outcome.body) != 1<<20) {
					t.Fatalf("overflow capture length=%d truncated=%v", len(outcome.body), outcome.bodyTruncated)
				}
			})
		}
	}
}

type faultBody struct {
	data     []byte
	readErr  error
	closeErr error
	read     bool
}

func TestHermeticTransportAllowsLoopbackAndRejectsGateway(t *testing.T) {
	clearCredentialEnvironment(t)
	originalDefaultTransport := http.DefaultTransport
	guard, ok := originalDefaultTransport.(loopbackOnlyTransport)
	if !ok {
		t.Fatalf("default transport = %T, want loopbackOnlyTransport", originalDefaultTransport)
	}
	delegateCalls := 0
	originalDelegate := guard.base
	guard.base = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		delegateCalls++
		return originalDelegate.RoundTrip(request)
	})
	http.DefaultTransport = guard
	t.Cleanup(func() { http.DefaultTransport = originalDefaultTransport })

	fixture := testserver.New(testserver.Response{})
	defer fixture.Close()
	client, err := NewClient(WithAPIKey("key"), WithBaseURL(fixture.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.executeEvaluationRequest(context.Background(), "model", nil); err != nil {
		t.Fatalf("loopback request failed: %v", err)
	}
	if delegateCalls != 1 {
		t.Fatalf("loopback delegate calls = %d, want 1", delegateCalls)
	}

	client, err = NewClient(WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.executeEvaluationRequest(context.Background(), "model", nil)
	assertTransportOperation(t, err, "send request")
	if delegateCalls != 1 {
		t.Fatalf("rejected Gateway request delegated; calls = %d, want 1", delegateCalls)
	}
}

func TestExecuteEvaluationRequestConcurrentTokenSource(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(nil)), Request: request}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	const count = 16
	var wait sync.WaitGroup
	wait.Add(count)
	errorsSeen := make(chan error, count)
	for range count {
		go func() {
			defer wait.Done()
			_, err := client.executeEvaluationRequest(context.Background(), "model", []byte("{}"))
			errorsSeen <- err
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	if source.callCount() != count {
		t.Fatalf("source called %d times, want %d", source.callCount(), count)
	}
}

func (body *faultBody) Read(destination []byte) (int, error) {
	if body.read {
		if body.readErr != nil {
			return 0, body.readErr
		}
		return 0, io.EOF
	}
	body.read = true
	return copy(destination, body.data), body.readErr
}

func (body *faultBody) Close() error { return body.closeErr }

func assertTransportOperation(t *testing.T, err error, operation string) {
	t.Helper()
	var transportError *TransportError
	if !errors.As(err, &transportError) || transportError.Operation() != operation {
		t.Fatalf("error = %v, want TransportError operation %q", err, operation)
	}
}
