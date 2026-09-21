package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

func TestRetryPolicyValidationBoundaries(t *testing.T) {
	valid := []RetryPolicy{
		{},
		{MaxAttempts: 1},
		{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, Multiplier: 1, Jitter: 0},
		{MaxAttempts: 10, InitialDelay: time.Minute, MaxDelay: 5 * time.Minute, Multiplier: 10, Jitter: 1},
	}
	for _, policy := range valid {
		if _, err := resolveRetryPolicy(policy); err != nil {
			t.Fatalf("valid policy %#v: %v", policy, err)
		}
	}
	invalid := []RetryPolicy{
		{MaxAttempts: -1},
		{MaxAttempts: 11},
		{MaxAttempts: 2, InitialDelay: time.Millisecond - 1},
		{MaxAttempts: 2, InitialDelay: time.Minute + 1},
		{MaxAttempts: 2, MaxDelay: time.Millisecond - 1},
		{MaxAttempts: 2, MaxDelay: 5*time.Minute + 1},
		{MaxAttempts: 2, InitialDelay: 2 * time.Millisecond, MaxDelay: time.Millisecond},
		{MaxAttempts: 2, Multiplier: math.Nextafter(1, 0)},
		{MaxAttempts: 2, Multiplier: math.Nextafter(10, 11)},
		{MaxAttempts: 2, Multiplier: math.NaN()},
		{MaxAttempts: 2, Multiplier: math.Inf(1)},
		{MaxAttempts: 2, Jitter: math.Nextafter(0, -1)},
		{MaxAttempts: 2, Jitter: math.Nextafter(1, 2)},
		{MaxAttempts: 2, Jitter: math.NaN()},
		{MaxAttempts: 2, Jitter: math.Inf(1)},
	}
	for _, policy := range invalid {
		if _, err := resolveRetryPolicy(policy); err == nil {
			t.Fatalf("accepted invalid policy %#v", policy)
		}
	}
}

func TestRetryDelayFractionalMultiplierAndSaturation(t *testing.T) {
	client := &Client{config: clientConfig{
		retryPolicy: RetryPolicy{MaxAttempts: 10, InitialDelay: time.Millisecond, MaxDelay: time.Second, Multiplier: 1.0000005},
		retryHooks:  retryHooks{jitter: func() float64 { return 0 }},
	}}
	responseErr := composeResponseError(rawEvaluationResponse{statusCode: 500, headers: make(http.Header)}, time.Time{})

	if delay := client.retryDelay(3, responseErr); delay != 1_000_001*time.Nanosecond {
		t.Fatalf("fractional exponential delay = %v; want 1.000001ms", delay)
	}

	for _, test := range []struct {
		name     string
		maxDelay time.Duration
		retry    int
		want     time.Duration
	}{
		{name: "below product", maxDelay: 1_499_999 * time.Nanosecond, retry: 2, want: 1_499_999 * time.Nanosecond},
		{name: "at product", maxDelay: 1_500_000 * time.Nanosecond, retry: 2, want: 1_500_000 * time.Nanosecond},
		{name: "above product", maxDelay: 1_500_001 * time.Nanosecond, retry: 2, want: 1_500_000 * time.Nanosecond},
		{name: "huge retry number", maxDelay: 5 * time.Minute, retry: int(^uint(0) >> 1), want: 5 * time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			client.config.retryPolicy = RetryPolicy{MaxAttempts: 10, InitialDelay: time.Millisecond, MaxDelay: test.maxDelay, Multiplier: 1.5}
			if delay := client.retryDelay(test.retry, responseErr); delay != test.want {
				t.Fatalf("delay = %v; want %v", delay, test.want)
			}
		})
	}
}

