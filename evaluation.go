package gateway

import (
	"context"
	"net/http"
	"time"
)

// EvaluationRequest contains shared State, a nonempty keyed Questions map, and
// optional ProviderOptions. State and question instructions/criteria are
// validated recursively as JSON-compatible input before network I/O.
type EvaluationRequest struct {
	State           any
	Questions       map[string]Question
	ProviderOptions map[string]map[string]any
}

// Evaluate validates and submits an Evaluation Model V4 request. A nil context
// is a ValidationError at $["context"]. Only HTTP status 200 is success; every
// other status is a ResponseError, and response Content-Type is ignored.
// Retries occur only when explicitly configured and may duplicate billable work.
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

// BooleanQuestion asks for P(true). Instructions is required and must be a
// JSON-compatible string, object, or array. Nil Criteria omits the wire field.
type BooleanQuestion struct {
	Instructions any
	Criteria     *BooleanCriteria
}

func (BooleanQuestion) questionType() string { return "boolean" }

// ChoiceQuestion asks the provider to select one keyed criterion. Instructions
// is required; Criteria must contain at least one JSON-compatible description.
type ChoiceQuestion struct {
	Instructions any
	Criteria     map[string]any
}

func (ChoiceQuestion) questionType() string { return "choice" }

// ScoreQuestion asks for a score over ordered criteria indexed from zero.
// Instructions is required; Criteria must contain at least two descriptions.
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

// BooleanCriteria optionally describes true and false. A non-nil zero value
// emits an empty criteria object; each OptionalJSON controls key presence.
type BooleanCriteria struct {
	True  OptionalJSON
	False OptionalJSON
}

// EvaluationResult is a strictly validated Evaluation Model V4 result. Missing
// warnings normalize to a non-nil empty slice; other metadata preserves the
// documented absent-versus-present distinction.
type EvaluationResult struct {
	Answers          map[string]Answer
	Rounding         *Rounding
	Usage            *Usage
	Warnings         []Warning
	ProviderMetadata map[string]map[string]any
	Response         ResponseMetadata
}

// Rounding reports provider decimal places. A nil Rounding object means the
// wire object was absent; an empty object is non-nil. Each nil field means its
// key was absent, while a non-nil pointer may contain zero and must be 0..15.
type Rounding struct {
	ProbabilityDecimals *int
	ScoreDecimals       *int
}

// Usage reports token counts. A nil Usage object means the wire object was
// absent; an empty object is non-nil. Each nil field means its key was absent;
// a non-nil pointer may contain zero and is a non-negative JSON integer that
// fits int64.
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
// caller-supplied ID. Headers and Body are defensive copies and non-nil after a
// successful bounded read, including an empty body; zero values mean no response
// metadata. Body is limited to 1 MiB and may contain echoed state or provider
// options, so it must be sanitized before logging or persistence.
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
