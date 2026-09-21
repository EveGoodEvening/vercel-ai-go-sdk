package gateway

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
)

type unsupportedQuestion struct{}

func (unsupportedQuestion) questionType() string { return "unsupported" }

func validRequest() EvaluationRequest {
	return EvaluationRequest{State: map[string]any{"items": []any{"x", true, nil, 3.0}}, Questions: map[string]Question{
		"boolean": BooleanQuestion{Instructions: "is it good?"},
		"choice":  ChoiceQuestion{Instructions: map[string]any{"task": "choose"}, Criteria: map[string]any{"a": "A"}},
		"score":   ScoreQuestion{Instructions: []any{"score"}, Criteria: []any{"low", "high"}},
	}}
}

func requireValidationError(t *testing.T, err error, path, reason string) {
	t.Helper()
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if validation.Path() != path || validation.Reason() != reason {
		t.Fatalf("got (%q, %q), want (%q, %q)", validation.Path(), validation.Reason(), path, reason)
	}
}

func TestValidateEvaluationRequestAcceptsContract(t *testing.T) {
	requests := []EvaluationRequest{validRequest(), {State: []any{"shared", map[string]any{"n": 1}}, Questions: map[string]Question{"q": BooleanQuestion{Instructions: []any{"judge"}, Criteria: &BooleanCriteria{True: OptionalJSON{Set: true}, False: OptionalJSON{Set: true, Value: map[string]any{"label": "no"}}}}}, ProviderOptions: map[string]map[string]any{}}, {State: "shared", Questions: map[string]Question{"q": ChoiceQuestion{Instructions: "choose", Criteria: map[string]any{"x": nil}}}, ProviderOptions: map[string]map[string]any{"acme": {"flag": true}}}}
	for index, request := range requests {
		if err := validateEvaluationRequest("provider/model", request); err != nil {
			t.Fatalf("case %d: %v", index, err)
		}
	}
}

