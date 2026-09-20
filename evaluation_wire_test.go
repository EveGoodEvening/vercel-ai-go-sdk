package gateway

import (
	"encoding/json"
	"testing"
)

type wireMarshaledString string

func (wireMarshaledString) MarshalJSON() ([]byte, error) { return []byte("true"), nil }

type wirePointerMarshaledString string

func (*wirePointerMarshaledString) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

type wireNamedInt int

func assertWireJSON(t *testing.T, request EvaluationRequest, expected string) {
	t.Helper()
	body, err := encodeEvaluationRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != expected {
		t.Fatalf("got %s, want %s", body, expected)
	}
}

func TestEncodeEvaluationRequestQuestionVariantsAndProviderOptions(t *testing.T) {
	assertWireJSON(t, EvaluationRequest{State: []any{"s"}, Questions: map[string]Question{
		"b": BooleanQuestion{Instructions: "bool"},
		"c": ChoiceQuestion{Instructions: map[string]any{"do": "choose"}, Criteria: map[string]any{"a": "A"}},
		"s": ScoreQuestion{Instructions: []any{"score"}, Criteria: []any{"low", "high"}},
	}}, `{"state":["s"],"questions":{"b":{"type":"boolean","instructions":"bool"},"c":{"type":"choice","instructions":{"do":"choose"},"criteria":{"a":"A"}},"s":{"type":"score","instructions":["score"],"criteria":["low","high"]}}}`)
	assertWireJSON(t, EvaluationRequest{State: "s", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x"}}, ProviderOptions: map[string]map[string]any{}}, `{"state":"s","questions":{"q":{"type":"boolean","instructions":"x"}},"providerOptions":{}}`)
}

func TestEncodeEvaluationRequestNormalizesValidatedValues(t *testing.T) {
	pointerValue := wirePointerMarshaledString("pointer text")
	request := EvaluationRequest{
		State: map[string]any{
			"bytes":   []byte{1, 2, 255},
			"number":  json.Number("1e3"),
			"named":   wireNamedInt(7),
			"marshal": wireMarshaledString("state text"),
		},
		Questions: map[string]Question{
			"q": BooleanQuestion{
				Instructions: &pointerValue,
				Criteria:     &BooleanCriteria{True: OptionalJSON{Set: true, Value: wireMarshaledString("criterion text")}},
			},
		},
		ProviderOptions: map[string]map[string]any{
			"acme": {"special": wireMarshaledString("provider text")},
		},
	}
	assertWireJSON(t, request, `{"state":{"bytes":[1,2,255],"marshal":"state text","named":7,"number":"1e3"},"questions":{"q":{"type":"boolean","instructions":"pointer text","criteria":{"true":"criterion text"}}},"providerOptions":{"acme":{"special":"provider text"}}}`)
}

func TestEncodeEvaluationRequestPreservesFloatWidths(t *testing.T) {
	request := EvaluationRequest{
		State: map[string]any{
			"float32": float32(0.1),
			"float64": float64(0.10000000149011612),
		},
		Questions: map[string]Question{
			"q": BooleanQuestion{Instructions: "x"},
		},
	}
	assertWireJSON(t, request, `{"state":{"float32":0.1,"float64":0.10000000149011612},"questions":{"q":{"type":"boolean","instructions":"x"}}}`)
}

func TestEncodeBooleanCriteriaPresence(t *testing.T) {
	cases := []struct {
		name     string
		criteria *BooleanCriteria
		expected string
	}{
		{"omitted", nil, `{"type":"boolean","instructions":"x"}`},
		{"empty", &BooleanCriteria{}, `{"type":"boolean","instructions":"x","criteria":{}}`},
		{"true null", &BooleanCriteria{True: OptionalJSON{Set: true}}, `{"type":"boolean","instructions":"x","criteria":{"true":null}}`},
		{"false null", &BooleanCriteria{False: OptionalJSON{Set: true}}, `{"type":"boolean","instructions":"x","criteria":{"false":null}}`},
		{"both null", &BooleanCriteria{True: OptionalJSON{Set: true}, False: OptionalJSON{Set: true}}, `{"type":"boolean","instructions":"x","criteria":{"false":null,"true":null}}`},
		{"both values", &BooleanCriteria{True: OptionalJSON{Set: true, Value: "yes"}, False: OptionalJSON{Set: true, Value: []any{"no"}}}, `{"type":"boolean","instructions":"x","criteria":{"false":["no"],"true":"yes"}}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := EvaluationRequest{State: "s", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "x", Criteria: test.criteria}}}
			assertWireJSON(t, request, `{"state":"s","questions":{"q":`+test.expected+`}}`)
		})
	}
}
