//go:build livecontract

package livecontract_test

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

const (
	liveModelID              = "typesafe-ai/jev"
	costAck                  = "I_ACCEPT_LIVE_EVALUATION_COSTS"
	evaluationAckEnv         = "AI_GATEWAY_LIVE_COST_ACK"
	evaluationOppositeAckEnv = "AI_GATEWAY_PUBLIC_LIVE_COST_ACK"
	evaluationAPIKeyEnv      = "AI_GATEWAY_API_KEY"
	evaluationOIDCTokenEnv   = "VERCEL_OIDC_TOKEN"
	bodyLimit                = 1 << 20
)

func TestGatewayEvaluationContract(t *testing.T) {
	credential := requireLivePrerequisites(t)

	client, err := gateway.NewClient(credential)
	if err != nil {
		t.Fatalf("construct live Gateway client: %v", err)
	}

	result, err := client.Evaluate(context.Background(), liveModelID, gateway.EvaluationRequest{
		State: map[string]any{
			"candidate": "The service returned HTTP 200 in 120 milliseconds.",
			"target":    "The service request succeeded in under 500 milliseconds.",
		},
		Questions: map[string]gateway.Question{
			"boolean": gateway.BooleanQuestion{
				Instructions: "Estimate whether the candidate satisfies the target.",
				Criteria: &gateway.BooleanCriteria{
					True:  gateway.OptionalJSON{Set: true, Value: "The request succeeded and latency was below 500 milliseconds."},
					False: gateway.OptionalJSON{Set: true, Value: "The request failed or latency was at least 500 milliseconds."},
				},
			},
			"choice": gateway.ChoiceQuestion{
				Instructions: "Choose the outcome that best describes the candidate.",
				Criteria: map[string]any{
					"meets-target":  "The request succeeded in under 500 milliseconds.",
					"misses-target": "The request failed or took at least 500 milliseconds.",
				},
			},
			"score": gateway.ScoreQuestion{
				Instructions: "Score how completely the candidate satisfies the target.",
				Criteria: []any{
					"Does not satisfy the target.",
					"Partially satisfies the target.",
					"Fully satisfies the target.",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluate live Gateway contract: %v", err)
	}

	if len(result.Answers) != 3 {
		t.Fatalf("answer count = %d, want 3", len(result.Answers))
	}
	booleanAnswer, ok := result.Answers["boolean"].(gateway.BooleanAnswer)
	if !ok {
		t.Fatalf("boolean answer type = %T, want gateway.BooleanAnswer", result.Answers["boolean"])
	}
	assertProbability(t, "boolean probability", booleanAnswer.Probability)

	choiceAnswer, ok := result.Answers["choice"].(gateway.ChoiceAnswer)
	if !ok {
		t.Fatalf("choice answer type = %T, want gateway.ChoiceAnswer", result.Answers["choice"])
	}
	if choiceAnswer.Choice != "meets-target" && choiceAnswer.Choice != "misses-target" {
		t.Fatalf("choice = %q, want a declared criterion", choiceAnswer.Choice)
	}
	assertProbabilities(t, "choice probabilities", choiceAnswer.Probabilities)

	scoreAnswer, ok := result.Answers["score"].(gateway.ScoreAnswer)
	if !ok {
		t.Fatalf("score answer type = %T, want gateway.ScoreAnswer", result.Answers["score"])
	}
	if math.IsNaN(scoreAnswer.Score) || math.IsInf(scoreAnswer.Score, 0) || scoreAnswer.Score < 0 || scoreAnswer.Score > 2 {
		t.Fatalf("score = %v, want a finite value in [0,2]", scoreAnswer.Score)
	}
	assertProbabilities(t, "score probabilities", scoreAnswer.Probabilities)

	assertOptionalContract(t, result)
	if result.Response.ModelID != liveModelID {
		t.Fatalf("response model ID = %q, want %q", result.Response.ModelID, liveModelID)
	}
	if result.Response.Headers == nil {
		t.Fatal("response headers are nil")
	}
	if result.Response.Body == nil {
		t.Fatal("response body is nil")
	}
	if len(result.Response.Body) > bodyLimit {
		t.Fatalf("response body length = %d, want at most %d", len(result.Response.Body), bodyLimit)
	}
}

func requireLivePrerequisites(t *testing.T) gateway.Option {
	t.Helper()
	if os.Getenv(evaluationAckEnv) != costAck {
		t.Fatalf("live Gateway evaluation contract requires %s=%s exactly", evaluationAckEnv, costAck)
	}
	if _, present := os.LookupEnv(evaluationOppositeAckEnv); present {
		t.Fatalf("live Gateway evaluation contract requires opposite acknowledgement %s to be unset", evaluationOppositeAckEnv)
	}

	apiKey := os.Getenv(evaluationAPIKeyEnv)
	oidcToken := os.Getenv(evaluationOIDCTokenEnv)
	hasAPIKey := strings.TrimSpace(apiKey) != ""
	hasOIDCToken := strings.TrimSpace(oidcToken) != ""
	if hasAPIKey == hasOIDCToken {
		t.Fatalf("live Gateway evaluation contract requires exactly one non-empty credential across %s and %s", evaluationAPIKeyEnv, evaluationOIDCTokenEnv)
	}
	if hasAPIKey {
		return gateway.WithAPIKey(apiKey)
	}
	return gateway.WithOIDCToken(oidcToken)
}

func assertProbability(t *testing.T, name string, probability float64) {
	t.Helper()
	if math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 || probability > 1 {
		t.Fatalf("%s = %v, want a finite value in [0,1]", name, probability)
	}
}

func assertProbabilities(t *testing.T, name string, probabilities map[string]float64) {
	t.Helper()
	for key, probability := range probabilities {
		assertProbability(t, name+"["+key+"]", probability)
	}
}

func assertOptionalContract(t *testing.T, result *gateway.EvaluationResult) {
	t.Helper()
	if result.Rounding != nil {
		if result.Rounding.ProbabilityDecimals != nil && *result.Rounding.ProbabilityDecimals < 0 {
			t.Fatalf("probability decimals = %d, want non-negative", *result.Rounding.ProbabilityDecimals)
		}
		if result.Rounding.ScoreDecimals != nil && *result.Rounding.ScoreDecimals < 0 {
			t.Fatalf("score decimals = %d, want non-negative", *result.Rounding.ScoreDecimals)
		}
	}
	if result.Usage != nil {
		if result.Usage.InputTokens != nil && *result.Usage.InputTokens < 0 {
			t.Fatalf("input tokens = %d, want non-negative", *result.Usage.InputTokens)
		}
		if result.Usage.OutputTokens != nil && *result.Usage.OutputTokens < 0 {
			t.Fatalf("output tokens = %d, want non-negative", *result.Usage.OutputTokens)
		}
	}
	for index, warning := range result.Warnings {
		switch warning.Type {
		case gateway.WarningUnsupported, gateway.WarningCompatibility:
			if warning.Setting != "" || warning.Message != "" {
				t.Fatalf("warning %d violates %q discriminator contract", index, warning.Type)
			}
		case gateway.WarningDeprecated:
			if warning.Feature != "" || warning.Details != nil {
				t.Fatalf("warning %d violates deprecated discriminator contract", index)
			}
		case gateway.WarningOther:
			if warning.Feature != "" || warning.Details != nil || warning.Setting != "" {
				t.Fatalf("warning %d violates other discriminator contract", index)
			}
		default:
			t.Fatalf("warning %d has unknown type %q", index, warning.Type)
		}
	}
	for provider, metadata := range result.ProviderMetadata {
		if metadata == nil {
			t.Fatalf("provider metadata contains invalid entry for %q", provider)
		}
	}
}