func TestRetryDelayBackoffJitterAndRetryAfter(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	client := &Client{config: clientConfig{
		retryPolicy: RetryPolicy{MaxAttempts: 10, InitialDelay: 100 * time.Millisecond, MaxDelay: 750 * time.Millisecond, Multiplier: 2, Jitter: 0},
		retryHooks:  retryHooks{jitter: func() float64 { return 0 }},
	}}
	responseErr := composeResponseError(rawEvaluationResponse{statusCode: 500, headers: make(http.Header)}, now)
	var got []time.Duration
	for retry := 1; retry <= 6; retry++ {
		got = append(got, client.retryDelay(retry, responseErr))
	}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 750 * time.Millisecond, 750 * time.Millisecond, 750 * time.Millisecond}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delays = %v; want %v", got, want)
	}

	client.config.retryPolicy = RetryPolicy{MaxAttempts: 2, InitialDelay: time.Minute, MaxDelay: 5 * time.Minute, Multiplier: 10}
	if delay := client.retryDelay(10, responseErr); delay != 5*time.Minute {
		t.Fatalf("overflow-safe delay = %v", delay)
	}

	client.config.retryPolicy = RetryPolicy{MaxAttempts: 2, InitialDelay: 100 * time.Millisecond, MaxDelay: time.Second, Multiplier: 2, Jitter: .25}
	client.config.retryHooks.jitter = func() float64 { return 0 }
	if delay := client.retryDelay(1, responseErr); delay != 75*time.Millisecond {
		t.Fatalf("lower jitter endpoint = %v", delay)
	}
	client.config.retryHooks.jitter = func() float64 { return math.Nextafter(1, 0) }
	if delay := client.retryDelay(1, responseErr); delay != 125*time.Millisecond {
		t.Fatalf("upper jitter endpoint = %v", delay)
	}

	for _, test := range []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "zero", value: "0", want: 0},
		{name: "seconds", value: "3", want: 3 * time.Second},
		{name: "date", value: now.Add(4 * time.Second).Format(http.TimeFormat), want: 4 * time.Second},
		{name: "cap", value: "30", want: 5 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			client.config.retryPolicy.MaxDelay = 5 * time.Second
			err := composeResponseError(rawEvaluationResponse{statusCode: 429, headers: http.Header{"Retry-After": {test.value}}}, now)
			if delay := client.retryDelay(1, err); delay != test.want {
				t.Fatalf("delay = %v; want %v", delay, test.want)
			}
		})
	}
	for _, value := range []string{"", "malformed", "-1"} {
		err := composeResponseError(rawEvaluationResponse{statusCode: 429, headers: http.Header{"Retry-After": {value}}}, now)
		client.config.retryPolicy = RetryPolicy{MaxAttempts: 2, InitialDelay: 125 * time.Millisecond, MaxDelay: time.Second, Multiplier: 2}
		if delay := client.retryDelay(1, err); delay != 125*time.Millisecond {
			t.Fatalf("Retry-After %q delay = %v", value, delay)
		}
	}
}

