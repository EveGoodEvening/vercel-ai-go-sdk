package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/testserver"
)

func TestNewClientDefaultPublicBaseURL(t *testing.T) {
	clearCredentialEnvironment(t)
	client, err := NewClient(WithAPIKey("key"))
	if err != nil {
		t.Fatal(err)
	}
	if client.config.publicBaseURL != "https://ai-gateway.vercel.sh/v1" {
		t.Fatalf("public base URL = %q", client.config.publicBaseURL)
	}
}

func TestWithPublicBaseURL(t *testing.T) {
	clearCredentialEnvironment(t)

	for _, test := range []struct {
		name, value, reason string
	}{
		{"blank", "\t", "must not be empty or whitespace"},
		{"boundary whitespace", " https://example.com/v1", "must not be empty or whitespace"},
		{"relative", "/v1", "must be an absolute http or https URL"},
		{"opaque", "https:opaque", "must be an absolute http or https URL"},
		{"unsupported scheme", "ftp://example.com/v1", "must be an absolute http or https URL"},
		{"userinfo", "https://user@example.com/v1", "must not contain userinfo, query, or fragment"},
		{"query", "https://example.com/v1?x=1", "must not contain userinfo, query, or fragment"},
		{"empty query", "https://example.com/v1?", "must not contain userinfo, query, or fragment"},
		{"fragment", "https://example.com/v1#x", "must not contain userinfo, query, or fragment"},
		{"empty fragment", "https://example.com/v1#", "must not contain userinfo, query, or fragment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(WithAPIKey("key"), WithPublicBaseURL(test.value))
			var configurationError *ConfigurationError
			if !errors.As(err, &configurationError) {
				t.Fatalf("error = %T %v, want *ConfigurationError", err, err)
			}
			if configurationError.Option() != "WithPublicBaseURL" || configurationError.Reason() != test.reason {
				t.Fatalf("configuration error = (%q, %q), want (%q, %q)", configurationError.Option(), configurationError.Reason(), "WithPublicBaseURL", test.reason)
			}
		})
	}

	client, err := NewClient(WithAPIKey("key"), WithPublicBaseURL("https://example.com/root%2Fsegment%23value///"))
	if err != nil {
		t.Fatal(err)
	}
	if client.config.publicBaseURL != "https://example.com/root%2Fsegment%23value" {
		t.Fatalf("public base URL = %q", client.config.publicBaseURL)
	}
	if got := client.config.publicEndpoint("/generate"); got != "https://example.com/root%2Fsegment%23value/generate" {
		t.Fatalf("public endpoint = %q", got)
	}
	if got := client.config.publicEndpoint("/models/a/../b%2Fc"); got != "https://example.com/root%2Fsegment%23value/models/a/../b%2Fc" {
		t.Fatalf("public endpoint was cleaned or re-escaped: %q", got)
	}
}

func TestPublicHeadersOwnedSet(t *testing.T) {
	client, err := NewClient(
		WithAPIKey("key"),
		WithTeam("team"),
		WithHeaders(http.Header{"X-Custom": {"first", "second"}}),
	)
	if err != nil {
		t.Fatal(err)
	}

	headers := client.config.buildPublicHeaders("Bearer key")
	want := http.Header{
		headerAuthorization: {"Bearer key"},
		headerContentType:   {"application/json"},
		headerTeam:          {"team"},
		"X-Custom":          {"first", "second"},
	}
	if !reflect.DeepEqual(headers, want) {
		t.Fatalf("public headers = %#v, want %#v", headers, want)
	}
	for _, name := range []string{headerGatewayProtocolVersion, headerGatewayAuthMethod, headerEvaluationModelSpecificationVersion, headerModelID} {
		if values := headers.Values(name); len(values) != 0 {
			t.Fatalf("public headers contain provider header %s: %#v", name, values)
		}
	}
}

func TestExistingEvaluationCompatibility(t *testing.T) {
	clearCredentialEnvironment(t)
	fixture := testserver.New(testserver.Response{
		Status: http.StatusOK,
		Body:   []byte(`{"answers":{"quality":{"type":"boolean","probability":1}}}`),
	})
	defer fixture.Close()

	client, err := NewClient(
		WithAPIKey("secret"),
		WithBaseURL(fixture.URL+"/v4/ai/"),
		WithPublicBaseURL("https://public.invalid/v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := EvaluationRequest{
		State: "candidate",
		Questions: map[string]Question{
			"quality": BooleanQuestion{Instructions: "Assess quality"},
		},
	}
	result, err := client.Evaluate(context.Background(), "provider/model", request)
	if err != nil {
		t.Fatal(err)
	}
	if answer, ok := result.Answers["quality"].(BooleanAnswer); !ok || answer.Probability != 1 {
		t.Fatalf("answer = %#v", result.Answers["quality"])
	}

	requests := fixture.Requests()
	if len(requests) != 1 {
		t.Fatalf("captured %d requests", len(requests))
	}
	got := requests[0]
	if got.URL != "/v4/ai/evaluation-model" || !bytes.Contains(got.Body, []byte(`"state":"candidate"`)) {
		t.Fatalf("provider request = %#v", got)
	}
	if got.Header.Get(headerGatewayProtocolVersion) != gatewayProtocolVersion || got.Header.Get(headerModelID) != "provider/model" {
		t.Fatalf("provider headers = %#v", got.Header)
	}
}
