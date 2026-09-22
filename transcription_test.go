package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

type unsupportedTranscriptionAudio struct{}

func (unsupportedTranscriptionAudio) transcriptionAudio() {}

func transcriptionClient(t *testing.T, fn func(*http.Request) *http.Response) *Client {
	t.Helper()
	c, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return fn(r), nil })}))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTranscribeExactWireAndVariants(t *testing.T) {
	for _, audio := range []TranscriptionAudio{TranscriptionBase64{"audio/x", "not base64 !"}, &TranscriptionBase64{"audio/x", "not base64 !"}, TranscriptionBytes{"audio/x", []byte{0, 1, 2}}, &TranscriptionBytes{"audio/x", []byte{0, 1, 2}}} {
		var got []byte
		c := transcriptionClient(t, func(r *http.Request) *http.Response {
			if r.URL.Path != "/v4/ai/transcription-model" || r.Header.Get(headerTranscriptionModelSpecificationVersion) != "4" || r.Header.Get(headerModelID) != "p/m" {
				t.Fatalf("request=%s headers=%v", r.URL.Path, r.Header)
			}
			got, _ = io.ReadAll(r.Body)
			return &http.Response{StatusCode: 200, Header: http.Header{"X-Test": {"yes"}}, Body: io.NopCloser(strings.NewReader(`{"text":"hello","segments":[{"text":"x","startSecond":-1.5,"endSecond":-2}],"language":"","durationInSeconds":-3.5,"warnings":[],"providerMetadata":{"p":{"x":1}}}`)), Request: r}
		})
		r, err := c.Transcribe(context.Background(), "p/m", TranscriptionRequest{Audio: audio})
		if err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				t.Fatalf("%s: %s", ve.Path(), ve.Reason())
			}
			t.Fatal(err)
		}
		wantData := "not base64 !"
		if _, ok := audio.(TranscriptionBytes); ok {
			wantData = base64.StdEncoding.EncodeToString([]byte{0, 1, 2})
		}
		if _, ok := audio.(*TranscriptionBytes); ok {
			wantData = base64.StdEncoding.EncodeToString([]byte{0, 1, 2})
		}
		want := `{"audio":"` + wantData + `","mediaType":"audio/x"}`
		if string(got) != want || strings.Contains(string(got), `"model"`) || r.Text != "hello" || len(r.Segments) != 1 || r.Segments[0].StartSeconds != -1.5 || r.Language == nil || *r.Language != "" || r.DurationSeconds == nil || *r.DurationSeconds != -3.5 {
			t.Fatalf("body=%s result=%#v", got, r)
		}
	}
}

