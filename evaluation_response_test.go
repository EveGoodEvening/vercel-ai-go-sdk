package gateway

import (
	"encoding/json"
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

func TestComposeEvaluationResultIntegralMetadataNumberSpellings(t *testing.T) {
	for _, test := range []struct {
		name                string
		probabilityDecimals string
		inputTokens         string
		wantDecimals        int
		wantTokens          int64
	}{
		{"integer", "2", "2", 2, 2},
		{"decimal", "2.0", "1000.0", 2, 1000},
		{"exponent", "2e0", "1e3", 2, 1000},
		{"upper boundaries", "15e0", "9.223372036854775807e18", 15, math.MaxInt64},
		{"huge positive exponent zero", "0e999999999999999999999999999999999999", "-0.000E+999999999999999999999999999999999999", 0, 0},
		{"huge negative exponent zero", "-0e-999999999999999999999999999999999999", "0.000e-999999999999999999999999999999999999", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(`{"answers":{},"rounding":{"probabilityDecimals":` + test.probabilityDecimals + `},"usage":{"inputTokens":` + test.inputTokens + `}}`)
			result, err := composeEvaluationResult("model", map[string]Question{}, rawEvaluationResponse{statusCode: 200, body: body})
			if err != nil {
				t.Fatalf("integral number rejected: %v", err)
			}
			if *result.Rounding.ProbabilityDecimals != test.wantDecimals || *result.Usage.InputTokens != test.wantTokens {
				t.Fatalf("got decimals/tokens %d/%d, want %d/%d", *result.Rounding.ProbabilityDecimals, *result.Usage.InputTokens, test.wantDecimals, test.wantTokens)
			}
		})
	}

	for _, test := range []struct{ name, field, value, path, reason string }{
		{"rounding fractional", "rounding", `{"probabilityDecimals":2.5}`, `$["rounding"]["probabilityDecimals"]`, "must be an integer"},
		{"rounding negative", "rounding", `{"probabilityDecimals":-1.0}`, `$["rounding"]["probabilityDecimals"]`, "must be between 0 and 15"},
		{"rounding out of range", "rounding", `{"probabilityDecimals":1.6e1}`, `$["rounding"]["probabilityDecimals"]`, "must be between 0 and 15"},
		{"usage fractional", "usage", `{"inputTokens":1e-1}`, `$["usage"]["inputTokens"]`, "must be an integer"},
		{"usage negative", "usage", `{"inputTokens":-1.0}`, `$["usage"]["inputTokens"]`, "must be an integer"},
		{"usage overflow", "usage", `{"inputTokens":9223372036854775808.0}`, `$["usage"]["inputTokens"]`, "must be an integer"},
		{"usage exponent overflow", "usage", `{"inputTokens":1e10000}`, `$["usage"]["inputTokens"]`, "must be an integer"},
		{"usage exponent safety bound", "usage", `{"inputTokens":1e10001}`, `$["usage"]["inputTokens"]`, "must be an integer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(`{"answers":{},"` + test.field + `":` + test.value + `}`)
			_, err := composeEvaluationResult("model", map[string]Question{}, rawEvaluationResponse{statusCode: 200, body: body})
			var validation *ResponseValidationError
			if !errors.As(err, &validation) || validation.Path() != test.path || validation.Reason() != test.reason {
				t.Fatalf("got %v, want path/reason %q/%q", err, test.path, test.reason)
			}
		})
	}
}

func TestComposeEvaluationResultBalancedExponentMetadata(t *testing.T) {
	rounding := "1" + strings.Repeat("0", 10001) + "e-10001"
	usage := "0." + strings.Repeat("0", 10000) + "1e10001"
	body := []byte(`{"answers":{},"rounding":{"probabilityDecimals":` + rounding + `},"usage":{"inputTokens":` + usage + `}}`)

	result, err := composeEvaluationResult("model", map[string]Question{}, rawEvaluationResponse{statusCode: 200, body: body})
	if err != nil {
		t.Fatalf("balanced exponent metadata rejected: %v", err)
	}
	if got := *result.Rounding.ProbabilityDecimals; got != 1 {
		t.Fatalf("probability decimals = %d, want 1", got)
	}
	if got := *result.Usage.InputTokens; got != 1 {
		t.Fatalf("input tokens = %d, want 1", got)
	}
}

func TestExactJSONIntegerBalancedExponentInt64Edges(t *testing.T) {
	zeros := strings.Repeat("0", 10001)
	for _, test := range []struct {
		name     string
		spelling string
		want     int64
	}{
		{"maximum", "9223372036854775807" + zeros + "e-10001", math.MaxInt64},
		{"minimum", "-9223372036854775808" + zeros + "e-10001", math.MinInt64},
	} {
		t.Run(test.name, func(t *testing.T) {
			integer, ok := exactJSONInteger(json.Number(test.spelling))
			if !ok || !integer.IsInt64() || integer.Int64() != test.want {
				t.Fatalf("exactJSONInteger() = %v, %t; want %d, true", integer, ok, test.want)
			}
		})
	}

	for _, spelling := range []string{
		"9223372036854775808" + zeros + "e-10001",
		"-9223372036854775809" + zeros + "e-10001",
		"1" + zeros + "e-10002",
	} {
		if integer, ok := exactJSONInteger(json.Number(spelling)); ok {
			t.Fatalf("exactJSONInteger() = %v, true; want rejection", integer)
		}
	}
}

