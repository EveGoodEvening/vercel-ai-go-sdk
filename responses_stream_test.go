package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSSEFramingAndLimits(t *testing.T) {
	t.Run("LF CRLF multiline order IDs and types", func(t *testing.T) {
		parser := newSSEParser(strings.NewReader(": keepalive\r\nevent: first\r\nid: one\r\ndata: {\"a\":\r\ndata: 1}\r\n\r\nevent: second\ndata: {}\n\n"))
		first, err := parser.next()
		if err != nil || first.name != "first" || first.id != "one" || string(first.data) != "{\"a\":\n1}" {
			t.Fatalf("first = %#v, %v", first, err)
		}
		second, err := parser.next()
		if err != nil || second.name != "second" || second.id != "one" || string(second.data) != "{}" {
			t.Fatalf("second = %#v, %v", second, err)
		}
		if _, err := parser.next(); !errors.Is(err, io.EOF) {
			t.Fatalf("EOF = %v", err)
		}
	})

	t.Run("id-only block persists and omitted event defaults to message", func(t *testing.T) {
		parser := newSSEParser(strings.NewReader("id: inherited\n\ndata: {}\n\n"))
		event, err := parser.next()
		if err != nil || event.name != "message" || event.id != "inherited" || string(event.data) != "{}" {
			t.Fatalf("event = %#v, %v", event, err)
		}
		if _, err := parser.next(); !errors.Is(err, io.EOF) {
			t.Fatalf("EOF = %v", err)
		}
	})

	t.Run("line limit plus one", func(t *testing.T) {
		atLimit := "data:" + strings.Repeat("x", maxSSELineBytes-len("data:")) + "\n\n"
		if _, err := newSSEParser(strings.NewReader(atLimit)).next(); err != nil {
			t.Fatalf("at limit: %v", err)
		}
		over := "data:" + strings.Repeat("x", maxSSELineBytes-len("data:")+1) + "\n\n"
		if _, err := newSSEParser(strings.NewReader(over)).next(); !errors.Is(err, errSSELineTooLarge) {
			t.Fatalf("over limit = %v", err)
		}
	})

	t.Run("event limit plus one", func(t *testing.T) {
		makeEvent := func(size int) string {
			const chunk = 32 << 10
			var b strings.Builder
			remaining := size
			for remaining > 0 {
				n := min(remaining, chunk)
				b.WriteString("data:")
				b.WriteString(strings.Repeat("x", n))
				b.WriteByte('\n')
				remaining -= n
				if remaining > 0 {
					remaining-- // the inserted newline is part of assembled data
				}
			}
			b.WriteByte('\n')
			return b.String()
		}
		if event, err := newSSEParser(strings.NewReader(makeEvent(maxSSEEventBytes))).next(); err != nil || len(event.data) != maxSSEEventBytes {
			t.Fatalf("at limit length=%d error=%v", len(event.data), err)
		}
		if _, err := newSSEParser(strings.NewReader(makeEvent(maxSSEEventBytes + 1))).next(); !errors.Is(err, errSSEEventTooLarge) {
			t.Fatalf("over limit = %v", err)
		}
	})

	for name, input := range map[string]string{
		"unterminated line":  "data: {}",
		"unterminated event": "data: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := newSSEParser(strings.NewReader(input)).next(); !errors.Is(err, errSSETruncated) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

type countedBlockingBody struct {
	readStarted chan struct{}
	closed      chan struct{}
	readRelease chan struct{}
	once        sync.Once
	mu          sync.Mutex
	closeCount  int
	payload     *bytes.Reader
}

func newCountedBlockingBody(payload []byte, block bool) *countedBlockingBody {
	body := &countedBlockingBody{readStarted: make(chan struct{}), closed: make(chan struct{}), payload: bytes.NewReader(payload)}
	if !block {
		close(body.closed)
	}
	return body
}

func (body *countedBlockingBody) Read(buffer []byte) (int, error) {
	if body.payload.Len() > 0 {
		return body.payload.Read(buffer)
	}
	body.once.Do(func() { close(body.readStarted) })
	<-body.closed
	if body.readRelease != nil {
		<-body.readRelease
	}
	return 0, io.EOF
}

func (body *countedBlockingBody) Close() error {
	body.mu.Lock()
	body.closeCount++
	body.mu.Unlock()
	select {
	case <-body.closed:
	default:
		close(body.closed)
	}
	return nil
}

func (body *countedBlockingBody) closes() int {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.closeCount
}

func streamHTTPClient(status int, headers http.Header, body io.ReadCloser, requests *int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		*requests++
		return &http.Response{StatusCode: status, Header: headers, Body: body, Request: request}, nil
	})}
}

func waitForSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(message)
	}
}

func TestStreamResponseNoBackgroundGoroutineBaseline(t *testing.T) {
	// A generous baseline catches an accidentally leaked producer per stream while
	// tolerating runtime and HTTP housekeeping goroutines.
	before := runtime.NumGoroutine()
	for range 20 {
		body := newCountedBlockingBody(nil, true)
		requests := 0
		client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(streamHTTPClient(http.StatusOK, http.Header{"Content-Type": {"text/event-stream"}}, body, &requests)))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := client.StreamResponse(context.Background(), validResponsesRequest())
		if err != nil {
			t.Fatal(err)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+4 {
		t.Fatalf("goroutines grew from %d to %d", before, after)
	}
}

func TestStreamResponseRequestEventsAndCleanEOF(t *testing.T) {
	clearCredentialEnvironment(t)
	body := newCountedBlockingBody([]byte("id: delta-1\r\ndata: {\"type\":\"response.output_text.delta\",\r\ndata: \"delta\":\"hello\"}\r\n\r\nevent: future\nid: raw-2\ndata: {\"type\":\"response.future\",\"value\":[1,true]}\n\n"), false)
	var captured *http.Request
	var capturedBody []byte
	requests := 0
	client, err := NewClient(WithAPIKey("secret"), WithPublicBaseURL("https://example.test/v1///"), WithHeaders(http.Header{"X-Custom": {"one", "two"}}), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		captured = request.Clone(request.Context())
		capturedBody, _ = io.ReadAll(request.Body)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body, Request: request}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.StreamResponse(context.Background(), validResponsesRequest())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || captured.Method != http.MethodPost || captured.URL.String() != "https://example.test/v1/responses" || string(capturedBody) != `{"input":"hello","model":"provider/model","stream":true}` {
		t.Fatalf("request count=%d method=%s URL=%s body=%s", requests, captured.Method, captured.URL, capturedBody)
	}
	if captured.Header.Get("Authorization") != "Bearer secret" || captured.Header.Get("Content-Type") != "application/json" || captured.Header.Get("Accept") != "text/event-stream" || strings.Join(captured.Header.Values("X-Custom"), ",") != "one,two" {
		t.Fatalf("headers = %#v", captured.Header)
	}
	if !stream.Next() {
		t.Fatalf("first Next false: %v", stream.Err())
	}
	delta, ok := stream.Event().(ResponseOutputTextDeltaEvent)
	if !ok || delta != (ResponseOutputTextDeltaEvent{Type: "response.output_text.delta", Event: "message", ID: "delta-1", Delta: "hello"}) {
		t.Fatalf("delta = %#v", stream.Event())
	}
	if !stream.Next() {
		t.Fatalf("second Next false: %v", stream.Err())
	}
	raw, ok := stream.Event().(RawResponseEvent)
	wantRaw := []byte(`{"type":"response.future","value":[1,true]}`)
	if !ok || raw.Type != "response.future" || raw.Event != "future" || raw.ID != "raw-2" || !bytes.Equal(raw.RawJSON(), wantRaw) {
		t.Fatalf("raw = %#v / %q", stream.Event(), raw.RawJSON())
	}
	first := raw.RawJSON()
	first[0] = 'x'
	if bytes.Equal(first, raw.RawJSON()) {
		t.Fatal("RawJSON did not return a defensive copy")
	}
	if stream.Next() || stream.Event() != nil || stream.Err() != nil || stream.Next() || stream.Err() != nil {
		t.Fatalf("clean EOF was not stable: event=%#v err=%v", stream.Event(), stream.Err())
	}
	if body.closes() != 1 {
		t.Fatalf("body closes = %d", body.closes())
	}
	if err := stream.Close(); err != nil || body.closes() != 1 {
		t.Fatalf("idempotent Close error=%v closes=%d", err, body.closes())
	}
}

func TestStreamResponseMalformedAndResourceLimits(t *testing.T) {
	deep := `"leaf"`
	for range 64 {
		deep = `[` + deep + `]`
	}
	members := strings.Repeat(`"x",`, maxResponseMembers) + `"x"`
	var objectMembers strings.Builder
	for index := range maxResponseMembers + 1 {
		if index != 0 {
			objectMembers.WriteByte(',')
		}
		objectMembers.WriteString(`"`)
		objectMembers.WriteString(strings.Repeat("a", index/26))
		objectMembers.WriteByte(byte('a' + index%26))
		objectMembers.WriteString(`":0`)
	}
	cases := []struct {
		name string
		body string
	}{
		{"malformed JSON", "data: {\"type\":\n\n"},
		{"truncated framing", "data: {}\n"},
		{"depth limit plus one", "data: {\"type\":\"future\",\"value\":" + deep + "}\n\n"},
		{"array member limit plus one", "data: {\"type\":\"future\",\"value\":[" + members + "]}\n\n"},
		{"object member limit plus one", "data: {\"type\":\"future\",\"value\":{" + objectMembers.String() + "}}\n\n"},
		{"typed delta missing delta", "data: {\"type\":\"response.output_text.delta\"}\n\n"},
		{"missing type discriminator", "data: {}\n\n"},
		{"null type discriminator", "data: {\"type\":null}\n\n"},
		{"empty type discriminator", "data: {\"type\":\"\"}\n\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := newCountedBlockingBody([]byte(test.body), false)
			requests := 0
			client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(streamHTTPClient(http.StatusOK, nil, body, &requests)))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := client.StreamResponse(context.Background(), validResponsesRequest())
			if err != nil {
				t.Fatal(err)
			}
			if stream.Next() || stream.Event() != nil || stream.Err() == nil {
				t.Fatalf("Next/event/error = false/%#v/%v", stream.Event(), stream.Err())
			}
			var transportErr *TransportError
			if !errors.As(stream.Err(), &transportErr) || transportErr.Operation() != "read response stream" {
				t.Fatalf("error = %T %v", stream.Err(), stream.Err())

			}
			stable := stream.Err()
			if stream.Next() || stream.Err() != stable || body.closes() != 1 {
				t.Fatalf("terminal state changed: error=%v closes=%d", stream.Err(), body.closes())
			}
		})
	}
}
func TestStreamResponseDecodedStringLimitPlusOne(t *testing.T) {
	for _, test := range []struct {
		name  string
		size  int
		valid bool
	}{
		{"at limit", maxResponseValueBytes, true},
		{"limit plus one", maxResponseValueBytes + 1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := []byte(`{"type":"future","value":"` + strings.Repeat("x", test.size) + `"}`)
			event, err := decodeResponseEvent(sseEvent{name: "future", id: "id", data: data})
			if test.valid {
				raw, ok := event.(RawResponseEvent)
				if err != nil || !ok || !bytes.Equal(raw.RawJSON(), data) {
					t.Fatalf("event=%#v error=%v", event, err)
				}
			} else if err == nil || event != nil {
				t.Fatalf("event=%#v error=%v", event, err)
			}
		})
	}
}

