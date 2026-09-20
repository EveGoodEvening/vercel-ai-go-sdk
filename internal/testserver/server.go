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

// Response describes the fixture response returned for every request.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Server is a loopback fixture server with concurrency-safe request capture.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	requests []Request
}

// New starts a loopback server returning response for every captured request.
func New(response Response) *Server {
	server := &Server{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		_ = request.Body.Close()
		server.mu.Lock()
		server.requests = append(server.requests, Request{
			Method: request.Method,
			URL:    request.URL.RequestURI(),
			Header: request.Header.Clone(),
			Body:   bytes.Clone(body),
		})
		server.mu.Unlock()
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
