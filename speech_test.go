package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func speechClient(t *testing.T, fn func(*http.Request) *http.Response) *Client {
	t.Helper()
	c, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return fn(r), nil })}))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGenerateSpeech(t *testing.T) {
	speed := math.Copysign(0, -1)
	var got []byte
	c := speechClient(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/v4/ai/speech-model" || r.Header.Get(headerSpeechModelSpecificationVersion) != "4" || r.Header.Get(headerModelID) != "p/m" {
			t.Fatalf("request=%s headers=%v", r.URL.Path, r.Header)
		}
		got, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Test": {"yes"}}, Body: io.NopCloser(strings.NewReader(`{"audio":"base64-audio","warnings":[{"type":"unsupported","feature":"voice","details":"d"}],"providerMetadata":{"p":{"x":1}}}`)), Request: r}
	})
	r, err := c.GenerateSpeech(context.Background(), "p/m", SpeechRequest{Text: "", Voice: " ", Instructions: "say", Language: "en", OutputFormat: "mp3", Speed: &speed})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"text":"","voice":" ","instructions":"say","language":"en","outputFormat":"mp3","speed":-0}` {
		t.Fatalf("body=%s", got)
	}
	if r.Audio != "base64-audio" || len(r.Warnings) != 1 || r.Warnings[0].Details == nil || *r.Warnings[0].Details != "d" || r.Response.ModelID != "p/m" || r.Response.Headers.Get("X-Test") != "yes" {
		t.Fatalf("result=%#v", r)
	}
}

func TestSpeechOptionalPresenceAndSpeed(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  SpeechRequest
		want string
	}{
		{"omitted", SpeechRequest{Text: "x"}, `{"text":"x"}`},
		{"whitespace", SpeechRequest{Text: "x", Voice: " ", Instructions: " ", Language: " ", OutputFormat: " "}, `{"text":"x","voice":" ","instructions":" ","language":" ","outputFormat":" "}`},
		{"zero", func() SpeechRequest { v := 0.0; return SpeechRequest{Text: "x", Speed: &v} }(), `{"text":"x","speed":0}`},
		{"negative", func() SpeechRequest { v := -2.5; return SpeechRequest{Text: "x", Speed: &v} }(), `{"text":"x","speed":-2.5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, e := prepareSpeechRequest("p/m", tc.req)
			if e != nil || string(b) != tc.want {
				t.Fatalf("%s %v", b, e)
			}
		})
	}
}

func TestSpeechValidationOrderAndBounds(t *testing.T) {
	badUTF8 := string([]byte{0xff})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name, model  string
		req          SpeechRequest
		path, reason string
	}{
		{"model", "bad", SpeechRequest{}, `$["modelID"]`, "must be a nonempty provider/model string"}, {"text utf8", "p/m", SpeechRequest{Text: badUTF8}, `$["text"]`, "must be valid UTF-8"},
		{"text limit", "p/m", SpeechRequest{Text: strings.Repeat("x", maxStringBytes+1)}, `$["text"]`, "must not exceed 1048576 bytes"}, {"voice utf8", "p/m", SpeechRequest{Voice: badUTF8}, `$["voice"]`, "must be valid UTF-8"},
		{"voice limit", "p/m", SpeechRequest{Voice: strings.Repeat("x", 256)}, `$["voice"]`, "must not exceed 255 bytes"}, {"instructions utf8", "p/m", SpeechRequest{Instructions: badUTF8}, `$["instructions"]`, "must be valid UTF-8"},
		{"instructions limit", "p/m", SpeechRequest{Instructions: strings.Repeat("x", maxStringBytes+1)}, `$["instructions"]`, "must not exceed 1048576 bytes"}, {"language utf8", "p/m", SpeechRequest{Language: badUTF8}, `$["language"]`, "must be valid UTF-8"},
		{"language limit", "p/m", SpeechRequest{Language: strings.Repeat("x", 256)}, `$["language"]`, "must not exceed 255 bytes"}, {"format utf8", "p/m", SpeechRequest{OutputFormat: badUTF8}, `$["outputFormat"]`, "must be valid UTF-8"},
		{"format limit", "p/m", SpeechRequest{OutputFormat: strings.Repeat("x", 256)}, `$["outputFormat"]`, "must not exceed 255 bytes"}, {"nan", "p/m", func() SpeechRequest { v := math.NaN(); return SpeechRequest{Speed: &v} }(), `$["speed"]`, "must be a finite number"},
		{"inf", "p/m", func() SpeechRequest { v := math.Inf(1); return SpeechRequest{Speed: &v} }(), `$["speed"]`, "must be a finite number"}, {"negative inf", "p/m", func() SpeechRequest { v := math.Inf(-1); return SpeechRequest{Speed: &v} }(), `$["speed"]`, "must be a finite number"},
		{"nil provider option", "p/m", SpeechRequest{ProviderOptions: []ProviderOption{nil}}, `$["providerOptions"][0]`, "must be a non-nil provider option"},
	}
	c := speechClient(t, func(r *http.Request) *http.Response { t.Fatal("network reached"); return nil })
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e := c.GenerateSpeech(canceled, tc.model, tc.req)
			var ve *ValidationError
			if !errors.As(e, &ve) || ve.Path() != tc.path || ve.Reason() != tc.reason {
				t.Fatalf("err=%T %v", e, e)
			}
		})
	}
	_, e := c.GenerateSpeech(canceled, "p/m", SpeechRequest{})
	var te *TransportError
	if !errors.As(e, &te) || !errors.Is(e, context.Canceled) {
		t.Fatalf("err=%T %v", e, e)
	}
}