func TestStreamResponseEventCountLimitPlusOne(t *testing.T) {
	var payload strings.Builder
	for range maxResponseStreamEvents + 1 {
		payload.WriteString("data: {\"type\":\"future\"}\n\n")
	}
	body := newCountedBlockingBody([]byte(payload.String()), false)
	requests := 0
	client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(streamHTTPClient(http.StatusOK, nil, body, &requests)))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.StreamResponse(context.Background(), validResponsesRequest())
	if err != nil {
		t.Fatal(err)
	}
	for index := range maxResponseStreamEvents {
		if !stream.Next() {
			t.Fatalf("event %d: %v", index, stream.Err())
		}
	}
	if stream.Next() || stream.Err() == nil || body.closes() != 1 {
		t.Fatalf("limit+1 Next/error/closes = false/%v/%d", stream.Err(), body.closes())
	}
}

func TestStreamResponseStatusAndDiagnosticLimit(t *testing.T) {
	diagnostic := bytes.Repeat([]byte("x"), (1<<20)+1)
	body := newCountedBlockingBody(diagnostic, false)
	requests := 0
	client, err := NewClient(WithAPIKey("secret"), WithRetryPolicy(RetryPolicy{MaxAttempts: 3}), WithHTTPClient(streamHTTPClient(http.StatusServiceUnavailable, http.Header{"Retry-After": {"1"}}, body, &requests)))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.StreamResponse(context.Background(), validResponsesRequest())
	if stream != nil {
		t.Fatal("non-200 exposed a stream")
	}
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode() != http.StatusServiceUnavailable || !responseErr.BodyTruncated() || len(responseErr.RawResponseBody()) != 1<<20 || requests != 1 || body.closes() != 1 {
		t.Fatalf("error=%T %v status=%d truncated=%v diagnostic=%d requests=%d closes=%d", err, err, responseErr.StatusCode(), responseErr.BodyTruncated(), len(responseErr.RawResponseBody()), requests, body.closes())
	}
}

