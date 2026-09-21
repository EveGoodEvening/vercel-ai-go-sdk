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

	booleanAnswers, choiceAnswers, scoreAnswers, unknownAnswers := 0, 0, 0, 0
	for _, answer := range result.Answers {
		switch answer.(type) {
		case gateway.BooleanAnswer:
			booleanAnswers++
		case gateway.ChoiceAnswer:
			choiceAnswers++
		case gateway.ScoreAnswer:
			scoreAnswers++
		default:
			unknownAnswers++
		}
	}
	fmt.Printf("evaluation completed: answers=%d boolean=%d choice=%d score=%d unknown=%d\n",
		len(result.Answers), booleanAnswers, choiceAnswers, scoreAnswers, unknownAnswers)
	fmt.Printf("response: headers_present=%t body_bytes=%d\n",
		result.Response.Headers != nil, len(result.Response.Body))
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
		log.Printf("configuration error: option=%q", configurationErr.Option())
	case errors.As(err, &validationErr):
		log.Printf("validation error: path=%q", validationErr.Path())
	case errors.As(err, &responseValidationErr):
		log.Printf("response validation error: status=%d path=%q request_id_present=%t response_id_present=%t truncated=%t",
			responseValidationErr.StatusCode(), responseValidationErr.Path(), responseValidationErr.RequestID() != "",
			responseValidationErr.ResponseID() != "", responseValidationErr.BodyTruncated())
	case errors.As(err, &responseErr):
		_, hasRetryAfter := responseErr.RetryAfter()
		log.Printf("response error: status=%d request_id_present=%t response_id_present=%t generation_id_present=%t retryable=%t retry_after_present=%t truncated=%t",
			responseErr.StatusCode(), responseErr.RequestID() != "", responseErr.ResponseID() != "",
			responseErr.GenerationID() != "", responseErr.Retryable(), hasRetryAfter, responseErr.BodyTruncated())
	case errors.As(err, &transportErr):
		log.Printf("transport error: operation=%q", transportErr.Operation())
	default:
		log.Printf("evaluation failed: %T", err)
	}
}
