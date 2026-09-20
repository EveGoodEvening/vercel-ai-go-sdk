package gateway

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestProtectedHeaderNames(t *testing.T) {
	want := map[string]struct{}{
		"Authorization":                             {},
		"Content-Type":                              {},
		"Ai-Gateway-Protocol-Version":               {},
		"Ai-Gateway-Auth-Method":                    {},
		"Ai-Evaluation-Model-Specification-Version": {},
		"Ai-Model-Id":                               {},
		"X-Vercel-Ai-Gateway-Team":                  {},
	}
	if !reflect.DeepEqual(protectedHeaderNames, want) {
		t.Fatalf("protected headers = %#v, want %#v", protectedHeaderNames, want)
	}
}

func TestWithHeadersRejectsProtectedNamesCaseInsensitively(t *testing.T) {
	values := [][]string{nil, {}, {""}, {"caller value"}}
	for protectedName := range protectedHeaderNames {
		for _, name := range []string{protectedName, strings.ToLower(protectedName), strings.ToUpper(protectedName)} {
			for _, value := range values {
				t.Run(name, func(t *testing.T) {
					clearCredentialEnvironment(t)
					headers := http.Header{name: value}
					_, err := NewClient(WithHeaders(headers))
					assertConfigurationError(t, err, "WithHeaders", "contains protected header")
				})
			}
		}
	}
}

func TestWithHeadersAcceptsNilAndPreservesCallerValues(t *testing.T) {
	clearCredentialEnvironment(t)
	client, err := NewClient(WithAPIKey("key"), WithHeaders(nil))
	if err != nil {
		t.Fatal(err)
	}
	if client.config.headers != nil {
		t.Fatalf("nil headers stored as %#v", client.config.headers)
	}

	input := http.Header{
		"x-original-spelling": {"first", "", "first", "last"},
		"X-Nil":               nil,
		"X-Empty":             {},
	}
	client, err = NewClient(WithAPIKey("key"), WithHeaders(input))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.config.headers, input) {
		t.Fatalf("stored headers = %#v, want %#v", client.config.headers, input)
	}
}

func TestWithHeadersDefensiveCopiesAtOptionApplication(t *testing.T) {
	clearCredentialEnvironment(t)
	input := http.Header{"X-Custom": {"first", "second"}}
	option := WithHeaders(input)
	config := clientConfig{}
	if err := option(&config); err != nil {
		t.Fatal(err)
	}
	input["X-Custom"][0] = "mutated"
	input["X-Custom"] = append(input["X-Custom"], "third")
	input["X-New"] = []string{"new"}
	want := http.Header{"X-Custom": {"first", "second"}}
	if !reflect.DeepEqual(config.headers, want) {
		t.Fatalf("stored headers changed with caller map: %#v", config.headers)
	}
}

func TestBuildHeadersClonesStoredHeadersPerRequest(t *testing.T) {
	config := clientConfig{
		team: " team ",
		headers: http.Header{
			"X-Custom": {"first", "", "first", "last"},
			"X-Nil":    nil,
			"X-Empty":  {},
		},
	}
	first := config.buildHeaders("Bearer token", "oidc", "provider/model")
	second := config.buildHeaders("Bearer token-2", "api-key", "provider/model-2")

	if !reflect.DeepEqual(first.Values("X-Custom"), []string{"first", "", "first", "last"}) {
		t.Fatalf("custom values = %#v", first.Values("X-Custom"))
	}
	if _, present := first["X-Nil"]; !present || first["X-Nil"] != nil {
		t.Fatalf("nil slice not preserved: %#v", first["X-Nil"])
	}
	if value, present := first["X-Empty"]; !present || value == nil || len(value) != 0 {
		t.Fatalf("empty slice not preserved: %#v, present %v", value, present)
	}
	wantOwned := map[string]string{
		headerAuthorization:                       "Bearer token",
		headerContentType:                         "application/json",
		headerGatewayProtocolVersion:              gatewayProtocolVersion,
		headerGatewayAuthMethod:                   "oidc",
		headerEvaluationModelSpecificationVersion: evaluationModelSpecificationVersion,
		headerModelID:                             "provider/model",
		headerTeam:                                " team ",
	}
	for name, want := range wantOwned {
		if got := first.Get(name); got != want || len(first.Values(name)) != 1 {
			t.Fatalf("header %s = %#v, want one value %q", name, first.Values(name), want)
		}
	}

	first["X-Custom"][0] = "mutated"
	first.Set(headerAuthorization, "mutated")
	if config.headers["X-Custom"][0] != "first" {
		t.Fatalf("request mutated stored headers: %#v", config.headers)
	}
	if second.Get("X-Custom") != "first" || second.Get(headerAuthorization) != "Bearer token-2" || second.Get(headerGatewayAuthMethod) != "api-key" || second.Get(headerModelID) != "provider/model-2" {
		t.Fatalf("second request headers were not independent: %#v", second)
	}
}

func TestWithTeamValidationAndPreservation(t *testing.T) {
	for _, value := range []string{"", " \t", "\u2003"} {
		t.Run(value, func(t *testing.T) {
			clearCredentialEnvironment(t)
			_, err := NewClient(WithTeam(value))
			assertConfigurationError(t, err, "WithTeam", "must not be empty or whitespace")
		})
	}

	clearCredentialEnvironment(t)
	client, err := NewClient(WithAPIKey("key"), WithTeam(" team slug "))
	if err != nil {
		t.Fatal(err)
	}
	if client.config.team != " team slug " {
		t.Fatalf("team = %q", client.config.team)
	}
}
