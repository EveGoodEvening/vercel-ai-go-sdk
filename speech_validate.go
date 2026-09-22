package gateway

import (
	"encoding/json"
	"math"
	"unicode/utf8"
)

type speechRequestWire struct {
	Text            string         `json:"text"`
	Voice           string         `json:"voice,omitempty"`
	Instructions    string         `json:"instructions,omitempty"`
	Language        string         `json:"language,omitempty"`
	OutputFormat    string         `json:"outputFormat,omitempty"`
	Speed           *float64       `json:"speed,omitempty"`
	ProviderOptions map[string]any `json:"providerOptions,omitempty"`
}

func cloneSpeechRequest(r SpeechRequest) SpeechRequest {
	if r.Speed != nil {
		v := *r.Speed
		r.Speed = &v
	}
	r.ProviderOptions = append([]ProviderOption(nil), r.ProviderOptions...)
	return r
}

func preflightSpeechRequest(modelID string, r SpeechRequest) (int, error) {
	if modelID == "" || !validModelID(modelID) {
		return 0, validationError(memberPath("$", "modelID"), "must be a nonempty provider/model string")
	}
	fields := []struct {
		name  string
		value string
		limit int
		emit  bool
	}{
		{"text", r.Text, maxStringBytes, true},
		{"voice", r.Voice, 255, r.Voice != ""},
		{"instructions", r.Instructions, maxStringBytes, r.Instructions != ""},
		{"language", r.Language, 255, r.Language != ""},
		{"outputFormat", r.OutputFormat, 255, r.OutputFormat != ""},
	}
	for _, f := range fields {
		path := memberPath("$", f.name)
		if !utf8.ValidString(f.value) {
			return 0, validationError(path, "must be valid UTF-8")
		}
		if f.emit && len(f.value) > f.limit {
			return 0, validationError(path, speechStringLimitReason(f.limit))
		}
	}
	if r.Speed != nil && (math.IsNaN(*r.Speed) || math.IsInf(*r.Speed, 0)) {
		return 0, validationError(memberPath("$", "speed"), "must be a finite number")
	}
	options, err := encodeProviderOptions(r.ProviderOptions)
	if err != nil {
		return 0, err
	}
	wire := speechRequestWire{Text: r.Text, Voice: r.Voice, Instructions: r.Instructions, Language: r.Language, OutputFormat: r.OutputFormat, Speed: r.Speed, ProviderOptions: options}
	payload, marshalErr := json.Marshal(wire)
	if marshalErr != nil {
		return 0, &TransportError{operation: "encode request", cause: marshalErr}
	}
	if len(payload) > maxRequestBodyBytes {
		return 0, validationError("$", "encoded request exceeds 16777216 bytes")
	}
	return len(payload), nil
}

func speechStringLimitReason(limit int) string {
	if limit == 255 {
		return "must not exceed 255 bytes"
	}
	return "must not exceed 1048576 bytes"
}

func prepareSpeechRequest(modelID string, r SpeechRequest) ([]byte, error) {
	if _, err := preflightSpeechRequest(modelID, r); err != nil {
		return nil, err
	}
	options, err := encodeProviderOptions(r.ProviderOptions)
	if err != nil {
		return nil, err
	}
	payload, marshalErr := json.Marshal(speechRequestWire{Text: r.Text, Voice: r.Voice, Instructions: r.Instructions, Language: r.Language, OutputFormat: r.OutputFormat, Speed: r.Speed, ProviderOptions: options})
	if marshalErr != nil {
		return nil, &TransportError{operation: "encode request", cause: marshalErr}
	}
	return payload, nil
}