func TestSpeechStrictResponseAndOpaqueAudio(t *testing.T) {
	cases := []struct{ body, path, reason string }{
		{`{}`, `$["audio"]`, "required string"}, {`{"audio":null}`, `$["audio"]`, "required string"}, {`{"audio":1}`, `$["audio"]`, "required string"},
		{`{"audio":"x","audio":"y"}`, `$["audio"]`, "duplicate field"}, {`{"audio":"x","future":1}`, `$["future"]`, "unknown field"}, {`{"audio":"x","warnings":null}`, `$["warnings"]`, "must be an array"},
		{`{"audio":"x","providerMetadata":null}`, `$["providerMetadata"]`, "must be an object"}, {`{"audio":"x"} {}`, `$`, "trailing JSON value"},
		{`{"audio":"x","providerMetadata":{"p":{"audio":"` + strings.Repeat("x", maxStringBytes+1) + `"}}}`, `$["providerMetadata"]["p"]["audio"]`, "must not exceed 1048576 bytes"},
		{`{"audio":"x","providerMetadata":{"p":{"` + strings.Repeat("k", maxStringBytes+1) + `":1}}}`, `$["providerMetadata"]["p"]`, "object key exceeds 1 MiB"},
	}
	for _, tc := range cases {
		r, e := decodeSpeechResult("p/m", rawProviderResponse{statusCode: 200, headers: make(http.Header), body: []byte(tc.body)})
		var ve *ResponseValidationError
		if r != nil || !errors.As(e, &ve) || ve.Path() != tc.path || ve.Reason() != tc.reason {
			t.Fatalf("body prefix %.30q result=%v err=%T %v", tc.body, r, e, e)
		}
	}
	r, e := decodeSpeechResult("p/m", rawProviderResponse{statusCode: 200, headers: make(http.Header), body: []byte(`{"audio":"not base64 !"}`)})
	if e != nil || r.Audio != "not base64 !" {
		t.Fatalf("%#v %v", r, e)
	}
}

func TestSpeechAudioLimitMetadataAndCopies(t *testing.T) {
	for _, n := range []int{maxStringBytes + 1, maxSpeechAudioBytes} {
		body := []byte(`{"audio":"` + strings.Repeat("a", n) + `"}`)
		r, e := decodeSpeechResult("p/m", rawProviderResponse{statusCode: 200, headers: http.Header{"X": {"y"}}, body: body})
		if e != nil || len(r.Audio) != n || len(r.Response.Body) != maxDiagnosticBodyBytes {
			t.Fatalf("n=%d result=%v err=%v", n, r, e)
		}
		body[2] = 'X'
		r.Response.Body[0] = 'X'
		r.Response.Headers.Set("X", "z")
		if r.Audio[0] != 'a' {
			t.Fatal("audio aliased body")
		}
	}
	body := []byte(`{"audio":"` + strings.Repeat("a", maxSpeechAudioBytes+1) + `"}`)
	_, e := decodeSpeechResult("p/m", rawProviderResponse{statusCode: 200, body: body})
	var ve *ResponseValidationError
	if !errors.As(e, &ve) || ve.Path() != `$["audio"]` {
		t.Fatalf("err=%v", e)
	}
	r, e := decodeSpeechResult("p/m", rawProviderResponse{statusCode: 200, body: []byte(`{"audio":"x","warnings":[],"providerMetadata":{}}`)})
	if e != nil || r.Warnings == nil || r.ProviderMetadata == nil || !reflect.DeepEqual(r.Warnings, []ProviderWarning{}) {
		t.Fatalf("%#v %v", r, e)
	}
}

func TestGenerateSpeechOneAttemptErrorsAndClose(t *testing.T) {
	var calls atomic.Int32
	c := speechClient(t, func(r *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"limited"}}`)), Request: r}
	})
	c.config.retryPolicy = RetryPolicy{MaxAttempts: 4}
	_, e := c.GenerateSpeech(context.Background(), "p/m", SpeechRequest{})
	var re *ResponseError
	if !errors.As(e, &re) || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", e, calls.Load())
	}
	_, e = decodeSpeechResult("p/m", rawProviderResponse{statusCode: 200, bodyTruncated: true, body: bytes.Repeat([]byte{'x'}, maxDiagnosticBodyBytes), bodyErr: errors.New("large")})
	var ve *ResponseValidationError
	if !errors.As(e, &ve) || !ve.BodyTruncated() || len(ve.RawResponseBody()) != maxDiagnosticBodyBytes {
		t.Fatalf("err=%v", e)
	}
}

func TestSpeech(t *testing.T) {
	t.Run("optional presence and speed", TestSpeechOptionalPresenceAndSpeed)
	t.Run("validation order and bounds", TestSpeechValidationOrderAndBounds)
	t.Run("strict response and opaque audio", TestSpeechStrictResponseAndOpaqueAudio)
	t.Run("audio limit metadata and copies", TestSpeechAudioLimitMetadataAndCopies)
	t.Run("one attempt errors and close", TestGenerateSpeechOneAttemptErrorsAndClose)
}
