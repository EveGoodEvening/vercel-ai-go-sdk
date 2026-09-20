package gateway

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestComposeEvaluationResultValidMixedResponse(t *testing.T) {
	questions := map[string]Question{
		"boolean": BooleanQuestion{Instructions: "ok?"},
		"choice":  ChoiceQuestion{Instructions: "pick", Criteria: map[string]any{"a": "A", "b": "B"}},
		"score":   ScoreQuestion{Instructions: "score", Criteria: []any{"bad", "good"}},
	}
	headers := http.Header{"X-Test": {"original"}, "Content-Type": {"text/plain"}}
	body := []byte(`{"answers":{"boolean":{"type":"boolean","probability":0.75},"choice":{"type":"choice","choice":"a","probabilities":{"a":0.6,"b":0.4}},"score":{"type":"score","score":0.4,"probabilities":{"0":0.6,"1":0.4}}},"rounding":{"probabilityDecimals":2,"scoreDecimals":3},"usage":{"inputTokens":0,"outputTokens":5},"warnings":[{"type":"unsupported","feature":"tool","details":"detail"},{"type":"compatibility","feature":"schema"},{"type":"deprecated","setting":"old","message":"use new"},{"type":"other","message":"note"}],"providerMetadata":{"acme":{"nested":{"ok":true},"list":[1,null]}}}`)
	result, err := composeEvaluationResult("model", questions, rawEvaluationResponse{statusCode: 200, headers: headers, body: body})
	if err != nil {
		t.Fatalf("compose response: %v", err)
	}
	if result.Response.ModelID != "model" || !reflect.DeepEqual(result.Response.Body, body) {
		t.Fatalf("unexpected response metadata: %#v", result.Response)
	}
	if result.Warnings == nil || len(result.Warnings) != 4 || result.ProviderMetadata["acme"] == nil {
		t.Fatalf("metadata not preserved: %#v", result)
	}
	if result.Rounding == nil || result.Rounding.ProbabilityDecimals == nil || *result.Rounding.ProbabilityDecimals != 2 {
		t.Fatalf("rounding not decoded: %#v", result.Rounding)
	}
	if result.Usage == nil || result.Usage.InputTokens == nil || *result.Usage.InputTokens != 0 {
		t.Fatalf("usage not decoded: %#v", result.Usage)
	}
	headers.Set("X-Test", "mutated")
	body[0] = 'x'
	if result.Response.Headers.Get("X-Test") != "original" || result.Response.Body[0] != '{' {
		t.Fatal("response metadata aliases transport buffers")
	}
}

