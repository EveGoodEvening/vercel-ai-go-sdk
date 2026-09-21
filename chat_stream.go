package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

const maxChatCompletionStreamEvents = 10000

// ChatCompletionChunk is one documented chat.completion.chunk payload.
// Fields not represented by this type remain available through RawJSON.
type ChatCompletionChunk struct {
	Object  string
	Choices []ChatCompletionChunkChoice
	rawJSON []byte
}

// RawJSON returns a fresh copy of the complete chunk JSON.
func (c *ChatCompletionChunk) RawJSON() []byte {
	if c == nil {
		return nil
	}
	return append([]byte(nil), c.rawJSON...)
}

// ChatCompletionChunkChoice is one choice in a streamed chat completion chunk.
type ChatCompletionChunkChoice struct {
	Index int
	Delta ChatCompletionChunkDelta
}

// ChatCompletionChunkDelta contains the documented streamed text content.
type ChatCompletionChunkDelta struct {
	Content string
}

// ChatCompletionStream is a caller-owned, incrementally read Chat Completions
// stream. It starts no producer goroutine and retains no transcript.
type ChatCompletionStream struct {
	ctx        context.Context
	body       io.ReadCloser
	parser     *sseParser
	stopCancel func() bool

	mu            sync.Mutex
	once          sync.Once
	closed        bool
	terminating   bool
	terminalDone  chan struct{}
	terminalErr   error
	includeClose  bool
	event         *ChatCompletionChunk
	err           error
	count         int
	close         error
	beforePublish func()
}

// StreamChatCompletion validates and starts a streaming Chat Completions
// request. A non-200 response is consumed and returned as ResponseError before
// a stream is exposed. Streaming responses are never retried after headers.
func (client *Client) StreamChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionStream, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "required")
	}
	if err := validateChatCompletionRequest(request); err != nil {
		return nil, err
	}
	payload, err := encodeStreamingChatCompletionRequest(request)
	if err != nil {
		return nil, &TransportError{operation: "encode request", cause: err}
	}
	authorization, _, err := client.config.credential.authorization(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.publicEndpoint("/chat/completions"), bytes.NewReader(payload))
	if err != nil {
		return nil, &TransportError{operation: "create request", cause: err}
	}
	req.Header = client.config.buildPublicHeaders(authorization)
	req.Header.Set("Accept", "text/event-stream")
	httpClient := &http.Client{Transport: client.config.httpClient.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Jar: client.config.httpClient.Jar, Timeout: client.config.httpClient.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &TransportError{operation: "send request", cause: err}
	}
	if resp.StatusCode != http.StatusOK {
		capture := httpx.ReadAndClose(resp.Body)
		now := time.Now()
		if client.config.retryHooks.now != nil {
			now = client.config.retryHooks.now()
		}
		return nil, composeResponseError(rawEvaluationResponse{statusCode: resp.StatusCode, headers: resp.Header.Clone(), body: capture.Body, bodyTruncated: capture.Truncated, bodyErr: capture.Err}, now)
	}
	stream := &ChatCompletionStream{ctx: ctx, body: resp.Body, terminalDone: make(chan struct{})}
	stream.parser = newSSEParser(resp.Body)
	stream.stopCancel = context.AfterFunc(ctx, func() {
		stream.finish(&TransportError{operation: "read chat completion stream", cause: ctx.Err()})
	})
	return stream, nil
}

func encodeStreamingChatCompletionRequest(request ChatCompletionRequest) ([]byte, error) {
	payload, err := encodeChatCompletionRequest(request)
	if err != nil {
		return nil, err
	}
	needle := []byte(`"stream":false`)
	at := bytes.Index(payload, needle)
	if at < 0 {
		return nil, errors.New("encoded request omitted stream field")
	}
	out := make([]byte, 0, len(payload)-1)
	out = append(out, payload[:at]...)
	out = append(out, `"stream":true`...)
	out = append(out, payload[at+len(needle):]...)
	return out, nil
}