func TestEvaluateRetryAttemptCountsDefaultsExhaustionAndBodies(t *testing.T) {
	for _, attempts := range []int{0, 1, 2, 10} {
		t.Run(string(rune('0'+attempts)), func(t *testing.T) {
			requests := 0
			var bodies [][]byte
			var sleeps []time.Duration
			client, err := NewClient(
				WithAPIKey("key"),
				WithRetryPolicy(RetryPolicy{MaxAttempts: attempts, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, Multiplier: 1}),
				withRetryHooks(retryHooks{now: time.Now, jitter: func() float64 { return .5 }, sleep: func(_ context.Context, delay time.Duration) error { sleeps = append(sleeps, delay); return nil }}),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					requests++
					body, readErr := io.ReadAll(request.Body)
					if readErr != nil {
						t.Fatal(readErr)
					}
					bodies = append(bodies, body)
					return &http.Response{StatusCode: 500, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"retry"}}`))), Request: request}, nil
				})}),
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Evaluate(context.Background(), "provider/model", validRequest())
			var responseErr *ResponseError
			if !errors.As(err, &responseErr) || responseErr.StatusCode() != 500 {
				t.Fatalf("error = %T %v", err, err)
			}
			wantRequests := attempts
			if wantRequests < 1 {
				wantRequests = 1
			}
			if requests != wantRequests || len(sleeps) != wantRequests-1 {
				t.Fatalf("requests=%d sleeps=%v", requests, sleeps)
			}
			for index := 1; index < len(bodies); index++ {
				if !bytes.Equal(bodies[0], bodies[index]) {
					t.Fatal("request body was not recreated consistently")
				}
			}
		})
	}
}

func TestEvaluateRetryAfterCredentialRefreshAndSuccess(t *testing.T) {
	source := &sequenceTokenSource{tokens: []string{"first", "second"}}
	attempt := 0
	var authorizations []string
	var sleeps []time.Duration
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	client, err := NewClient(
		WithOIDCTokenSource(source),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2}),
		withRetryHooks(retryHooks{now: func() time.Time { return now }, jitter: func() float64 { return .5 }, sleep: func(_ context.Context, delay time.Duration) error { sleeps = append(sleeps, delay); return nil }}),
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			attempt++
			authorizations = append(authorizations, request.Header.Get("Authorization"))
			if attempt == 1 {
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {now.Add(time.Second).Format(http.TimeFormat)}}, Body: io.NopCloser(bytes.NewReader(nil)), Request: request}, nil
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader([]byte(`{"answers":{"q":{"type":"boolean","probability":1}}}`))), Request: request}, nil
		})}),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := EvaluationRequest{State: "state", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "answer"}}}
	if _, err := client.Evaluate(context.Background(), "provider/model", request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(authorizations, []string{"Bearer first", "Bearer second"}) || !reflect.DeepEqual(sleeps, []time.Duration{time.Second}) {
		t.Fatalf("authorization=%v sleeps=%v", authorizations, sleeps)
	}
}

func TestEvaluateRetryStopsForCancellationAndNonRetryableOutcomes(t *testing.T) {
	t.Run("cancellation during wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		requests := 0
		client, err := NewClient(WithAPIKey("key"), WithRetryPolicy(RetryPolicy{MaxAttempts: 2}), withRetryHooks(retryHooks{
			now: time.Now, jitter: func() float64 { return .5 }, sleep: func(waitCtx context.Context, _ time.Duration) error { cancel(); return waitCtx.Err() },
		}), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{StatusCode: 500, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(nil)), Request: request}, nil
		})}))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Evaluate(ctx, "provider/model", validRequest())
		if !errors.Is(err, context.Canceled) || requests != 1 {
			t.Fatalf("error=%v requests=%d", err, requests)
		}
	})

	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "nonretryable status", status: 400, body: `{}`},
		{name: "response validation", status: 200, body: `{"answers":{}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests, sleeps := 0, 0
			client, err := retryTestClient(3, func(request *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader([]byte(test.body))), Request: request}, nil
			}, func(context.Context, time.Duration) error { sleeps++; return nil })
			if err != nil {
				t.Fatal(err)
			}
			_, _ = client.Evaluate(context.Background(), "provider/model", validRequest())
			if requests != 1 || sleeps != 0 {
				t.Fatalf("requests=%d sleeps=%d", requests, sleeps)
			}
		})
	}
}

func TestEvaluateNeverRetriesPreResponseFailures(t *testing.T) {
	t.Run("credential source", func(t *testing.T) {
		calls, requests, sleeps := 0, 0, 0
		source := TokenSourceFunc(func(context.Context) (string, error) {
			calls++
			return "", errors.New("credential failure")
		})
		client, err := NewClient(WithOIDCTokenSource(source), WithRetryPolicy(RetryPolicy{MaxAttempts: 3}), withRetryHooks(retryHooks{
			now: time.Now, jitter: func() float64 { return .5 }, sleep: func(context.Context, time.Duration) error { sleeps++; return nil },
		}), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return nil, errors.New("must not send")
		})}))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Evaluate(context.Background(), "provider/model", validRequest())
		var transportErr *TransportError
		if !errors.As(err, &transportErr) || transportErr.Operation() != "resolve OIDC token" || calls != 1 || requests != 0 || sleeps != 0 {
			t.Fatalf("error=%v calls=%d requests=%d sleeps=%d", err, calls, requests, sleeps)
		}
	})

	t.Run("send request", func(t *testing.T) {
		requests, sleeps := 0, 0
		client, err := retryTestClient(3, func(*http.Request) (*http.Response, error) {
			requests++
			return nil, errors.New("send failure")
		}, func(context.Context, time.Duration) error { sleeps++; return nil })
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Evaluate(context.Background(), "provider/model", validRequest())
		var transportErr *TransportError
		if !errors.As(err, &transportErr) || transportErr.Operation() != "send request" || requests != 1 || sleeps != 0 {
			t.Fatalf("error=%v requests=%d sleeps=%d", err, requests, sleeps)
		}
	})
}