func TestTranscriptionValidationAndBounds(t *testing.T) {
	bad := string([]byte{0xff})
	var nilB64 *TranscriptionBase64
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name         string
		audio        TranscriptionAudio
		path, reason string
	}{
		{"nil", nil, `$["audio"]`, "must be TranscriptionBase64 or TranscriptionBytes"}, {"typed nil", nilB64, `$["audio"]`, "must be TranscriptionBase64 or TranscriptionBytes"}, {"unsupported", unsupportedTranscriptionAudio{}, `$["audio"]`, "must be TranscriptionBase64 or TranscriptionBytes"},
		{"media utf8", TranscriptionBase64{bad, ""}, `$["audio"]["mediaType"]`, "must be valid UTF-8"}, {"media limit", TranscriptionBase64{strings.Repeat("m", 256), ""}, `$["audio"]["mediaType"]`, "must not exceed 255 bytes"},
		{"data utf8", TranscriptionBase64{"", bad}, `$["audio"]["data"]`, "must be valid UTF-8"}, {"string limit", TranscriptionBase64{"", strings.Repeat("x", maxStringBytes+1)}, `$["audio"]["data"]`, "must not exceed 1048576 bytes"}, {"bytes limit", TranscriptionBytes{"", make([]byte, maxTranscriptionDecodedInputBytes+1)}, `$["audio"]["data"]`, "must not exceed 8388608 bytes"},
	}
	c := transcriptionClient(t, func(*http.Request) *http.Response { t.Fatal("network reached"); return nil })
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e := c.Transcribe(canceled, "p/m", TranscriptionRequest{Audio: tc.audio})
			var ve *ValidationError
			if !errors.As(e, &ve) || ve.Path() != tc.path || ve.Reason() != tc.reason {
				t.Fatalf("%T %v", e, e)
			}
		})
	}
	_, e := c.Transcribe(canceled, "p/m", TranscriptionRequest{Audio: TranscriptionBase64{}})
	var te *TransportError
	if !errors.As(e, &te) || !errors.Is(e, context.Canceled) {
		t.Fatalf("%T %v", e, e)
	}
	for _, audio := range []TranscriptionAudio{TranscriptionBase64{"", strings.Repeat("x", maxStringBytes)}, TranscriptionBytes{"", make([]byte, maxTranscriptionDecodedInputBytes)}} {
		if _, e := prepareTranscriptionRequest("p/m", TranscriptionRequest{Audio: audio}); e != nil {
			t.Fatal(e)
		}
	}
	if n, ok := checkedTranscriptionRequestSize(2, maxRequestBodyBytes-len(`{"audio":`)-len(`,"mediaType":`)-2-1, 0); !ok || n != maxRequestBodyBytes {
		t.Fatalf("n=%d ok=%v", n, ok)
	}
	if _, ok := checkedTranscriptionRequestSize(2, maxRequestBodyBytes, 0); ok {
		t.Fatal("oversize accepted")
	}
}

func TestTranscriptionStrictResponse(t *testing.T) {
	cases := []struct{ body, path string }{
		{`{}`, `$["text"]`}, {`{"text":null}`, `$["text"]`}, {`{"text":"x","segments":null}`, `$["segments"]`}, {`{"text":"x","segments":[{}]}`, `$["segments"][0]["text"]`}, {`{"text":"x","segments":[{"text":"x","startSecond":null,"endSecond":1}]}`, `$["segments"][0]["startSecond"]`}, {`{"text":"x","segments":[{"text":"x","startSecond":1,"endSecond":1,"future":1}]}`, `$["segments"][0]["future"]`}, {`{"text":"x","warnings":null}`, `$["warnings"]`}, {`{"text":"x","providerMetadata":null}`, `$["providerMetadata"]`}, {`{"text":"x","text":"y"}`, `$["text"]`}, {`{"text":"x","future":1}`, `$["future"]`}, {`{"text":"x"} {}`, `$`},
	}
	for _, tc := range cases {
		_, e := decodeTranscriptionResult(context.Background(), "p/m", rawProviderResponse{statusCode: 200, body: []byte(tc.body)})
		var ve *ResponseValidationError
		if !errors.As(e, &ve) || ve.Path() != tc.path {
			t.Fatalf("body=%s err=%T %v", tc.body, e, e)
		}
	}
	r, e := decodeTranscriptionResult(context.Background(), "p/m", rawProviderResponse{statusCode: 200, body: []byte(`{"text":"x"}`)})
	if e != nil || r.Segments == nil || r.Warnings == nil || r.Language != nil || r.DurationSeconds != nil {
		t.Fatalf("%#v %v", r, e)
	}
	tooMany := make([]string, maxCollectionItems+1)
	for i := range tooMany {
		tooMany[i] = `{"text":"","startSecond":0,"endSecond":0}`
	}
	_, e = decodeTranscriptionResult(context.Background(), "p/m", rawProviderResponse{statusCode: 200, body: []byte(`{"text":"x","segments":[` + strings.Join(tooMany, ",") + `]}`)})
	var ve *ResponseValidationError
	if !errors.As(e, &ve) || ve.Path() != `$["segments"]` {
		t.Fatalf("%v", e)
	}
	for _, raw := range []json.RawMessage{[]byte(`1e999`), []byte(`-1e999`)} {
		if _, f := transcriptionRequiredFloat(context.Background(), raw, "$"); f == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

type transcriptionScalarCancelContext struct {
	context.Context
	target   string
	canceled atomic.Bool
}

func (c *transcriptionScalarCancelContext) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, c.target) {
			c.canceled.Store(true)
			return context.Canceled
		}
		if !more {
			return nil
		}
	}
}

