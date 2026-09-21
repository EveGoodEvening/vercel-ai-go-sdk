package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSharedSSEFramingLimits exercises the byte boundaries independently of
// either public streaming surface. Those surfaces share this parser, so a
// boundary regression here would affect both Responses and Chat Completions.
func TestSharedSSEFramingLimits(t *testing.T) {
	lineAtLimit := "data:" + strings.Repeat("x", maxSSELineBytes-len("data:"))
	lineOverLimit := lineAtLimit + "x"

	var eventAtLimit strings.Builder
	remaining := maxSSEEventBytes
	for remaining > 0 {
		// Account for the newline inserted between data fields.
		if eventAtLimit.Len() != 0 {
			remaining--
		}
		n := min(remaining, maxSSELineBytes-len("data:"))
		eventAtLimit.WriteString("data:")
		eventAtLimit.WriteString(strings.Repeat("x", n))
		eventAtLimit.WriteByte('\n')
		remaining -= n
	}
	eventAtLimit.WriteByte('\n')

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr error
	}{
		{name: "LF line at limit", input: lineAtLimit + "\n\n", wantLen: len(lineAtLimit) - len("data:")},
		{name: "CRLF line at limit", input: lineAtLimit + "\r\n\r\n", wantLen: len(lineAtLimit) - len("data:")},
		{name: "line limit plus one", input: lineOverLimit + "\n\n", wantErr: errSSELineTooLarge},
		{name: "event at limit", input: eventAtLimit.String(), wantLen: maxSSEEventBytes},
		{name: "event limit plus one", input: eventAtLimit.String()[:len(eventAtLimit.String())-1] + "data:x\n\n", wantErr: errSSEEventTooLarge},
		{name: "unterminated line", input: "data:x", wantErr: errSSETruncated},
		{name: "unterminated event", input: "data:x\n", wantErr: errSSETruncated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := newSSEParser(strings.NewReader(test.input))
			event, err := parser.next()
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && len(event.data) != test.wantLen {
				t.Fatalf("data length = %d, want %d", len(event.data), test.wantLen)
			}
			if cap(parser.data) > maxSSEEventBytes || cap(event.data) > maxSSEEventBytes {
				t.Fatalf("retained event capacity = %d/%d, exceeds %d", cap(parser.data), cap(event.data), maxSSEEventBytes)
			}
		})
	}
}

func TestSharedSSEDeterministicFieldSequences(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantID    string
		wantData  string
		secondID  string
		secondEOF bool
	}{
		{name: "defaults and joins data", input: "data:a\ndata: b\n\n", wantName: "message", wantData: "a\nb", secondEOF: true},
		{name: "event and id", input: "event:custom\nid: 42\ndata:x\n\n", wantName: "custom", wantID: "42", wantData: "x", secondEOF: true},
		{name: "comment and unknown fields", input: ":comment\nretry:1\ndata:x\n\n", wantName: "message", wantData: "x", secondEOF: true},
		{name: "id persists", input: "id:7\ndata:a\n\ndata:b\n\n", wantName: "message", wantID: "7", wantData: "a", secondID: "7"},
		{name: "NUL id ignored", input: "id:old\nid:new\x00bad\ndata:x\n\n", wantName: "message", wantID: "old", wantData: "x", secondEOF: true},
		{name: "empty data is an event", input: "data\n\n", wantName: "message", wantData: "", secondEOF: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := newSSEParser(strings.NewReader(test.input))
			event, err := parser.next()
			if err != nil {
				t.Fatal(err)
			}
			if event.name != test.wantName || event.id != test.wantID || string(event.data) != test.wantData {
				t.Fatalf("event = %#v", event)
			}
			second, err := parser.next()
			if test.secondEOF {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("second error = %v, want EOF", err)
				}
				return
			}
			if err != nil || second.id != test.secondID || string(second.data) != "b" {
				t.Fatalf("second = %#v, %v", second, err)
			}
		})
	}
}

