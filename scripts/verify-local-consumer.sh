#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
consumer_dir=$(mktemp -d "${TMPDIR:-/tmp}/gateway-local-consumer.XXXXXX")
trap 'rm -rf -- "$consumer_dir"' EXIT

cat >"$consumer_dir/go.mod" <<EOF
module example.com/gateway-local-consumer

go 1.26

require github.com/EveGoodEvening/vercel-ai-go-sdk v0.0.0

replace github.com/EveGoodEvening/vercel-ai-go-sdk => $repo_root
EOF

cat >"$consumer_dir/main.go" <<'EOF'
package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

type tokenSource struct{}

func (tokenSource) Token(context.Context) (string, error) { return "token", nil }

func compileRequest(client *gateway.Client) (*gateway.EvaluationResult, error) {
	zero := 0
	zero64 := int64(0)
	details := "details"

	request := gateway.EvaluationRequest{
		State: map[string]any{"candidate": "answer"},
		Questions: map[string]gateway.Question{
			"boolean": gateway.BooleanQuestion{Instructions: "judge", Criteria: &gateway.BooleanCriteria{
				True:  gateway.OptionalJSON{Set: true, Value: "yes"},
				False: gateway.OptionalJSON{Set: true, Value: "no"},
			}},
			"choice": gateway.ChoiceQuestion{Instructions: "choose", Criteria: map[string]any{"a": "A"}},
			"score": gateway.ScoreQuestion{Instructions: "score", Criteria: []any{"low", "high"}},
		},
		ProviderOptions: map[string]map[string]any{"provider": {"flag": true}},
	}

	_ = gateway.RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Second, Multiplier: 2, Jitter: 0}
	_ = gateway.EvaluationResult{
		Answers: map[string]gateway.Answer{
			"boolean": gateway.BooleanAnswer{Probability: 1},
			"choice": gateway.ChoiceAnswer{Choice: "a", Probabilities: map[string]float64{"a": 1}},
			"score": gateway.ScoreAnswer{Score: 1, Probabilities: map[string]float64{"1": 1}},
		},
		Rounding: &gateway.Rounding{ProbabilityDecimals: &zero, ScoreDecimals: &zero},
		Usage: &gateway.Usage{InputTokens: &zero64, OutputTokens: &zero64},
		Warnings: []gateway.Warning{{Type: gateway.WarningUnsupported, Feature: "feature", Details: &details}},
		ProviderMetadata: map[string]map[string]any{"provider": {"key": "value"}},
		Response: gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.WarningType{gateway.WarningUnsupported, gateway.WarningCompatibility, gateway.WarningDeprecated, gateway.WarningOther}

	return client.Evaluate(context.Background(), "provider/model", request)
}

func inspectErrors(err error) {
	var configuration *gateway.ConfigurationError
	if errors.As(err, &configuration) {
		_, _ = configuration.Option(), configuration.Reason()
	}
	var validation *gateway.ValidationError
	if errors.As(err, &validation) {
		_, _ = validation.Path(), validation.Reason()
	}
	var transport *gateway.TransportError
	if errors.As(err, &transport) {
		_, _ = transport.Operation(), transport.Unwrap()
	}
	var response *gateway.ResponseError
	if errors.As(err, &response) {
		_, _, _, _ = response.StatusCode(), response.Message(), response.Type(), response.Code()
		_, _, _, _ = response.Param(), response.GenerationID(), response.RequestID(), response.ResponseID()
		_, _ = response.RetryAfter()
		_, _, _, _ = response.Retryable(), response.BodyTruncated(), response.RawResponseBody(), response.Unwrap()
	}
	var responseValidation *gateway.ResponseValidationError
	if errors.As(err, &responseValidation) {
		_, _, _ = responseValidation.StatusCode(), responseValidation.Path(), responseValidation.Reason()
		_, _, _, _ = responseValidation.RequestID(), responseValidation.ResponseID(), responseValidation.BodyTruncated(), responseValidation.RawResponseBody()
		_ = responseValidation.Unwrap()
	}
}

func main() {
	client, err := gateway.NewClient(
		gateway.WithAPIKey("key"),
		gateway.WithOIDCToken("token"),
		gateway.WithOIDCTokenSource(tokenSource{}),
		gateway.WithBaseURL("http://127.0.0.1"),
		gateway.WithHTTPClient(&http.Client{}),
		gateway.WithTeam("team"),
		gateway.WithHeaders(http.Header{"X-Consumer": {"value"}}),
		gateway.WithRetryPolicy(gateway.RetryPolicy{}),
	)
	inspectErrors(err)
	if client != nil {
		_ = compileRequest
	}
}
EOF

cat >"$consumer_dir/check_api.go" <<'EOF'
//go:build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	entries, err := os.ReadDir(os.Args[1])
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(os.Args[1], entry.Name()), nil, 0)
		if err != nil {
			panic(err)
		}
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range typed.Specs {
					typeSpec, ok := specification.(*ast.TypeSpec)
					if !ok || !ast.IsExported(typeSpec.Name.Name) {
						continue
					}
					if typeSpec.Assign.IsValid() {
						fmt.Fprintf(os.Stderr, "exported compatibility alias found: %s\n", typeSpec.Name.Name)
						os.Exit(1)
					}
					if typeSpec.Name.Name == "ConfigError" || strings.Contains(typeSpec.Name.Name, "RetryHook") {
						fmt.Fprintf(os.Stderr, "forbidden compatibility or retry-hook export found: %s\n", typeSpec.Name.Name)
						os.Exit(1)
					}
				}
			case *ast.FuncDecl:
				if ast.IsExported(typed.Name.Name) && strings.Contains(typed.Name.Name, "RetryHook") {
					fmt.Fprintf(os.Stderr, "forbidden retry-hook export found: %s\n", typed.Name.Name)
					os.Exit(1)
				}
			}
		}
	}
}
EOF

env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK GOWORK=off GOPROXY=off go run "$consumer_dir/check_api.go" "$repo_root"

cd "$consumer_dir"
env -u AI_GATEWAY_API_KEY -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK GOWORK=off GOPROXY=off go build ./...