func TestValidateEvaluationRequestCanonicalFailures(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	sorted := map[string]any{"z": func() {}, "a": make(chan int)}
	cases := []struct {
		name         string
		model        string
		request      EvaluationRequest
		path, reason string
	}{
		{"model", "", validRequest(), `$["modelID"]`, "must be nonempty"},
		{"state required", "m", EvaluationRequest{Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"]`, "required"},
		{"state shape", "m", EvaluationRequest{State: true, Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"]`, "must be a string, object, or array"},
		{"questions", "m", EvaluationRequest{State: "x", Questions: map[string]Question{}}, `$["questions"]`, "must be nonempty"},
		{"empty id", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"": BooleanQuestion{Instructions: "x"}}}, `$["questions"][""]`, "must be nonempty"},
		{"nil question", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": (*BooleanQuestion)(nil)}}, `$["questions"]["q"]`, "must be a non-nil question"},
		{"unsupported", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": unsupportedQuestion{}}}, `$["questions"]["q"]`, "question type is unsupported"},
		{"instructions required", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{}}}, `$["questions"]["q"]["instructions"]`, "required"},
		{"instructions shape", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: 1}}}, `$["questions"]["q"]["instructions"]`, "must be a string, object, or array"},
		{"instructions null", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: map[string]any(nil)}}}, `$["questions"]["q"]["instructions"]`, "required"},
		{"instructions invalid nested", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: map[string]any{"bad": func() {}}}}}, `$["questions"]["q"]["instructions"]["bad"]`, "must be JSON-compatible"},
		{"choice count", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": ChoiceQuestion{Instructions: "x"}}}, `$["questions"]["q"]["criteria"]`, "must contain at least one option"},
		{"score count", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": ScoreQuestion{Instructions: "x", Criteria: []any{"one"}}}}, `$["questions"]["q"]["criteria"]`, "must contain at least two levels"},
		{"optional", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x", Criteria: &BooleanCriteria{True: OptionalJSON{Value: "bad"}}}}}, `$["questions"]["q"]["criteria"]["true"]`, "must be omitted when Set is false"},
		{"provider nil", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}, ProviderOptions: map[string]map[string]any{"acme": nil}}, `$["providerOptions"]["acme"]`, "must be a non-nil object"},
		{"provider invalid nested", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}, ProviderOptions: map[string]map[string]any{"acme": {"nested": []any{"ok", make(chan int)}}}}, `$["providerOptions"]["acme"]["nested"][1]`, "must be JSON-compatible"},
		{"map keys", "m", EvaluationRequest{State: map[int]string{1: "x"}, Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"]`, "map keys must be strings"},
		{"function", "m", EvaluationRequest{State: map[string]any{"bad": func() {}}, Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"]["bad"]`, "must be JSON-compatible"},
		{"channel sorted", "m", EvaluationRequest{State: sorted, Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"]["a"]`, "must be JSON-compatible"},
		{"cycle", "m", EvaluationRequest{State: cycle, Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"]["self"]`, "cycle detected"},
		{"nan", "m", EvaluationRequest{State: []any{math.NaN()}, Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}}, `$["state"][0]`, "number must be finite"},
		{"infinity escaped", "m", EvaluationRequest{State: "x", Questions: map[string]Question{"a\"b\\c\n": ChoiceQuestion{Instructions: "x", Criteria: map[string]any{"k": []any{math.Inf(1)}}}}}, `$["questions"]["a\"b\\c\n"]["criteria"]["k"][0]`, "number must be finite"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			requireValidationError(t, validateEvaluationRequest(test.model, test.request), test.path, test.reason)
		})
	}
}

func TestJSONValidationRootAndQuotedPaths(t *testing.T) {
	requireValidationError(t, validateJSONValue(func() {}, "$"), "$", "must be JSON-compatible")
	requireValidationError(t, validateJSONInput(nil, "$"), "$", "required")
	value := map[string]any{"quote\"slash\\control\x01": math.Inf(-1)}
	requireValidationError(t, validateJSONValue(value, "$"), `$["quote\"slash\\control\x01"]`, "number must be finite")
}

func TestEvaluateValidationErrorFormattingRedactsOversizedKeys(t *testing.T) {
	const marker = "evaluation-validation-secret-marker"
	key := strings.Repeat(marker, 4096)

	tests := []struct {
		name    string
		request EvaluationRequest
		path    string
	}{
		{
			name:    "question key",
			request: EvaluationRequest{State: "state", Questions: map[string]Question{key: (*BooleanQuestion)(nil)}},
			path:    `$["questions"]["` + key + `"]`,
		},
		{
			name: "provider key",
			request: EvaluationRequest{
				State:           "state",
				Questions:       map[string]Question{"q": BooleanQuestion{Instructions: "answer"}},
				ProviderOptions: map[string]map[string]any{key: nil},
			},
			path: `$["providerOptions"]["` + key + `"]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return nil, errors.New("request must not be sent")
			})}))
			if err != nil {
				t.Fatal(err)
			}

			_, err = client.Evaluate(context.Background(), "provider/model", test.request)
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			if requests != 0 {
				t.Fatalf("validation sent %d requests", requests)
			}
			if validation.Path() != test.path {
				t.Fatalf("Path() length = %d, want %d", len(validation.Path()), len(test.path))
			}

			formatted := map[string]struct {
				got, want string
			}{
				"Error()": {validation.Error(), "gateway validation error"},
				"%v":      {fmt.Sprintf("%v", validation), "gateway validation error"},
				"%s":      {fmt.Sprintf("%s", validation), "gateway validation error"},
				"wrapped": {fmt.Errorf("wrapped: %w", validation).Error(), "wrapped: gateway validation error"},
			}
			for surface, diagnostic := range formatted {
				if diagnostic.got != diagnostic.want {
					t.Fatalf("%s = %q, want %q", surface, diagnostic.got, diagnostic.want)
				}
				if strings.Contains(diagnostic.got, marker) {
					t.Fatalf("%s disclosed caller-controlled key", surface)
				}
				if len(diagnostic.got) > 64 {
					t.Fatalf("%s diagnostic length = %d, want at most 64", surface, len(diagnostic.got))
				}
			}
		})
	}
}
