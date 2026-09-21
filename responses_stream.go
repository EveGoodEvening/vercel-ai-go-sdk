package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

const maxResponseStreamEvents = 10000

// ResponseEvent is one event read from a Responses API stream. Callers should
// use a type switch to distinguish ResponseOutputTextDeltaEvent from
// RawResponseEvent.
type ResponseEvent interface{ responseEvent() }

// ResponseOutputTextDeltaEvent is the documented response.output_text.delta
// event. Event is the SSE event name and ID is the SSE event ID.
type ResponseOutputTextDeltaEvent struct {
	Type  string
	Event string
	ID    string
	Delta string
}

func (ResponseOutputTextDeltaEvent) responseEvent() {}

// RawResponseEvent retains an otherwise uninterpreted JSON event. Event is the
// SSE event name, ID is the SSE event ID, and Type is the JSON discriminator.
type RawResponseEvent struct {
	Type  string
	Event string
	ID    string
	raw   []byte
}

func (RawResponseEvent) responseEvent() {}

// RawJSON returns a fresh copy of the event JSON.
func (e RawResponseEvent) RawJSON() []byte { return append([]byte(nil), e.raw...) }

// ResponseStream is a caller-owned, incrementally read Responses API stream.
// It does not start a producer goroutine and does not retain a transcript.
type ResponseStream struct {
	ctx        context.Context
	body       io.ReadCloser
	parser     *sseParser
	stopCancel func() bool

	closed        atomic.Bool
	once          sync.Once
	mu            sync.Mutex
	event         ResponseEvent
	err           error
	count         int
	close         error
	beforePublish func()
}

// StreamResponse validates and starts a streaming Responses API request.
// A non-200 response is consumed and returned as ResponseError before a stream
// is exposed. Streaming responses are never retried after headers are received.
func (client *Client) StreamResponse(ctx context.Context, request ResponsesRequest) (*ResponseStream, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "required")
	}
	if err := validateResponsesRequest(request); err != nil {
		return nil, err
	}
	payload, err := encodeStreamingResponsesRequest(request)
	if err != nil {
		return nil, &TransportError{operation: "encode request", cause: err}
	}
	authorization, _, err := client.config.credential.authorization(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.publicEndpoint("/responses"), bytes.NewReader(payload))
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
	stream := &ResponseStream{ctx: ctx, body: resp.Body}
	stream.parser = newSSEParser(resp.Body)
	stream.stopCancel = context.AfterFunc(ctx, func() {
		stream.mu.Lock()
		if !stream.closed.Load() {
			stream.closed.Store(true)
			stream.event = nil
			stream.err = &TransportError{operation: "read response stream", cause: ctx.Err()}
		}
		stream.mu.Unlock()
		stream.once.Do(func() {
			err := stream.body.Close()
			stream.mu.Lock()
			stream.close = err
			stream.mu.Unlock()
		})
	})
	return stream, nil
}

func encodeStreamingResponsesRequest(request ResponsesRequest) ([]byte, error) {
	payload, err := encodeResponsesRequest(request)
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

// Next advances to the next event. It returns false at clean framed EOF, after
// Close, or on error. Event is valid only after Next returns true.
func (s *ResponseStream) Next() bool {
	if s == nil || s.closed.Load() {
		return false
	}
	if err := s.ctx.Err(); err != nil {
		s.finish(&TransportError{operation: "read response stream", cause: err})
		return false
	}
	event, err := s.parser.next()
	if err != nil {
		if s.closed.Load() {
			return false
		} else if contextErr := s.ctx.Err(); contextErr != nil {
			s.finish(&TransportError{operation: "read response stream", cause: contextErr})
		} else if errors.Is(err, io.EOF) {
			s.finish(nil)
		} else {
			s.finish(&TransportError{operation: "read response stream", cause: err})
		}
		return false
	}
	if s.closed.Load() {
		return false
	}

	s.mu.Lock()
	if s.count >= maxResponseStreamEvents {
		s.mu.Unlock()
		s.finish(&TransportError{operation: "read response stream", cause: errors.New("response stream exceeds 10000 events")})
		return false
	}
	s.count++
	s.mu.Unlock()

	decoded, err := decodeResponseEvent(event)
	if err != nil {
		s.finish(&TransportError{operation: "read response stream", cause: err})
		return false
	}
	if s.beforePublish != nil {
		s.beforePublish()
	}
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		return false
	}
	if contextErr := s.ctx.Err(); contextErr != nil {
		s.closed.Store(true)
		s.event = nil
		s.err = &TransportError{operation: "read response stream", cause: contextErr}
		s.mu.Unlock()
		s.once.Do(func() {
			closeErr := s.body.Close()
			s.mu.Lock()
			s.close = closeErr
			s.mu.Unlock()
		})
		if s.stopCancel != nil {
			s.stopCancel()
		}
		return false
	}
	s.event = decoded
	s.mu.Unlock()
	return true
}

// Event returns the current event, or nil before the first successful Next and
// after Next terminates.
func (s *ResponseStream) Event() ResponseEvent {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.event
}

// Err returns the terminal stream error. Clean EOF and an explicit Close return
// nil. The returned error remains stable after termination.
func (s *ResponseStream) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close stops the stream and closes its response body exactly once. It is safe
// to call repeatedly and may be called to unblock a concurrent Next.
func (s *ResponseStream) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.closed.Store(true)
	s.event = nil
	s.mu.Unlock()
	s.once.Do(func() {
		err := s.body.Close()
		s.mu.Lock()
		s.close = err
		s.mu.Unlock()
	})
	if s.stopCancel != nil {
		s.stopCancel()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close
}

func (s *ResponseStream) finish(streamErr error) {
	if s.closed.Swap(true) {
		return
	}
	s.once.Do(func() {
		closeErr := s.body.Close()
		s.mu.Lock()
		s.close = closeErr
		s.mu.Unlock()
	})
	if s.stopCancel != nil {
		s.stopCancel()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.event = nil
	if streamErr != nil || s.close != nil {
		s.err = &TransportError{operation: "read response stream", cause: errors.Join(streamErr, s.close)}
		if transportErr, ok := streamErr.(*TransportError); ok && s.close == nil {
			s.err = transportErr
		}
	}
}

func decodeResponseEvent(event sseEvent) (ResponseEvent, error) {
	if err := validateResponseJSON(event.data); err != nil {
		return nil, err
	}
	var header struct {
		Type  *string `json:"type"`
		Delta *string `json:"delta"`
	}
	if err := json.Unmarshal(event.data, &header); err != nil {
		return nil, err
	}
	if header.Type == nil || *header.Type == "" {
		return nil, errors.New("response event is missing a nonempty type discriminator")
	}
	if *header.Type == "response.output_text.delta" {
		if header.Delta == nil {
			return nil, errors.New("response.output_text.delta event is missing delta")
		}
		return ResponseOutputTextDeltaEvent{Type: *header.Type, Event: event.name, ID: event.id, Delta: *header.Delta}, nil
	}
	return RawResponseEvent{Type: *header.Type, Event: event.name, ID: event.id, raw: append([]byte(nil), event.data...)}, nil
}