func TestSharedStreamLimitConstantsAndSafeDiagnostics(t *testing.T) {
	if maxChatCompletionStreamEvents != maxResponseStreamEvents {
		t.Fatalf("stream event limits diverged: chat=%d responses=%d", maxChatCompletionStreamEvents, maxResponseStreamEvents)
	}
	secret := "sensitive-payload-marker"
	for _, err := range []error{
		&TransportError{operation: "read response stream", cause: errors.New(secret)},
		&TransportError{operation: "read chat completion stream", cause: errors.New(secret)},
	} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("formatted error exposed cause: %q", err)
		}
	}

	// Exercise fragmented reads as a deterministic fuzz-style check that parser
	// limits are based on framing bytes rather than an io.Reader's chunk sizes.
	wire := []byte("event:delta\nid:1\ndata:{\"x\":1}\n\n")
	for chunk := 1; chunk <= len(wire); chunk++ {
		reader := &fixedChunkReader{data: append([]byte(nil), wire...), chunk: chunk}
		event, err := newSSEParser(reader).next()
		if err != nil || event.name != "delta" || event.id != "1" || !bytes.Equal(event.data, []byte(`{"x":1}`)) {
			t.Fatalf("chunk %d: event=%#v error=%v", chunk, event, err)
		}
	}
}

func TestCancellationUnblocksBufferedPublicBodyReads(t *testing.T) {
	tests := []struct {
		name   string
		status int
		call   func(*Client, context.Context) error
	}{
		{name: "Chat success", status: http.StatusOK, call: func(client *Client, ctx context.Context) error {
			_, err := client.CreateChatCompletion(ctx, validChatRequest())
			return err
		}},
		{name: "Chat non-200", status: http.StatusServiceUnavailable, call: func(client *Client, ctx context.Context) error {
			_, err := client.CreateChatCompletion(ctx, validChatRequest())
			return err
		}},
		{name: "Responses success", status: http.StatusOK, call: func(client *Client, ctx context.Context) error {
			_, err := client.CreateResponse(ctx, validResponsesRequest())
			return err
		}},
		{name: "Responses non-200", status: http.StatusServiceUnavailable, call: func(client *Client, ctx context.Context) error {
			_, err := client.CreateResponse(ctx, validResponsesRequest())
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := newBlockedBufferedBody()
			var requests atomic.Int32
			var retryWaits atomic.Int32
			client, err := NewClient(
				WithAPIKey("key"),
				WithRetryPolicy(RetryPolicy{MaxAttempts: 3}),
				withRetryHooks(retryHooks{now: time.Now, jitter: func() float64 { return 0 }, sleep: func(context.Context, time.Duration) error {
					retryWaits.Add(1)
					return nil
				}}),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					requests.Add(1)
					return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: body, Request: request}, nil
				})}),
			)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() { result <- test.call(client, ctx) }()
			select {
			case <-body.readStarted:
			case <-time.After(time.Second):
				t.Fatal("buffered body read did not start")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %T %v, want context cancellation", err, err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not unblock buffered body read")
			}
			if got := body.closeCalls.Load(); got != 1 {
				t.Fatalf("body close calls = %d, want 1", got)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1 (no retry or replay after response receipt)", got)
			}
			if got := retryWaits.Load(); got != 0 {
				t.Fatalf("retry waits = %d, want 0 after response receipt cancellation", got)
			}
		})
	}
}

func TestPreCanceledPublicGenerationSkipsCredentialAndTransport(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client, context.Context) error
	}{
		{name: "CreateChatCompletion", call: func(client *Client, ctx context.Context) error {
			_, err := client.CreateChatCompletion(ctx, validChatRequest())
			return err
		}},
		{name: "CreateResponse", call: func(client *Client, ctx context.Context) error {
			_, err := client.CreateResponse(ctx, validResponsesRequest())
			return err
		}},
		{name: "StreamChatCompletion", call: func(client *Client, ctx context.Context) error {
			_, err := client.StreamChatCompletion(ctx, validChatRequest())
			return err
		}},
		{name: "StreamResponse", call: func(client *Client, ctx context.Context) error {
			_, err := client.StreamResponse(ctx, validResponsesRequest())
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearCredentialEnvironment(t)
			source := &transportTokenSource{token: "must-not-be-resolved"}
			var requests atomic.Int32
			client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return nil, errors.New("must not send")
			})}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err = test.call(client, ctx)
			var transportErr *TransportError
			if !errors.As(err, &transportErr) || transportErr.Operation() != "send request" || !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %T %v, want send-request TransportError wrapping context.Canceled", err, err)
			}
			if got := source.callCount(); got != 0 {
				t.Fatalf("token source calls = %d, want 0", got)
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("transport calls = %d, want 0", got)
			}
		})
	}
}