func TestComposeEvaluationResultPresenceSemantics(t *testing.T) {
	questions := map[string]Question{"choice": ChoiceQuestion{Instructions: "pick", Criteria: map[string]any{"a": "A"}}}
	result, err := composeEvaluationResult("model", questions, rawEvaluationResponse{statusCode: 200, body: []byte(`{"answers":{"choice":{"type":"choice","choice":"a"}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rounding != nil || result.Usage != nil || result.ProviderMetadata != nil || result.Warnings == nil || result.Answers["choice"].(ChoiceAnswer).Probabilities != nil {
		t.Fatalf("absence normalized incorrectly: %#v", result)
	}
	result, err = composeEvaluationResult("model", questions, rawEvaluationResponse{statusCode: 200, body: []byte(`{"answers":{"choice":{"type":"choice","choice":"a"}},"rounding":{},"usage":{},"warnings":[],"providerMetadata":{}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rounding == nil || result.Usage == nil || result.ProviderMetadata == nil || len(result.ProviderMetadata) != 0 {
		t.Fatalf("present empty values lost: %#v", result)
	}
	for _, decimals := range []int{0, 15} {
		body := []byte(`{"answers":{"choice":{"type":"choice","choice":"a"}},"rounding":{"probabilityDecimals":` + strconv.Itoa(decimals) + `,"scoreDecimals":` + strconv.Itoa(decimals) + `}}`)
		bounded, boundedErr := composeEvaluationResult("model", questions, rawEvaluationResponse{statusCode: 200, body: body})
		if boundedErr != nil || *bounded.Rounding.ProbabilityDecimals != decimals || *bounded.Rounding.ScoreDecimals != decimals {
			t.Fatalf("rounding boundary %d: %#v %v", decimals, bounded, boundedErr)
		}
	}
}

func TestComposeEvaluationResultStrictFailures(t *testing.T) {
	questions := map[string]Question{"choice": ChoiceQuestion{Instructions: "pick", Criteria: map[string]any{"a": "A", "b": "B"}}}
	tests := []struct{ name, body, path, reason string }{
		{"malformed", `{`, `$`, `malformed JSON`},
		{"trailing", `{"answers":{}} {}`, `$`, `trailing JSON value`},
		{"duplicate", `{"answers":{},"answers":{}}`, `$["answers"]`, `duplicate field`},
		{"unknown", `{"answers":{},"future":1}`, `$["future"]`, `unknown field`},
		{"missing answer", `{"answers":{}}`, `$["answers"]["choice"]`, `answer is missing`},
		{"extra answer", `{"answers":{"choice":{"type":"choice","choice":"a"},"extra":{"type":"boolean","probability":1}}}`, `$["answers"]["extra"]`, `answer is unexpected`},
		{"wrong type", `{"answers":{"choice":{"type":"boolean","probability":1}}}`, `$["answers"]["choice"]["type"]`, `answer type does not match question`},
		{"null probabilities", `{"answers":{"choice":{"type":"choice","choice":"a","probabilities":null}}}`, `$["answers"]["choice"]["probabilities"]`, `must be non-null`},
		{"missing probability", `{"answers":{"choice":{"type":"choice","choice":"a","probabilities":{"a":1}}}}`, `$["answers"]["choice"]["probabilities"]["b"]`, `probability key is missing`},
		{"bad sum", `{"answers":{"choice":{"type":"choice","choice":"a","probabilities":{"a":0.6,"b":0.3}}}}`, `$["answers"]["choice"]["probabilities"]`, `probabilities must sum to 1 within tolerance`},
		{"not maximal", `{"answers":{"choice":{"type":"choice","choice":"a","probabilities":{"a":0.4,"b":0.6}}}}`, `$["answers"]["choice"]["probabilities"]`, `selected choice is not maximal`},
		{"bad warning", `{"answers":{"choice":{"type":"choice","choice":"a"}},"warnings":[{"type":"other","message":"x","feature":"no"}]}`, `$["warnings"][0]["feature"]`, `field is forbidden for warning type`},
		{"bad rounding", `{"answers":{"choice":{"type":"choice","choice":"a"}},"rounding":{"probabilityDecimals":16}}`, `$["rounding"]["probabilityDecimals"]`, `must be between 0 and 15`},
		{"bad usage", `{"answers":{"choice":{"type":"choice","choice":"a"}},"usage":{"inputTokens":1.5}}`, `$["usage"]["inputTokens"]`, `must be an integer`},
		{"null metadata provider", `{"answers":{"choice":{"type":"choice","choice":"a"}},"providerMetadata":{"acme":null}}`, `$["providerMetadata"]["acme"]`, `must be a non-nil object`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := composeEvaluationResult("model", questions, rawEvaluationResponse{statusCode: 200, body: []byte(test.body)})
			var validation *ResponseValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("got %T %v", err, err)
			}
			if validation.StatusCode() != 200 || validation.Path() != test.path || validation.Reason() != test.reason {
				t.Fatalf("got status/path/reason %d %q %q", validation.StatusCode(), validation.Path(), validation.Reason())
			}
			first := validation.RawResponseBody()
			if len(first) != len(test.body) {
				t.Fatal("raw body not retained")
			}
			if len(first) > 0 {
				first[0] ^= 1
				if reflect.DeepEqual(first, validation.RawResponseBody()) {
					t.Fatal("raw body not defensively copied")
				}
			}
		})
	}
}

func TestComposeEvaluationResultRejectsUnknownFixedDTOFields(t *testing.T) {
	tests := []struct {
		name      string
		questions map[string]Question
		body      string
		path      string
		reason    string
	}{
		{
			name:      "boolean answer",
			questions: map[string]Question{"q": BooleanQuestion{Instructions: "ok?"}},
			body:      `{"answers":{"q":{"type":"boolean","probability":1,"future":true}}}`,
			path:      `$["answers"]["q"]["future"]`,
			reason:    "unknown field",
		},
		{
			name:      "choice answer",
			questions: map[string]Question{"q": ChoiceQuestion{Instructions: "pick", Criteria: map[string]any{"a": "A"}}},
			body:      `{"answers":{"q":{"type":"choice","choice":"a","future":true}}}`,
			path:      `$["answers"]["q"]["future"]`,
			reason:    "unknown field",
		},
		{
			name:      "score answer",
			questions: map[string]Question{"q": ScoreQuestion{Instructions: "score", Criteria: []any{"bad", "good"}}},
			body:      `{"answers":{"q":{"type":"score","score":0.5,"future":true}}}`,
			path:      `$["answers"]["q"]["future"]`,
			reason:    "unknown field",
		},
		{
			name:      "rounding",
			questions: map[string]Question{},
			body:      `{"answers":{},"rounding":{"future":1}}`,
			path:      `$["rounding"]["future"]`,
			reason:    "unknown field",
		},
		{
			name:      "usage",
			questions: map[string]Question{},
			body:      `{"answers":{},"usage":{"future":1}}`,
			path:      `$["usage"]["future"]`,
			reason:    "unknown field",
		},
		{
			name:      "unsupported warning",
			questions: map[string]Question{},
			body:      `{"answers":{},"warnings":[{"type":"unsupported","feature":"tools","future":true}]}`,
			path:      `$["warnings"][0]["future"]`,
			reason:    "field is forbidden for warning type",
		},
		{
			name:      "compatibility warning",
			questions: map[string]Question{},
			body:      `{"answers":{},"warnings":[{"type":"compatibility","feature":"schema","future":true}]}`,
			path:      `$["warnings"][0]["future"]`,
			reason:    "field is forbidden for warning type",
		},
		{
			name:      "deprecated warning",
			questions: map[string]Question{},
			body:      `{"answers":{},"warnings":[{"type":"deprecated","setting":"old","message":"use new","future":true}]}`,
			path:      `$["warnings"][0]["future"]`,
			reason:    "field is forbidden for warning type",
		},
		{
			name:      "other warning",
			questions: map[string]Question{},
			body:      `{"answers":{},"warnings":[{"type":"other","message":"note","future":true}]}`,
			path:      `$["warnings"][0]["future"]`,
			reason:    "field is forbidden for warning type",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := composeEvaluationResult("model", test.questions, rawEvaluationResponse{statusCode: 200, body: []byte(test.body)})
			var validation *ResponseValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("got %T %v", err, err)
			}
			if validation.Path() != test.path || validation.Reason() != test.reason {
				t.Fatalf("got path/reason %q %q", validation.Path(), validation.Reason())
			}
		})
	}
}