func TestTranscriptionScalarDecodeCancellation(t *testing.T) {
	cases := []struct {
		name, target, body string
	}{
		{"text", ".transcriptionRequiredString", `{"text":1}`},
		{"language", ".transcriptionNullableString", `{"text":"x","language":1}`},
		{"duration", ".transcriptionNullableFloat", `{"text":"x","durationInSeconds":"bad"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &transcriptionScalarCancelContext{Context: context.Background(), target: tc.target}
			_, err := decodeTranscriptionResult(ctx, "p/m", rawProviderResponse{statusCode: 200, body: []byte(tc.body)})
			var transportErr *TransportError
			var validationErr *ResponseValidationError
			if !errors.As(err, &transportErr) || transportErr.operation != "read response body" || !errors.Is(err, context.Canceled) || errors.As(err, &validationErr) {
				t.Fatalf("%T %v", err, err)
			}
		})
	}
}

func TestTranscriptionCopiesCancellationAndOneAttempt(t *testing.T) {
	body := []byte(`{"text":"x","segments":[{"text":"s","startSecond":1,"endSecond":2}],"language":"en","durationInSeconds":2,"warnings":[],"providerMetadata":{"p":{"x":1}}}`)
	headers := http.Header{"X": {"y"}}
	r, e := decodeTranscriptionResult(context.Background(), "p/m", rawProviderResponse{statusCode: 200, headers: headers, body: body})
	if e != nil {
		t.Fatal(e)
	}
	body[9] = 'z'
	headers.Set("X", "z")
	r.Response.Body[0] = 'z'
	*r.Language = "fr"
	if r.Text != "x" || r.Segments[0].Text != "s" || r.Response.Headers.Get("X") != "y" {
		t.Fatalf("alias %#v", r)
	}
	ctx := &cancelAfterChecksContext{Context: context.Background(), cancelAt: 8}
	large := []byte(`{"text":"` + strings.Repeat("a", maxStringBytes) + `","segments":[]}`)
	_, e = decodeTranscriptionResult(ctx, "p/m", rawProviderResponse{statusCode: 200, body: large})
	var te *TransportError
	if !errors.As(e, &te) || !errors.Is(e, context.Canceled) {
		t.Fatalf("%T %v", e, e)
	}
	var calls atomic.Int32
	c := transcriptionClient(t, func(r *http.Request) *http.Response {
		calls.Add(1)
		return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"limited"}}`)), Request: r}
	})
	c.config.retryPolicy = RetryPolicy{MaxAttempts: 4}
	_, e = c.Transcribe(context.Background(), "p/m", TranscriptionRequest{Audio: TranscriptionBase64{}})
	var re *ResponseError
	if !errors.As(e, &re) || calls.Load() != 1 {
		var ve *ValidationError
		if errors.As(e, &ve) {
			t.Fatalf("%s: %s", ve.Path(), ve.Reason())
		}
		t.Fatalf("%v calls=%d", e, calls.Load())
	}
	if !reflect.DeepEqual(r.Warnings, []ProviderWarning{}) {
		t.Fatal("warnings ownership")
	}
}

func TestTranscription(t *testing.T) {
	t.Run("wire and variants", TestTranscribeExactWireAndVariants)
	t.Run("validation", TestTranscriptionValidationAndBounds)
	t.Run("strict response", TestTranscriptionStrictResponse)
	t.Run("scalar decode cancellation", TestTranscriptionScalarDecodeCancellation)
	t.Run("copies cancellation one attempt", TestTranscriptionCopiesCancellationAndOneAttempt)
}