func TestCancellationUnblocksStreamingNon200BodyReads(t *testing.T) {
	prefix := []byte(`{"error":{"message":"safe message","type":"gateway_error","code":"blocked"},"requestId":"request-id","responseId":"response-id","generationId":"generation-id"}`)
	tests := []struct {
		name string
		call func(*Client, context.Context) error
	}{
		{name: "StreamChatCompletion", call: func(client *Client, ctx context.Context) error {
			_, err := client.StreamChatCompletion(ctx, validChatRequest())
			return err
		}},
		{name: "StreamResponse", call: func(client *Client, ctx context.Context) error {
			_, err := client.StreamResponse(ctx, validResponsesRequest())
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := newBlockedBufferedBodyWithPrefix(prefix)
			var requests atomic.Int32
			var retryWaits atomic.Int32
			client, err := NewClient(
				WithAPIKey("key"),
				WithRetryPolicy(RetryPolicy{MaxAttempts: 3}),
				withRetryHooks(retryHooks{now: time.Now, jitter: func() float64 { return 0 }, sleep: func(context.Context, time.Duration) error { retryWaits.Add(1); return nil }}),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					requests.Add(1)
					return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{"Retry-After": {"7"}, "X-Preserved": {"yes"}}, Body: body, Request: request}, nil
				})}),
			)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() { result <- test.call(client, ctx) }()
			select {
			case <-body.readStarted:
			case <-time.After(time.Second):
				t.Fatal("streaming error body read did not block")
			}
			cancel()
			var callErr error
			select {
			case callErr = <-result:
			case <-time.After(time.Second):
				t.Fatal("cancellation did not unblock streaming error body read")
			}
			if !errors.Is(callErr, context.Canceled) {
				t.Fatalf("error = %T %v, want context cancellation", callErr, callErr)
			}
			var responseErr *ResponseError
			if !errors.As(callErr, &responseErr) {
				t.Fatalf("error = %T, want ResponseError", callErr)
			}
			if responseErr.StatusCode() != http.StatusServiceUnavailable || responseErr.Message() != "safe message" || responseErr.Type() != "gateway_error" || responseErr.Code() != "blocked" || responseErr.RequestID() != "request-id" || responseErr.ResponseID() != "response-id" || responseErr.GenerationID() != "generation-id" {
				t.Fatalf("response metadata was not preserved: %#v", responseErr)
			}
			if retryAfter, ok := responseErr.RetryAfter(); !ok || retryAfter != 7*time.Second {
				t.Fatalf("Retry-After = %v, %v; want 7s, true", retryAfter, ok)
			}
			if raw := responseErr.RawResponseBody(); len(raw) > 1<<20 || !bytes.Equal(raw, prefix) || responseErr.BodyTruncated() {
				t.Fatalf("raw diagnostic length/truncation = %d/%v", len(raw), responseErr.BodyTruncated())
			}
			if strings.Contains(callErr.Error(), "safe message") || strings.Contains(callErr.Error(), "request-id") {
				t.Fatalf("formatted error exposed response diagnostics: %q", callErr)
			}
			if got := body.closeCalls.Load(); got != 1 {
				t.Fatalf("body close calls = %d, want 1", got)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1 (no retry or replay after response receipt)", got)
			}
			if got := retryWaits.Load(); got != 0 {
				t.Fatalf("retry waits = %d, want 0", got)
			}
		})
	}
}

type blockedBufferedBody struct {
	prefix      []byte
	offset      int
	readStarted chan struct{}
	unblock     chan struct{}
	readOnce    sync.Once
	closeOnce   sync.Once
	closeCalls  atomic.Int32
}

func newBlockedBufferedBody() *blockedBufferedBody {
	return &blockedBufferedBody{readStarted: make(chan struct{}), unblock: make(chan struct{})}
}

func newBlockedBufferedBodyWithPrefix(prefix []byte) *blockedBufferedBody {
	return &blockedBufferedBody{prefix: append([]byte(nil), prefix...), readStarted: make(chan struct{}), unblock: make(chan struct{})}
}

func (body *blockedBufferedBody) Read(dst []byte) (int, error) {
	if body.offset < len(body.prefix) {
		n := copy(dst, body.prefix[body.offset:])
		body.offset += n
		return n, nil
	}
	body.readOnce.Do(func() { close(body.readStarted) })
	<-body.unblock
	return 0, errors.New("body closed during read")
}

func (body *blockedBufferedBody) Close() error {
	body.closeCalls.Add(1)
	body.closeOnce.Do(func() { close(body.unblock) })
	return nil
}

type fixedChunkReader struct {
	data  []byte
	chunk int
}

func (r *fixedChunkReader) Read(dst []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(r.data), min(len(dst), r.chunk))
	copy(dst, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}