func TestComposeEvaluationResultRejectsWarningFieldCombinations(t *testing.T) {
	tests := []struct {
		name, warning, field string
	}{
		{"unsupported setting", `{"type":"unsupported","feature":"tools","setting":"old"}`, "setting"},
		{"unsupported message", `{"type":"unsupported","feature":"tools","message":"note"}`, "message"},
		{"compatibility setting", `{"type":"compatibility","feature":"schema","setting":"old"}`, "setting"},
		{"compatibility message", `{"type":"compatibility","feature":"schema","message":"note"}`, "message"},
		{"deprecated feature", `{"type":"deprecated","setting":"old","message":"use new","feature":"tools"}`, "feature"},
		{"deprecated details", `{"type":"deprecated","setting":"old","message":"use new","details":"note"}`, "details"},
		{"other feature", `{"type":"other","message":"note","feature":"tools"}`, "feature"},
		{"other details", `{"type":"other","message":"note","details":"detail"}`, "details"},
		{"other setting", `{"type":"other","message":"note","setting":"old"}`, "setting"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"answers":{},"warnings":[` + test.warning + `]}`
			_, err := composeEvaluationResult("model", map[string]Question{}, rawEvaluationResponse{statusCode: 200, body: []byte(body)})
			var validation *ResponseValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("got %T %v", err, err)
			}
			wantPath := `$["warnings"][0]["` + test.field + `"]`
			if validation.Path() != wantPath || validation.Reason() != "field is forbidden for warning type" {
				t.Fatalf("got path/reason %q %q", validation.Path(), validation.Reason())
			}
		})
	}
}

func TestComposeEvaluationResultValidationDiagnostics(t *testing.T) {
	const marker = "secret-response-marker"
	tests := []struct {
		name       string
		body       string
		requestID  string
		responseID string
	}{
		{"malformed", `{"requestId":"req-malformed","responseId":"res-malformed","answers":`, "req-malformed", "res-malformed"},
		{"trailing", `{"requestId":"req-trailing","responseId":"res-trailing","answers":{}} {"` + marker + `":true}`, "req-trailing", "res-trailing"},
		{"duplicate", `{"requestId":"req-duplicate","responseId":"res-duplicate","answers":{},"answers":{"` + marker + `":true}}`, "req-duplicate", "res-duplicate"},
		{"nested invalid", `{"requestId":"req-nested","responseId":"res-nested","answers":{"q":{"note":"` + marker + `","type":"choice","type":"choice"}}}`, "req-nested", "res-nested"},
	}
	questions := map[string]Question{"q": ChoiceQuestion{Instructions: "pick", Criteria: map[string]any{"a": "A"}}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := composeEvaluationResult("model", questions, rawEvaluationResponse{statusCode: 200, body: []byte(test.body), bodyTruncated: true})
			var validation *ResponseValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("got %T %v", err, err)
			}
			if validation.RequestID() != test.requestID || validation.ResponseID() != test.responseID {
				t.Fatalf("IDs = %q, %q; want %q, %q", validation.RequestID(), validation.ResponseID(), test.requestID, test.responseID)
			}
			if !validation.BodyTruncated() {
				t.Fatal("truncation flag was lost")
			}
			first := validation.RawResponseBody()
			if !reflect.DeepEqual(first, []byte(test.body)) {
				t.Fatal("bounded raw body was not retained")
			}
			first[0] ^= 1
			if reflect.DeepEqual(first, validation.RawResponseBody()) {
				t.Fatal("raw body accessor returned an aliased slice")
			}
			formatted := []string{
				validation.Error(),
				fmt.Sprint(validation),
				fmt.Sprintf("%s", validation),
				fmt.Sprintf("%v", validation),
				fmt.Sprintf("%+v", validation),
				fmt.Sprintf("%q", validation),
				fmt.Errorf("wrapped: %w", validation).Error(),
			}
			for _, text := range formatted {
				if strings.Contains(text, marker) || strings.Contains(text, test.body) {
					t.Fatalf("format leaked raw response: %q", text)
				}
			}
		})
	}
}

func TestComposeEvaluationResultToleranceBoundaries(t *testing.T) {
	choice := map[string]Question{"q": ChoiceQuestion{Instructions: "pick", Criteria: map[string]any{"a": "A", "b": "B"}}}
	choiceBody := func(choice string, a, b float64, probabilityDecimals string) string {
		body := `{"answers":{"q":{"type":"choice","choice":"` + choice + `","probabilities":{"a":` + strconv.FormatFloat(a, 'g', -1, 64) + `,"b":` + strconv.FormatFloat(b, 'g', -1, 64) + `}}}`
		if probabilityDecimals != "" {
			body += `,"rounding":{"probabilityDecimals":` + probabilityDecimals + `}`
		}
		return body + `}`
	}
	baseLower := 1 - responseBaseTolerance
	baseUpper := 1 + responseBaseTolerance
	baseUpperRemainder := baseUpper - 1
	roundedTolerance := responseBaseTolerance + 2*0.5*math.Pow10(-2)
	roundedLower := 1 - roundedTolerance
	roundedUpper := 1 + roundedTolerance
	roundedUpperRemainder := roundedUpper - 1
	selectedAtMaxBoundary := (1 - responseBaseTolerance) / 2
	otherAtMaxBoundary := selectedAtMaxBoundary + responseBaseTolerance
	choiceCases := []struct {
		name string
		body string
		want string
	}{
		{"base lower IEEE boundary", choiceBody("b", 0, baseLower, ""), "probabilities must sum to 1 within tolerance"},
		{"base lower inside", choiceBody("b", 0, math.Nextafter(baseLower, math.Inf(1)), ""), ""},
		{"base upper exact", choiceBody("a", 1, baseUpperRemainder, ""), ""},
		{"base upper outside", choiceBody("a", 1, math.Nextafter(baseUpper, math.Inf(1))-1, ""), "probabilities must sum to 1 within tolerance"},
		{"rounded lower IEEE boundary", choiceBody("b", 0, roundedLower, "2"), "probabilities must sum to 1 within tolerance"},
		{"rounded lower inside", choiceBody("b", 0, math.Nextafter(roundedLower, math.Inf(1)), "2"), ""},
		{"rounded upper exact", choiceBody("a", 1, roundedUpperRemainder, "2"), ""},
		{"rounded upper outside", choiceBody("a", 1, math.Nextafter(roundedUpper, math.Inf(1))-1, "2"), "probabilities must sum to 1 within tolerance"},
		{"selected maximum exact", choiceBody("a", selectedAtMaxBoundary, otherAtMaxBoundary, ""), ""},
		{"selected maximum outside", choiceBody("a", selectedAtMaxBoundary, math.Nextafter(otherAtMaxBoundary, math.Inf(1)), ""), "selected choice is not maximal"},
	}
	for _, test := range choiceCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := composeEvaluationResult("model", choice, rawEvaluationResponse{statusCode: 200, body: []byte(test.body)})
			if test.want == "" {
				if err != nil {
					t.Fatalf("boundary rejected: %v", err)
				}
				return
			}
			var validation *ResponseValidationError
			if !errors.As(err, &validation) || validation.Reason() != test.want {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}

	score := map[string]Question{"q": ScoreQuestion{Instructions: "score", Criteria: []any{"bad", "good"}}}
	scoreBody := func(score, probabilityOne float64) string {
		return `{"answers":{"q":{"type":"score","score":` + strconv.FormatFloat(score, 'g', -1, 64) + `,"probabilities":{"0":` + strconv.FormatFloat(1-probabilityOne, 'g', -1, 64) + `,"1":` + strconv.FormatFloat(probabilityOne, 'g', -1, 64) + `}}}}`
	}
	meanLower := 0.5 - responseBaseTolerance
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{"weighted mean exact", scoreBody(0.5, meanLower), ""},
		{"weighted mean outside", scoreBody(0.5, math.Nextafter(meanLower, math.Inf(-1))), "score does not match weighted mean"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := composeEvaluationResult("model", score, rawEvaluationResponse{statusCode: 200, body: []byte(test.body)})
			if test.want == "" {
				if err != nil {
					t.Fatalf("boundary rejected: %v", err)
				}
				return
			}
			var validation *ResponseValidationError
			if !errors.As(err, &validation) || validation.Reason() != test.want {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}
