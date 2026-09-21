package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"
)

func chatStreamClient(body io.ReadCloser, requests *int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		*requests++
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body, Request: request}, nil
	})}
}

func TestStreamChatCompletionRequestChunksAndDone(t *testing.T) {
	clearCredentialEnvironment(t)
	payload := ": keepalive\r\n\r\n" +
		"event: ignored\r\nid: one\r\ndata: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"provider/model\",\"choices\":[{\"index\":1,\"delta\":{\"content\":\"B\",\"role\":\"assistant\",\"tool_calls\":[{\"future\":true}]},\"finish_reason\":\"stop\"},{\"index\":0,\"delta\":{\"content\":\"A\",\"refusal\":\"no\"}}],\"usage\":{\"total_tokens\":7},\"future\":{\"x\":[1]}}\r\n\r\n" +
		"data: {\"object\":\"chat.completion.chunk\",\r\ndata: \"choices\":[{\"index\":0,\"delta\":{\"content\":\"C\"}}]}\r\n\r\n" +
		"data: [DONE]\r\n\r\n"
	body := newCountedBlockingBody([]byte(payload), false)
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
	stream, err := client.StreamChatCompletion(context.Background(), validChatRequest())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || captured.Method != http.MethodPost || captured.URL.String() != "https://example.test/v1/chat/completions" || string(capturedBody) != `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":true}` {
		t.Fatalf("request count=%d method=%s URL=%s body=%s", requests, captured.Method, captured.URL, capturedBody)
	}
	if captured.Header.Get("Authorization") != "Bearer secret" || captured.Header.Get("Content-Type") != "application/json" || captured.Header.Get("Accept") != "text/event-stream" || strings.Join(captured.Header.Values("X-Custom"), ",") != "one,two" {
		t.Fatalf("headers=%#v", captured.Header)
	}
	if !stream.Next() {
		t.Fatalf("first Next: %v", stream.Err())
	}
	first := stream.Event()
	if first.Object != "chat.completion.chunk" || len(first.Choices) != 2 || first.Choices[0].Index != 1 || first.Choices[0].Delta.Content != "B" || first.Choices[1].Index != 0 || first.Choices[1].Delta.Content != "A" {
		t.Fatalf("first=%#v", first)
	}
	wantRaw := []byte(`{"id":"chunk-1","object":"chat.completion.chunk","created":1,"model":"provider/model","choices":[{"index":1,"delta":{"content":"B","role":"assistant","tool_calls":[{"future":true}]},"finish_reason":"stop"},{"index":0,"delta":{"content":"A","refusal":"no"}}],"usage":{"total_tokens":7},"future":{"x":[1]}}`)
	if !bytes.Equal(first.RawJSON(), wantRaw) {
		t.Fatalf("raw=%s", first.RawJSON())
	}
	mutated := first.RawJSON()
	mutated[0] = 'x'
	if bytes.Equal(mutated, first.RawJSON()) {
		t.Fatal("RawJSON is not defensive")
	}
	if !stream.Next() {
		t.Fatalf("second Next: %v", stream.Err())
	}
	second := stream.Event()
	if len(second.Choices) != 1 || second.Choices[0].Delta.Content != "C" {
		t.Fatalf("second=%#v", second)
	}
	if stream.Next() || stream.Event() != nil || stream.Err() != nil || stream.Next() {
		t.Fatalf("DONE state event=%#v err=%v", stream.Event(), stream.Err())
	}
	if body.closes() != 1 {
		t.Fatalf("closes=%d", body.closes())
	}
	if err := stream.Close(); err != nil || body.closes() != 1 {
		t.Fatalf("Close=%v closes=%d", err, body.closes())
	}
	if (*ChatCompletionChunk)(nil).RawJSON() != nil {
		t.Fatal("nil RawJSON must be nil")
	}
}

func TestStreamChatCompletionRequiresDoneAndRejectsTruncation(t *testing.T) {
	for _, test := range []struct{ name, payload string }{
		{"premature EOF", "data: {\"object\":\"chat.completion.chunk\",\"choices\":[]}\n\n"},
		{"unterminated line", "data: [DONE]"},
		{"unterminated event", "data: [DONE]\n"},
		{"malformed JSON", "data: {\"choices\":\n\n"},
		{"missing object", "data: {\"choices\":[]}\n\n"},
		{"wrong object", "data: {\"object\":\"chat.completion\",\"choices\":[]}\n\n"},
		{"DONE must be exact", "data: [DONE] extra\n\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := newCountedBlockingBody([]byte(test.payload), false)
			requests := 0
			client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(chatStreamClient(body, &requests)))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := client.StreamChatCompletion(context.Background(), validChatRequest())
			if err != nil {
				t.Fatal(err)
			}
			for stream.Next() {
			}
			var transportErr *TransportError
			if stream.Event() != nil || !errors.As(stream.Err(), &transportErr) || transportErr.Operation() != "read chat completion stream" || body.closes() != 1 {
				t.Fatalf("event=%#v err=%T %v closes=%d", stream.Event(), stream.Err(), stream.Err(), body.closes())
			}
			stable := stream.Err()
			if stream.Next() || stream.Err() != stable {
				t.Fatal("terminal error is not stable")
			}
		})
	}
}