type TokenSourceFunc func(context.Context) (string, error)

func (function TokenSourceFunc) Token(ctx context.Context) (string, error) {
	return function(ctx)
}

func TestEvaluateNeverRetriesResponseBodyFailures(t *testing.T) {
	readCause := errors.New("read failed")
	closeCause := errors.New("close failed")
	for _, status := range []int{200, 408, 409, 429, 500, 599} {
		for _, failure := range []struct {
			name string
			body func() io.ReadCloser
		}{
			{name: "read", body: func() io.ReadCloser { return &faultBody{readErr: readCause} }},
			{name: "overflow", body: func() io.ReadCloser { return io.NopCloser(bytes.NewReader(make([]byte, (1<<20)+1))) }},
			{name: "close", body: func() io.ReadCloser { return &faultBody{data: []byte(`{}`), closeErr: closeCause} }},
		} {
			t.Run(failure.name, func(t *testing.T) {
				requests, sleeps := 0, 0
				client, err := retryTestClient(3, func(request *http.Request) (*http.Response, error) {
					requests++
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: failure.body(), Request: request}, nil
				}, func(context.Context, time.Duration) error { sleeps++; return nil })
				if err != nil {
					t.Fatal(err)
				}
				_, err = client.Evaluate(context.Background(), "provider/model", validRequest())
				if failure.name == "overflow" && status == http.StatusOK {
					var validationErr *ResponseValidationError
					if !errors.As(err, &validationErr) {
						t.Fatalf("error = %T %v, want ResponseValidationError", err, err)
					}
					if validationErr.StatusCode() != http.StatusOK || validationErr.Path() != "$" || validationErr.Reason() != "response body exceeds 1 MiB" || !validationErr.BodyTruncated() {
						t.Fatalf("status=%d path=%q reason=%q truncated=%v", validationErr.StatusCode(), validationErr.Path(), validationErr.Reason(), validationErr.BodyTruncated())
					}
					raw := validationErr.RawResponseBody()
					if len(raw) != 1<<20 || !bytes.Equal(raw, make([]byte, 1<<20)) {
						t.Fatalf("raw body length=%d", len(raw))
					}
					if !errors.Is(err, httpx.ErrResponseBodyTooLarge) {
						t.Fatalf("overflow cause not retained: %v", err)
					}
					var transportErr *TransportError
					if errors.As(err, &transportErr) {
						t.Fatalf("overflow returned TransportError: %v", err)
					}
				} else {
					var transportErr *TransportError
					if !errors.As(err, &transportErr) || transportErr.Operation() != "read response body" {
						t.Fatalf("error = %T %v", err, err)
					}
					if status != http.StatusOK {
						var responseErr *ResponseError
						if !errors.As(err, &responseErr) || responseErr.StatusCode() != status {
							t.Fatalf("response error = %T %v", err, err)
						}
					}
				}
				if requests != 1 || sleeps != 0 {
					t.Fatalf("requests=%d sleeps=%d", requests, sleeps)
				}
			})
		}
	}
}

type sequenceTokenSource struct {
	tokens []string
	calls  int
}

func (source *sequenceTokenSource) Token(context.Context) (string, error) {
	token := source.tokens[source.calls]
	source.calls++
	return token, nil
}

func retryTestClient(attempts int, transport roundTripFunc, sleep func(context.Context, time.Duration) error) (*Client, error) {
	return NewClient(WithAPIKey("key"), WithRetryPolicy(RetryPolicy{MaxAttempts: attempts}), withRetryHooks(retryHooks{now: time.Now, jitter: func() float64 { return .5 }, sleep: sleep}), WithHTTPClient(&http.Client{Transport: transport}))
}