func TestExactJSONIntegerRejectsMalformedZeroSpellings(t *testing.T) {
	for _, spelling := range []string{
		"",
		"-",
		"+0",
		"00",
		"-00",
		".0",
		"0.",
		"0.e1",
		"0e",
		"0e+",
		"0e-",
		"0e1.0",
		"0x0",
		"zero",
	} {
		t.Run(spelling, func(t *testing.T) {
			if integer, ok := exactJSONInteger(json.Number(spelling)); ok {
				t.Fatalf("exactJSONInteger(%q) = %v, true; want rejection", spelling, integer)
			}
			for _, metadata := range []string{
				`"rounding":{"probabilityDecimals":` + spelling + `}`,
				`"usage":{"inputTokens":` + spelling + `}`,
			} {
				body := []byte(`{"answers":{},` + metadata + `}`)
				if _, err := composeEvaluationResult("model", map[string]Question{}, rawEvaluationResponse{statusCode: 200, body: body}); err == nil {
					t.Fatalf("metadata accepted malformed number %q", spelling)
				}
			}
		})
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

func TestComposeEvaluationResultResourceLimits(t *testing.T) {
	metadataBody := func(value string) []byte {
		return []byte(`{"answers":{},"providerMetadata":{"acme":{"v":` + value + `}}}`)
	}
	deepValue := func(arrays int) string {
		return strings.Repeat("[", arrays) + "null" + strings.Repeat("]", arrays)
	}
	arrayValue := func(members int) string {
		if members == 0 {
			return "[]"
		}
		return "[" + strings.Repeat("null,", members-1) + "null]"
	}
	objectValue := func(members int) string {
		var builder strings.Builder
		builder.WriteByte('{')
		for index := range members {
			if index != 0 {
				builder.WriteByte(',')
			}
			fmt.Fprintf(&builder, `%q:null`, strconv.Itoa(index))
		}
		builder.WriteByte('}')
		return builder.String()
	}
	longKeyValue := func(size int) string {
		return `{"` + strings.Repeat("k", size) + `":null}`
	}
	longStringValue := func(size int) string {
		return `"` + strings.Repeat("v", size) + `"`
	}
	depthPath := `$["providerMetadata"]["acme"]["v"]` + strings.Repeat("[0]", 61)

	tests := []struct {
		name       string
		body       []byte
		wantPath   string
		wantReason string
	}{
		{name: "depth exact", body: metadataBody(deepValue(60))},
		{name: "depth limit plus one", body: metadataBody(deepValue(61)), wantPath: depthPath, wantReason: "maximum depth is 64"},
		{name: "array members exact", body: metadataBody(arrayValue(maxResponseMembers))},
		{name: "array members limit plus one", body: metadataBody(arrayValue(maxResponseMembers + 1)), wantPath: `$["providerMetadata"]["acme"]["v"]`, wantReason: "array exceeds 10000 members"},
		{name: "object members exact", body: metadataBody(objectValue(maxResponseMembers))},
		{name: "object members limit plus one", body: metadataBody(objectValue(maxResponseMembers + 1)), wantPath: `$["providerMetadata"]["acme"]["v"]`, wantReason: "object exceeds 10000 members"},
		{name: "decoded value exact", body: metadataBody(longStringValue(maxResponseValueBytes))},
		{name: "decoded value limit plus one", body: metadataBody(longStringValue(maxResponseValueBytes + 1)), wantPath: `$["providerMetadata"]["acme"]["v"]`, wantReason: "string exceeds 1 MiB"},
		{name: "decoded key exact", body: metadataBody(longKeyValue(maxResponseValueBytes))},
		{name: "decoded key limit plus one", body: metadataBody(longKeyValue(maxResponseValueBytes + 1)), wantPath: `$["providerMetadata"]["acme"]["v"]`, wantReason: "object key exceeds 1 MiB"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := composeEvaluationResult("model", map[string]Question{}, rawEvaluationResponse{statusCode: http.StatusOK, body: test.body})
			if test.wantReason == "" {
				if err != nil {
					t.Fatalf("exact limit rejected: %v", err)
				}
				if result == nil || result.Response.ModelID != "model" {
					t.Fatalf("unexpected result: %#v", result)
				}
				return
			}

			var validation *ResponseValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("got %T %v", err, err)
			}
			if validation.StatusCode() != http.StatusOK || validation.Path() != test.wantPath || validation.Reason() != test.wantReason {
				t.Fatalf("status/path/reason = %d %q %q; want %d %q %q", validation.StatusCode(), validation.Path(), validation.Reason(), http.StatusOK, test.wantPath, test.wantReason)
			}
			if diagnostic := validation.Error(); len(diagnostic) > 512 || strings.Contains(diagnostic, strings.Repeat("k", 64)) || strings.Contains(diagnostic, strings.Repeat("v", 64)) {
				t.Fatalf("diagnostic is not defensively bounded: length=%d", len(diagnostic))
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
