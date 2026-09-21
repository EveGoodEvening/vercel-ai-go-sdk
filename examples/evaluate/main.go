// Command evaluate demonstrates one Evaluation Model V4 request.
//
// Set AI_GATEWAY_API_KEY or VERCEL_OIDC_TOKEN before running it. The command
// may incur provider charges. It deliberately avoids printing raw bodies,
// headers, provider metadata values, request state, or credentials.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

const modelID = "typesafe-ai/jev-latest"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client, err := gateway.NewClient()
	if err != nil {
		printError(err)
		os.Exit(1)
	}

	result, err := client.Evaluate(ctx, modelID, gateway.EvaluationRequest{
		State: map[string]any{
			"response":  "Paris is the capital of France.",
			"reference": "The capital of France is Paris.",
		},
		Questions: map[string]gateway.Question{
			"factually-correct": gateway.BooleanQuestion{
				Instructions: "Determine whether the response is factually correct according to the reference.",
				Criteria: &gateway.BooleanCriteria{
					True:  gateway.OptionalJSON{Set: true, Value: "The response agrees with the reference."},
					False: gateway.OptionalJSON{Set: true, Value: "The response contradicts or is unsupported by the reference."},
				},
			},
			"quality": gateway.ChoiceQuestion{
				Instructions: "Choose the description that best matches the response quality.",
				Criteria: map[string]any{
					"good": "Correct, relevant, and clear.",
					"poor": "Incorrect, irrelevant, or unclear.",
				},
			},
			"score": gateway.ScoreQuestion{
				Instructions: "Score the factual quality from the ordered levels.",
				Criteria: []any{
					"Incorrect.",
					"Partially correct.",
					"Fully correct.",
				},
			},
		},
	})
	if err != nil {
		printError(err)
		os.Exit(1)
	}

	for id, answer := range result.Answers {
		switch answer := answer.(type) {
		case gateway.BooleanAnswer:
			fmt.Printf("answer %q: boolean probability=%g\n", id, answer.Probability)
		case gateway.ChoiceAnswer:
			fmt.Printf("answer %q: choice=%q probabilities_present=%t\n", id, answer.Choice, answer.Probabilities != nil)
		case gateway.ScoreAnswer:
			fmt.Printf("answer %q: score=%g probabilities_present=%t\n", id, answer.Score, answer.Probabilities != nil)
		}
	}

	fmt.Printf("response: model=%q headers_present=%t body_bytes=%d\n",
		result.Response.ModelID, result.Response.Headers != nil, len(result.Response.Body))
	fmt.Printf("metadata: rounding_present=%t usage_present=%t warnings=%d provider_metadata_present=%t\n",
		result.Rounding != nil, result.Usage != nil, len(result.Warnings), result.ProviderMetadata != nil)
}

func printError(err error) {
	var configurationErr *gateway.ConfigurationError
	var validationErr *gateway.ValidationError
	var transportErr *gateway.TransportError
	var responseErr *gateway.ResponseError
	var responseValidationErr *gateway.ResponseValidationError

	switch {
	case errors.As(err, &configurationErr):
		log.Printf("configuration error: option=%q reason=%q", configurationErr.Option(), configurationErr.Reason())
	case errors.As(err, &validationErr):
		log.Printf("validation error: path=%q reason=%q", validationErr.Path(), validationErr.Reason())
	case errors.As(err, &responseValidationErr):
		log.Printf("response validation error: status=%d path=%q reason=%q request_id=%q response_id=%q truncated=%t",
			responseValidationErr.StatusCode(), responseValidationErr.Path(), responseValidationErr.Reason(),
			responseValidationErr.RequestID(), responseValidationErr.ResponseID(), responseValidationErr.BodyTruncated())
	case errors.As(err, &responseErr):
		retryAfter, hasRetryAfter := responseErr.RetryAfter()
		log.Printf("response error: status=%d type=%q code=%q request_id=%q response_id=%q generation_id=%q retryable=%t retry_after=%s retry_after_present=%t truncated=%t",
			responseErr.StatusCode(), responseErr.Type(), responseErr.Code(), responseErr.RequestID(),
			responseErr.ResponseID(), responseErr.GenerationID(), responseErr.Retryable(), retryAfter,
			hasRetryAfter, responseErr.BodyTruncated())
	case errors.As(err, &transportErr):
		log.Printf("transport error: operation=%q", transportErr.Operation())
	default:
		log.Printf("evaluation failed: %T", err)
	}
}