func TestStreamResponseCloseAndCancellationUnblockRead(t *testing.T) {
	for _, test := range []struct {
		name    string
		stop    func(context.CancelFunc, *ResponseStream)
		wantErr bool
	}{
		{"Close", func(_ context.CancelFunc, stream *ResponseStream) { _ = stream.Close() }, false},
		{"cancel", func(cancel context.CancelFunc, _ *ResponseStream) { cancel() }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := newCountedBlockingBody(nil, true)
			requests := 0
			client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(streamHTTPClient(http.StatusOK, nil, body, &requests)))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, err := client.StreamResponse(ctx, validResponsesRequest())
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan bool, 1)
			go func() { done <- stream.Next() }()
			waitForSignal(t, body.readStarted, "Next never started reading")
			test.stop(cancel, stream)
			select {
			case next := <-done:
				if next {
					t.Fatal("blocked Next returned true")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("blocked Next was not promptly unblocked")
			}
			if (stream.Err() != nil) != test.wantErr || body.closes() != 1 || stream.Next() {
				t.Fatalf("error=%v closes=%d later Next=%v", stream.Err(), body.closes(), stream.Next())
			}
			if err := stream.Close(); err != nil || body.closes() != 1 {
				t.Fatalf("repeated Close=%v closes=%d", err, body.closes())
			}
		})
	}
}

func TestStreamResponseCloseClearsEventAfterCancellationOwnsBodyClose(t *testing.T) {
	body := newCountedBlockingBody([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"first\"}\n\n"), true)
	body.readRelease = make(chan struct{})
	defer func() {
		select {
		case <-body.readRelease:
		default:
			close(body.readRelease)
		}
	}()
	requests := 0
	client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(streamHTTPClient(http.StatusOK, nil, body, &requests)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.StreamResponse(ctx, validResponsesRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !stream.Next() || stream.Event() == nil {
		t.Fatalf("first Next/event = true/%#v, error=%v", stream.Event(), stream.Err())
	}

	done := make(chan bool, 1)
	go func() { done <- stream.Next() }()
	waitForSignal(t, body.readStarted, "second Next never started reading")
	cancel()
	waitForSignal(t, body.closed, "context cancellation did not close the body")
	if body.closes() != 1 {
		t.Fatalf("context callback body closes = %d", body.closes())
	}
	if err := stream.Close(); err != nil || body.closes() != 1 {
		t.Fatalf("Close error=%v body closes=%d", err, body.closes())
	}
	close(body.readRelease)

	select {
	case next := <-done:
		if next {
			t.Fatal("blocked Next returned true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Next was not promptly unblocked")
	}
	stableErr := stream.Err()
	if stream.Event() != nil || body.closes() != 1 || stream.Next() || stream.Err() != stableErr || stableErr != nil {
		t.Fatalf("event=%#v closes=%d later Next=%v stable error=%v current error=%v", stream.Event(), body.closes(), stream.Next(), stableErr, stream.Err())
	}
}
