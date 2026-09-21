// Package testserver provides hermetic HTTP request capture for SDK tests.
// It is intended to be imported only by test files.
package testserver

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// Request is a defensive snapshot of one received HTTP request.
type Request struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// Response describes a fixture response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Responder selects a response from a defensive request snapshot.
type Responder func(Request) Response

// Server is a loopback fixture server with concurrency-safe request capture.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	requests []Request
}

// New starts a loopback server returning response for every captured request.
func New(response Response) *Server {
	return NewResponder(func(Request) Response { return response })
}

// NewResponder starts a loopback server whose response may depend on the
// captured request. The responder receives its own defensive snapshot.
func NewResponder(responder Responder) *Server {
	if responder == nil {
		panic("testserver: nil responder")
	}
	server := &Server{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		_ = request.Body.Close()
		snapshot := Request{
			Method: request.Method,
			URL:    request.URL.RequestURI(),
			Header: request.Header.Clone(),
			Body:   bytes.Clone(body),
		}
		server.mu.Lock()
		server.requests = append(server.requests, snapshot)
		server.mu.Unlock()
		response := responder(Request{
			Method: snapshot.Method,
			URL:    snapshot.URL,
			Header: snapshot.Header.Clone(),
			Body:   bytes.Clone(snapshot.Body),
		})
		for name, values := range response.Header {
			for _, value := range values {
				writer.Header().Add(name, value)
			}
		}
		status := response.Status
		if status == 0 {
			status = http.StatusOK
		}
		writer.WriteHeader(status)
		_, _ = writer.Write(response.Body)
	}))
	return server
}

// Requests returns defensive copies of all captured requests in arrival order.
func (server *Server) Requests() []Request {
	server.mu.Lock()
	defer server.mu.Unlock()
	requests := make([]Request, len(server.requests))
	for index, request := range server.requests {
		requests[index] = Request{
			Method: request.Method,
			URL:    request.URL,
			Header: request.Header.Clone(),
			Body:   bytes.Clone(request.Body),
		}
	}
	return requests
}