func TestStreamChatCompletionResourceLimits(t *testing.T) {
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
	cases := []struct{ name, payload string }{
		{"line", "data:" + strings.Repeat("x", maxSSELineBytes) + "\n\n"},
		{"event", "data:" + strings.Repeat("x", maxSSEEventBytes+1) + "\n\n"},
		{"depth", "data: {\"choices\":[],\"future\":" + deep + "}\n\n"},
		{"array count", "data: {\"choices\":[],\"future\":[" + members + "]}\n\n"},
		{"object count", "data: {\"choices\":[],\"future\":{" + objectMembers.String() + "}}\n\n"},
		{"decoded string", "data: {\"choices\":[],\"future\":\"" + strings.Repeat("x", maxResponseValueBytes+1) + "\"}\n\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := newCountedBlockingBody([]byte(test.payload), false)
			requests := 0
			client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(chatStreamClient(body, &requests)))
			stream, err := client.StreamChatCompletion(context.Background(), validChatRequest())
			if err != nil {
				t.Fatal(err)
			}
			if stream.Next() || stream.Err() == nil || stream.Event() != nil || body.closes() != 1 {
				t.Fatalf("Next/event/error/closes=false/%#v/%v/%d", stream.Event(), stream.Err(), body.closes())
			}
		})
	}
}

func TestStreamChatCompletionEventCountLimit(t *testing.T) {
	var payload strings.Builder
	for range maxResponseStreamEvents + 1 {
		payload.WriteString("data: {\"object\":\"chat.completion.chunk\",\"choices\":[]}\n\n")
	}
	payload.WriteString("data: [DONE]\n\n")
	body := newCountedBlockingBody([]byte(payload.String()), false)
	requests := 0
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(chatStreamClient(body, &requests)))
	stream, err := client.StreamChatCompletion(context.Background(), validChatRequest())
	if err != nil {
		t.Fatal(err)
	}
	for index := range maxResponseStreamEvents {
		if !stream.Next() {
			t.Fatalf("event %d: %v", index, stream.Err())
		}
	}
	if stream.Next() || stream.Err() == nil || body.closes() != 1 {
		t.Fatalf("limit error=%v closes=%d", stream.Err(), body.closes())
	}
}

func TestStreamChatCompletionStatusDiagnosticAndNoRetry(t *testing.T) {
	diagnostic := bytes.Repeat([]byte("x"), (1<<20)+1)
	body := newCountedBlockingBody(diagnostic, false)
	requests := 0
	client, _ := NewClient(WithAPIKey("secret"), WithRetryPolicy(RetryPolicy{MaxAttempts: 3}), WithHTTPClient(streamHTTPClient(http.StatusServiceUnavailable, http.Header{"Retry-After": {"1"}}, body, &requests)))
	stream, err := client.StreamChatCompletion(context.Background(), validChatRequest())
	if stream != nil {
		t.Fatal("non-200 exposed stream")
	}
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode() != http.StatusServiceUnavailable || !responseErr.BodyTruncated() || len(responseErr.RawResponseBody()) != 1<<20 || requests != 1 || body.closes() != 1 {
		t.Fatalf("err=%T %v requests=%d closes=%d", err, err, requests, body.closes())
	}
}

func TestStreamChatCompletionCloseCancelAndConcurrentNext(t *testing.T) {
	for _, test := range []struct {
		name         string
		stop         func(context.CancelFunc, *ChatCompletionStream)
		wantCanceled bool
	}{
		{"Close", func(_ context.CancelFunc, stream *ChatCompletionStream) { _ = stream.Close() }, false},
		{"cancel", func(cancel context.CancelFunc, _ *ChatCompletionStream) { cancel() }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := newCountedBlockingBody(nil, true)
			requests := 0
			client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(chatStreamClient(body, &requests)))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream, err := client.StreamChatCompletion(ctx, validChatRequest())
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
					t.Fatal("Next returned true")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Next did not unblock")
			}
			if body.closes() != 1 || stream.Event() != nil {
				t.Fatalf("closes=%d event=%#v", body.closes(), stream.Event())
			}
			if test.wantCanceled {
				if !errors.Is(stream.Err(), context.Canceled) {
					t.Fatalf("error=%v", stream.Err())
				}
			} else if stream.Err() != nil {
				t.Fatalf("error=%v", stream.Err())
			}
			if err := stream.Close(); err != nil || body.closes() != 1 {
				t.Fatalf("Close=%v closes=%d", err, body.closes())
			}
		})
	}
}

func TestStreamChatCompletionNoProducerLeakAndResponsesCompatibility(t *testing.T) {
	before := runtime.NumGoroutine()
	for range 20 {
		body := newCountedBlockingBody(nil, true)
		requests := 0
		client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(chatStreamClient(body, &requests)))
		stream, err := client.StreamChatCompletion(context.Background(), validChatRequest())
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

	body := newCountedBlockingBody([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"), false)
	requests := 0
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(streamHTTPClient(http.StatusOK, nil, body, &requests)))
	stream, err := client.StreamResponse(context.Background(), validResponsesRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !stream.Next() {
		t.Fatalf("Responses stream changed: %v", stream.Err())
	}
	event, ok := stream.Event().(ResponseOutputTextDeltaEvent)
	if !ok || event.Delta != "ok" {
		t.Fatalf("Responses event=%#v", stream.Event())
	}
	if stream.Next() || stream.Err() != nil {
		t.Fatalf("Responses EOF=%v", stream.Err())
	}
}