// Next advances to the next JSON chunk. It returns false after the required
// [DONE] terminal marker, after Close, or on error. Event is valid only after
// Next returns true.
func (s *ChatCompletionStream) Next() bool {
	if s == nil || s.stopped() {
		return false
	}
	if err := s.ctx.Err(); err != nil {
		s.finish(&TransportError{operation: "read chat completion stream", cause: err})
		return false
	}
	event, err := s.parser.next()
	if err != nil {
		if s.stopped() {
			return false
		} else if contextErr := s.ctx.Err(); contextErr != nil {
			s.finish(&TransportError{operation: "read chat completion stream", cause: contextErr})
		} else if errors.Is(err, io.EOF) {
			s.finish(&TransportError{operation: "read chat completion stream", cause: errors.New("chat completion stream ended before [DONE]")})
		} else {
			s.finish(&TransportError{operation: "read chat completion stream", cause: err})
		}
		return false
	}
	if s.stopped() {
		return false
	}

	s.mu.Lock()
	if s.closed || s.terminating {
		done := s.terminalDone
		s.mu.Unlock()
		<-done
		return false
	}
	if s.count >= maxChatCompletionStreamEvents {
		s.mu.Unlock()
		s.finish(&TransportError{operation: "read chat completion stream", cause: errors.New("chat completion stream exceeds 10000 events")})
		return false
	}
	s.count++
	s.mu.Unlock()

	if bytes.Equal(event.data, []byte("[DONE]")) {
		s.finish(nil)
		return false
	}
	chunk, err := decodeChatCompletionChunk(event.data)
	if err != nil {
		s.finish(&TransportError{operation: "read chat completion stream", cause: err})
		return false
	}
	if s.beforePublish != nil {
		s.beforePublish()
	}
	s.mu.Lock()
	if s.closed || s.terminating {
		done := s.terminalDone
		s.mu.Unlock()
		<-done
		return false
	}
	if contextErr := s.ctx.Err(); contextErr != nil {
		s.mu.Unlock()
		s.finish(&TransportError{operation: "read chat completion stream", cause: contextErr})
		return false
	}
	s.event = chunk
	s.mu.Unlock()
	return true
}

func (s *ChatCompletionStream) stopped() bool {
	s.mu.Lock()
	if !s.closed && !s.terminating {
		s.mu.Unlock()
		return false
	}
	done := s.terminalDone
	s.mu.Unlock()
	<-done
	return true
}

// Event returns the current chunk, or nil before the first successful Next and
// after Next terminates.
func (s *ChatCompletionStream) Event() *ChatCompletionChunk {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.event
}

// Err returns the terminal stream error. Successful [DONE] termination and an
// explicit Close return nil. The returned error remains stable.
func (s *ChatCompletionStream) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close stops the stream and closes its response body exactly once. It is safe
// to call repeatedly and may unblock a concurrent Next.
func (s *ChatCompletionStream) Close() error {
	if s == nil {
		return nil
	}
	s.terminate(nil, false)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close
}

func (s *ChatCompletionStream) finish(streamErr error) { s.terminate(streamErr, true) }

func (s *ChatCompletionStream) terminate(streamErr error, includeClose bool) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.terminating {
		done := s.terminalDone
		s.mu.Unlock()
		<-done
		return
	}
	s.terminating = true
	s.terminalErr = streamErr
	s.includeClose = includeClose
	done := s.terminalDone
	s.mu.Unlock()

	var closeErr error
	s.once.Do(func() { closeErr = s.body.Close() })

	s.mu.Lock()
	s.close = closeErr
	s.event = nil
	if s.terminalErr != nil {
		s.err = s.terminalErr
	}
	if s.includeClose && closeErr != nil {
		s.err = &TransportError{operation: "read chat completion stream", cause: errors.Join(s.terminalErr, closeErr)}
	}
	s.closed = true
	close(done)
	s.mu.Unlock()

	if s.stopCancel != nil {
		s.stopCancel()
	}
}

func decodeChatCompletionChunk(data []byte) (*ChatCompletionChunk, error) {
	if err := validateResponseJSON(data); err != nil {
		return nil, err
	}
	var wire struct {
		Object  *string `json:"object"`
		Choices []struct {
			Index *int `json:"index"`
			Delta *struct {
				Content *string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, err
	}
	if wire.Object == nil || *wire.Object != "chat.completion.chunk" {
		return nil, errors.New("chat completion chunk is missing chat.completion.chunk object discriminator")
	}
	chunk := &ChatCompletionChunk{Object: *wire.Object, Choices: make([]ChatCompletionChunkChoice, len(wire.Choices)), rawJSON: append([]byte(nil), data...)}
	for i, choice := range wire.Choices {
		if choice.Index != nil {
			chunk.Choices[i].Index = *choice.Index
		}
		if choice.Delta != nil && choice.Delta.Content != nil {
			chunk.Choices[i].Delta.Content = *choice.Delta.Content
		}
	}
	return chunk, nil
}
