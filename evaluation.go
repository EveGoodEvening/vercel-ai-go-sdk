package gateway

import (
	"context"
	"net/http"
	"time"
)

// EvaluationRequest contains shared state, keyed questions, and optional
// provider-specific options.
type EvaluationRequest struct {
	State           any
	Questions       map[string]Question
	ProviderOptions map[string]map[string]any
}

// Evaluate validates and submits an Evaluation Model V4 request. Only HTTP
// status 200 is treated as success; every other response is returned as a
// ResponseError with bounded response diagnostics.
func (client *Client) Evaluate(ctx context.Context, modelID string, request EvaluationRequest) (*EvaluationResult, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "required")
	}
	if err := validateEvaluationRequest(modelID, request); err != nil {
		return nil, err
	}
	payload, err := encodeEvaluationRequest(request)
	if err != nil {
		return nil, &TransportError{operation: "encode request", cause: err}
	}
	for attempt := 1; ; attempt++ {
		raw, err := client.executeEvaluationRequest(ctx, modelID, payload)
		if err != nil {
			return nil, err
		}
		if raw.statusCode == http.StatusOK {
			if raw.bodyErr != nil {
				return nil, &TransportError{operation: "read response body", cause: raw.bodyErr}
			}
			return composeEvaluationResult(modelID, request.Questions, raw)
		}

		now := time.Now()
		if client.config.retryHooks.now != nil {
			now = client.config.retryHooks.now()
		}
		responseErr := composeResponseError(raw, now)
		if !client.canRetry(ctx, attempt, responseErr) {
			return nil, responseErr
		}
		if err := client.waitForRetry(ctx, attempt, responseErr); err != nil {
			return nil, err
		}
	}
}

// Question is the closed set of supported evaluation question variants.
type Question interface {
	questionType() string
}

// BooleanQuestion asks for the probability that its criteria are true.
type BooleanQuestion struct {
	Instructions any
	Criteria     *BooleanCriteria
}

func (BooleanQuestion) questionType() string { return "boolean" }

// ChoiceQuestion asks the provider to select one keyed criterion.
type ChoiceQuestion struct {
	Instructions any
	Criteria     map[string]any
}

func (ChoiceQuestion) questionType() string { return "choice" }

// ScoreQuestion asks for a score among ordered criteria levels.
type ScoreQuestion struct {
	Instructions any
	Criteria     []any
}

func (ScoreQuestion) questionType() string { return "score" }

// OptionalJSON distinguishes an omitted value from an explicitly present JSON
// null. Set=false omits the containing key and requires Value=nil; Set=true
// emits Value, including nil as JSON null.
type OptionalJSON struct {
	Set   bool
	Value any
}

// BooleanCriteria optionally describes the true and false outcomes.
type BooleanCriteria struct {
	True  OptionalJSON
	False OptionalJSON
}

// EvaluationResult is a validated Evaluation Model V4 result.
type EvaluationResult struct {
	Answers          map[string]Answer
	Rounding         *Rounding
	Usage            *Usage
	Warnings         []Warning
	ProviderMetadata map[string]map[string]any
	Response         ResponseMetadata
}

// Rounding reports decimal places used by the provider. A nil field means the
// corresponding wire key was absent; a non-nil pointer may contain zero.
type Rounding struct {
	ProbabilityDecimals *int
	ScoreDecimals       *int
}

// Usage reports token counts. A nil field means the corresponding wire key was
// absent; a non-nil pointer may contain zero. Present counts are finite,
// non-negative integers representable as int64.
type Usage struct {
	InputTokens  *int64
	OutputTokens *int64
}

// WarningType identifies an evaluation warning variant.
type WarningType string

const (
	// WarningUnsupported reports an unsupported feature.
	WarningUnsupported WarningType = "unsupported"
	// WarningCompatibility reports compatibility behavior.
	WarningCompatibility WarningType = "compatibility"
	// WarningDeprecated reports a deprecated setting.
	WarningDeprecated WarningType = "deprecated"
	// WarningOther reports another provider warning.
	WarningOther WarningType = "other"
)

// Warning is the lossless public form of one wire warning. Unsupported and
// compatibility warnings require Feature and may set Details; deprecated
// warnings require Setting and Message; other warnings require Message. Nil
// Details distinguishes an omitted details key from an empty string.
type Warning struct {
	Type    WarningType
	Feature string
	Details *string
	Setting string
	Message string
}

// ResponseMetadata describes a successful HTTP response. ModelID is the
// caller-supplied model ID. Headers and Body are defensive copies and are
// non-nil after a successfully read response, including an empty body; their
// zero values mean no response metadata. Body may contain echoed request state
// or provider options and must be sanitized before logging or persistence.
type ResponseMetadata struct {
	ModelID string
	Headers http.Header
	Body    []byte
}

// Answer is the closed set of supported evaluation answer variants.
type Answer interface {
	answerType() string
}

// BooleanAnswer reports P(true).
type BooleanAnswer struct {
	Probability float64
}

func (BooleanAnswer) answerType() string { return "boolean" }

// ChoiceAnswer reports a selected criterion and, when present, its probability
// distribution. A nil Probabilities map means the wire key was absent.
type ChoiceAnswer struct {
	Choice        string
	Probabilities map[string]float64
}

func (ChoiceAnswer) answerType() string { return "choice" }

// ScoreAnswer reports a score and, when present, its probability distribution.
// A nil Probabilities map means the wire key was absent.
type ScoreAnswer struct {
	Score         float64
	Probabilities map[string]float64
}

func (ScoreAnswer) answerType() string { return "score" }
