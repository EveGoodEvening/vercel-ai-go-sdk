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

	mu            sync.Mutex
	once          sync.Once
	closed        bool
	terminating   bool
	terminalDone  chan struct{}
	terminalErr   error
	includeClose  bool
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
	if err := ctx.Err(); err != nil {
		return nil, &TransportError{operation: "send request", cause: err}
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
		capture := readAndCloseResponse(ctx, resp.Body)
		now := time.Now()
		if client.config.retryHooks.now != nil {
			now = client.config.retryHooks.now()
		}
		return nil, composeResponseError(rawEvaluationResponse{statusCode: resp.StatusCode, headers: resp.Header.Clone(), body: capture.Body, bodyTruncated: capture.Truncated, bodyErr: capture.Err}, now)
	}
	stream := &ResponseStream{ctx: ctx, body: resp.Body, terminalDone: make(chan struct{})}
	stream.parser = newSSEParser(resp.Body)
	stream.stopCancel = context.AfterFunc(ctx, func() {
		stream.finish(&TransportError{operation: "read response stream", cause: ctx.Err()})
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
	if s == nil || s.stopped() {
		return false
	}
	if err := s.ctx.Err(); err != nil {
		s.finish(&TransportError{operation: "read response stream", cause: err})
		return false
	}
	event, err := s.parser.next()
	if err != nil {
		if s.stopped() {
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
	if s.closed || s.terminating {
		done := s.terminalDone
		s.mu.Unlock()
		<-done
		return false
	}
	if contextErr := s.ctx.Err(); contextErr != nil {
		s.mu.Unlock()
		s.finish(&TransportError{operation: "read response stream", cause: contextErr})
		return false
	}
	s.event = decoded
	s.mu.Unlock()
	return true
}

func (s *ResponseStream) stopped() bool {
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
	s.terminate(nil, false)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close
}

func (s *ResponseStream) finish(streamErr error) {
	s.terminate(streamErr, true)
}

func (s *ResponseStream) terminate(streamErr error, includeClose bool) {
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
	s.once.Do(func() {
		closeErr = s.body.Close()
	})

	s.mu.Lock()
	s.close = closeErr
	s.event = nil
	if s.terminalErr != nil {
		s.err = s.terminalErr
	}
	if s.includeClose && closeErr != nil {
		s.err = &TransportError{operation: "read response stream", cause: errors.Join(s.terminalErr, closeErr)}
	}
	s.closed = true
	close(done)
	s.mu.Unlock()

	if s.stopCancel != nil {
		s.stopCancel()
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
