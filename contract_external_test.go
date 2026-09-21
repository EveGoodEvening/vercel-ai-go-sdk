package gateway_test

import (
	"context"
	"net/http"
	"time"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

type tokenSource struct{}

func (tokenSource) Token(context.Context) (string, error) { return "token", nil }

func compilePublicContract() {
	var _ *gateway.Client
	var _ gateway.Option
	var _ gateway.TokenSource = tokenSource{}
	var _ func(...gateway.Option) (*gateway.Client, error) = gateway.NewClient
	var _ func(*gateway.Client, context.Context, string, gateway.EvaluationRequest) (*gateway.EvaluationResult, error) = (*gateway.Client).Evaluate
	var _ func(*gateway.Client, context.Context, gateway.ResponsesRequest) (*gateway.ResponseResult, error) = (*gateway.Client).CreateResponse
	var _ func(*gateway.Client, context.Context, gateway.ResponsesRequest) (*gateway.ResponseStream, error) = (*gateway.Client).StreamResponse
	var _ func(string) gateway.Option = gateway.WithAPIKey
	var _ func(string) gateway.Option = gateway.WithOIDCToken
	var _ func(gateway.TokenSource) gateway.Option = gateway.WithOIDCTokenSource
	var _ func(string) gateway.Option = gateway.WithBaseURL
	var _ func(string) gateway.Option = gateway.WithPublicBaseURL
	var _ func(*http.Client) gateway.Option = gateway.WithHTTPClient
	var _ func(string) gateway.Option = gateway.WithTeam
	var _ func(http.Header) gateway.Option = gateway.WithHeaders
	var _ func(gateway.RetryPolicy) gateway.Option = gateway.WithRetryPolicy
	description := "description"
	strict := true
	_ = gateway.ResponsesRequest{
		Model: "provider/model",
		Input: gateway.ResponseItemsInput{
			gateway.ResponseMessage{Role: "user", Content: "hello"},
			gateway.ResponseFunctionCall{ID: "item", CallID: "call", Name: "tool", Arguments: `{}`},
			gateway.ResponseFunctionCallOutput{CallID: "call", Output: `{}`},
		},
		MaxOutputTokens: new(128), Temperature: new(0.5), TopP: new(0.9),
		PresencePenalty: new(0.1), FrequencyPenalty: new(-0.1), Instructions: new("instruction"),
		Tools:      []gateway.ResponseTool{{Name: "tool", Description: &description, Parameters: map[string]any{"type": "object"}, Strict: &strict}},
		ToolChoice: gateway.ResponseSpecificToolChoice{Name: "tool"}, ParallelToolCalls: new(true),
		AllowedTools: []string{"tool"}, Reasoning: &gateway.ResponseReasoning{Effort: "high", Summary: new("auto")},
		Text:       &gateway.ResponseText{Format: gateway.ResponseJSONSchemaFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict}},
		Truncation: new("disabled"), PreviousResponseID: new("response"), Store: new(true),
		Metadata: map[string]string{"key": "value"}, Caching: new("auto"), CacheAnchorItems: new(1), CacheTTL: new("5m"), PromptCacheKey: new("cache"),
	}
	_ = gateway.ResponsesRequest{Model: "provider/model", Input: gateway.ResponseTextInput("hello"), ToolChoice: gateway.ResponseToolChoiceAuto, Text: &gateway.ResponseText{Format: gateway.ResponseTextFormatText}}
	_ = []gateway.ResponseToolChoiceMode{gateway.ResponseToolChoiceAuto, gateway.ResponseToolChoiceRequired, gateway.ResponseToolChoiceNone}
	_ = []gateway.ResponseTextFormatType{gateway.ResponseTextFormatText, gateway.ResponseTextFormatJSONObject}
	var responseResult *gateway.ResponseResult
	_ = responseResult.RawJSON()
	var responseStream *gateway.ResponseStream
	var responseEvent gateway.ResponseEvent
	_ = responseStream.Next()
	responseEvent = responseStream.Event()
	_, _ = responseStream.Err(), responseStream.Close()
	_ = gateway.ResponseOutputTextDeltaEvent{Type: "response.output_text.delta", Event: "event", ID: "id", Delta: "text"}
	rawResponseEvent := gateway.RawResponseEvent{Type: "future", Event: "event", ID: "id"}
	_ = rawResponseEvent.RawJSON()
	_ = responseEvent

	probabilityDecimals := 2
	scoreDecimals := 3
	inputTokens := int64(4)
	outputTokens := int64(5)
	details := "details"
	_ = gateway.RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Second,
		Multiplier:   2,
		Jitter:       0.5,
	}
	_ = gateway.EvaluationRequest{
		State: "state",
		Questions: map[string]gateway.Question{
			"boolean": gateway.BooleanQuestion{Instructions: "instruction"},
			"choice": gateway.ChoiceQuestion{
				Instructions: "instruction",
				Criteria:     map[string]any{"a": "A"},
			},
			"score": gateway.ScoreQuestion{
				Instructions: "instruction",
				Criteria:     []any{"low", "high"},
			},
		},
		ProviderOptions: map[string]map[string]any{"provider": {"key": "value"}},
	}
	_ = gateway.OptionalJSON{Set: true, Value: nil}
	_ = gateway.BooleanCriteria{}
	_ = gateway.EvaluationResult{
		Answers: map[string]gateway.Answer{
			"boolean": gateway.BooleanAnswer{Probability: 0.5},
			"choice":  gateway.ChoiceAnswer{Choice: "a", Probabilities: map[string]float64{"a": 1}},
			"score":   gateway.ScoreAnswer{Score: 1, Probabilities: map[string]float64{"0": 0, "1": 1}},
		},
		Rounding: &gateway.Rounding{
			ProbabilityDecimals: &probabilityDecimals,
			ScoreDecimals:       &scoreDecimals,
		},
		Usage: &gateway.Usage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
		Warnings: []gateway.Warning{{
			Type:    gateway.WarningUnsupported,
			Feature: "feature",
			Details: &details,
			Setting: "setting",
			Message: "message",
		}},
		ProviderMetadata: map[string]map[string]any{"provider": {}},
		Response:         gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.WarningType{
		gateway.WarningUnsupported,
		gateway.WarningCompatibility,
		gateway.WarningDeprecated,
		gateway.WarningOther,
	}

	var configurationError *gateway.ConfigurationError
	_, _, _ = configurationError.Error(), configurationError.Option(), configurationError.Reason()
	var validationError *gateway.ValidationError
	_, _, _ = validationError.Error(), validationError.Path(), validationError.Reason()
	var transportError *gateway.TransportError
	_, _, _ = transportError.Error(), transportError.Operation(), transportError.Unwrap()
	var responseError *gateway.ResponseError
	_, _, _, _, _, _ = responseError.Error(), responseError.Unwrap(), responseError.StatusCode(), responseError.Message(), responseError.Type(), responseError.Code()
	_, _, _, _, _, _ = responseError.Param(), responseError.GenerationID(), responseError.RequestID(), responseError.ResponseID(), responseError.Retryable(), responseError.BodyTruncated()
	_, _ = responseError.RetryAfter()
	_ = responseError.RawResponseBody()
	var responseValidationError *gateway.ResponseValidationError
	_, _, _, _, _, _ = responseValidationError.Error(), responseValidationError.Unwrap(), responseValidationError.StatusCode(), responseValidationError.Path(), responseValidationError.Reason(), responseValidationError.RequestID()
	_, _, _ = responseValidationError.ResponseID(), responseValidationError.BodyTruncated(), responseValidationError.RawResponseBody()
}
