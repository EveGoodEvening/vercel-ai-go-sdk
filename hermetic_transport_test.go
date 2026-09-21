package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/testserver"
)

type loopbackOnlyTransport struct {
	base http.RoundTripper
}

func (transport loopbackOnlyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("hermetic transport rejected request without URL")
	}
	host := request.URL.Hostname()
	allowed := strings.EqualFold(host, "localhost")
	if address := net.ParseIP(host); address != nil {
		allowed = address.IsLoopback()
	}
	if host == "" || !allowed {
		return nil, errors.New("hermetic transport rejected non-loopback destination")
	}
	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(request)
}

func TestMain(main *testing.M) {
	_ = os.Unsetenv(apiKeyEnvironment)
	_ = os.Unsetenv(oidcTokenEnvironment)
	_ = os.Unsetenv("AI_GATEWAY_LIVE_COST_ACK")
	_ = os.Unsetenv("AI_GATEWAY_PUBLIC_LIVE_COST_ACK")
	original := http.DefaultTransport
	http.DefaultTransport = loopbackOnlyTransport{base: original}
	code := main.Run()
	http.DefaultTransport = original
	os.Exit(code)
}

func TestHermeticPublicGenerationFixtures(t *testing.T) {
	fixture := testserver.NewResponder(func(request testserver.Request) testserver.Response {
		streaming := strings.Contains(string(request.Body), `"stream":true`)
		switch request.URL {
		case "/v1/responses":
			if streaming {
				return testserver.Response{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: []byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n")}
			}
			return testserver.Response{Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"id":"response-fixture"}`)}
		case "/v1/chat/completions":
			if streaming {
				return testserver.Response{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: []byte("data: {\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n")}
			}
			return testserver.Response{Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"id":"chat-fixture","object":"chat.completion","choices":[]}`)}
		default:
			return testserver.Response{Status: http.StatusNotFound, Body: []byte(`{"error":"unexpected fixture route"}`)}
		}
	})
	defer fixture.Close()

	client, err := NewClient(WithAPIKey("fixture-key"), WithPublicBaseURL(fixture.URL+"/v1"))
	if err != nil {
		t.Fatal(err)
	}
	responsesRequest := ResponsesRequest{Model: "fixture/model", Input: ResponseTextInput("fixture prompt")}
	if _, err := client.CreateResponse(context.Background(), responsesRequest); err != nil {
		t.Fatalf("CreateResponse: %v", err)
	}
	responseStream, err := client.StreamResponse(context.Background(), responsesRequest)
	if err != nil {
		t.Fatalf("StreamResponse: %v", err)
	}
	if !responseStream.Next() || responseStream.Event() == nil || responseStream.Next() || responseStream.Err() != nil {
		t.Fatalf("response stream event=%#v err=%v", responseStream.Event(), responseStream.Err())
	}
	if err := responseStream.Close(); err != nil {
		t.Fatalf("close response stream: %v", err)
	}

	chatRequest := ChatCompletionRequest{Model: "fixture/model", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("fixture prompt")}}}
	if _, err := client.CreateChatCompletion(context.Background(), chatRequest); err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	chatStream, err := client.StreamChatCompletion(context.Background(), chatRequest)
	if err != nil {
		t.Fatalf("StreamChatCompletion: %v", err)
	}
	if !chatStream.Next() || chatStream.Event() == nil || chatStream.Next() || chatStream.Err() != nil {
		t.Fatalf("chat stream event=%#v err=%v", chatStream.Event(), chatStream.Err())
	}
	if err := chatStream.Close(); err != nil {
		t.Fatalf("close chat stream: %v", err)
	}

	requests := fixture.Requests()
	if len(requests) != 4 {
		t.Fatalf("captured requests = %d, want 4", len(requests))
	}
	for _, request := range requests {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Fatalf("captured request method=%q URL=%q headers=%#v", request.Method, request.URL, request.Header)
		}
	}
}

func TestHermeticTransportRejectsDefaultPublicGenerationSurfaces(t *testing.T) {
	client, err := NewClient(WithAPIKey("fixture-key"))
	if err != nil {
		t.Fatal(err)
	}
	responsesRequest := ResponsesRequest{Model: "fixture/model", Input: ResponseTextInput("fixture prompt")}
	chatRequest := ChatCompletionRequest{Model: "fixture/model", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("fixture prompt")}}}
	checks := []struct {
		name string
		call func() error
	}{
		{"responses", func() error { _, err := client.CreateResponse(context.Background(), responsesRequest); return err }},
		{"responses stream", func() error { _, err := client.StreamResponse(context.Background(), responsesRequest); return err }},
		{"chat", func() error { _, err := client.CreateChatCompletion(context.Background(), chatRequest); return err }},
		{"chat stream", func() error { _, err := client.StreamChatCompletion(context.Background(), chatRequest); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.call()
			var transportErr *TransportError
			if !errors.As(err, &transportErr) || transportErr.Operation() != "send request" || transportErr.Unwrap() == nil || !strings.Contains(transportErr.Unwrap().Error(), "hermetic transport rejected non-loopback destination") {
				t.Fatalf("error = %T %v, want wrapped hermetic non-loopback rejection", err, err)
			}
		})
	}
}
