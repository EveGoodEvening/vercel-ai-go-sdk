package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func embeddingClient(t *testing.T, body string, status int, calls *atomic.Int32) *Client {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v4/ai/embedding-model" {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	client, err := NewClient(WithAPIKey("key"), WithBaseURL(s.URL+"/v4/ai"))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestEmbeddingRequestValidationBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	client := embeddingClient(t, `{}`, 200, &calls)
	cases := []struct {
		name, model string
		ctx         context.Context
		req         EmbeddingRequest
		path        string
	}{
		{"nil context", "p/m", nil, EmbeddingRequest{Values: []string{"x"}}, `$["context"]`},
		{"model", "bad", context.Background(), EmbeddingRequest{Values: []string{"x"}}, `$["modelID"]`},
		{"nil values", "p/m", context.Background(), EmbeddingRequest{}, `$["values"]`},
		{"empty values", "p/m", context.Background(), EmbeddingRequest{Values: []string{}}, `$["values"]`},
		{"too many", "p/m", context.Background(), EmbeddingRequest{Values: make([]string, 4097)}, `$["values"]`},
		{"value bytes", "p/m", context.Background(), EmbeddingRequest{Values: []string{strings.Repeat("x", maxStringBytes+1)}}, `$["values"][0]`},
		{"nil option", "p/m", context.Background(), EmbeddingRequest{Values: []string{"x"}, ProviderOptions: []ProviderOption{nil}}, `$["providerOptions"][0]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Embed(tc.ctx, tc.model, tc.req)
			var v *ValidationError
			if !errors.As(err, &v) || v.Path() != tc.path {
				t.Fatalf("err=%T %v", err, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("network calls=%d", calls.Load())
	}
}

func TestEmbedExactWireAndResult(t *testing.T) {
	var calls atomic.Int32
	var gotBody []byte
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotBody, _ = ioReadAll(r)
		w.Header().Set("X-Test", "yes")
		_, _ = w.Write([]byte(`{"embeddings":[[1,2],[-3]],"usage":{"tokens":-4},"warnings":[{"type":"unsupported","feature":"f","details":"d"},{"type":"deprecated","setting":"s","message":"m"},{"type":"other","message":"o"}],"providerMetadata":{"acme":{"x":1}}}`))
	}))
	defer s.Close()
	client, err := NewClient(WithAPIKey("key"), WithBaseURL(s.URL))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Embed(context.Background(), "p/m", EmbeddingRequest{Values: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != `{"values":["a","b"]}` || calls.Load() != 1 {
		t.Fatalf("body=%s calls=%d", gotBody, calls.Load())
	}
	if !reflect.DeepEqual(result.Embeddings, [][]float64{{1, 2}, {-3}}) || result.Usage == nil || result.Usage.Tokens == nil || *result.Usage.Tokens != -4 || len(result.Warnings) != 3 || result.Response.Headers.Get("X-Test") != "yes" {
		t.Fatalf("result=%#v", result)
	}
	result.Embeddings[0][0] = 99
	result.ProviderMetadata["acme"][0] = 'x'
	if bytes.Contains(gotBody, []byte("99")) {
		t.Fatal("request/result storage shared")
	}
}

func ioReadAll(r *http.Request) ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(r.Body)
	return b.Bytes(), err
}

func TestEmbeddingUsageExactInt64AndNullable(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		nilUsage bool
		want     int64
	}{{``, true, 0}, {`null`, true, 0}, {`{"tokens":-9223372036854775808}`, false, math.MinInt64}, {`{"tokens":9223372036854775807}`, false, math.MaxInt64}, {`{"tokens":0}`, false, 0}, {`{"tokens":1e3}`, false, 1000}, {`{"tokens":1.0}`, false, 1}} {
		top := `{"embeddings":[[1]]}`
		if tc.raw != "" {
			top = `{"embeddings":[[1]],"usage":` + tc.raw + `}`
		}
		r, e := decodeEmbeddingResult("p/m", 1, rawProviderResponse{statusCode: 200, body: []byte(top)})
		if e != nil {
			t.Fatal(e)
		}
		if (r.Usage == nil) != tc.nilUsage {
			t.Fatalf("usage=%#v", r.Usage)
		}
		if !tc.nilUsage && *r.Usage.Tokens != tc.want {
			t.Fatalf("tokens=%d", *r.Usage.Tokens)
		}
	}
}

func TestEmbeddingStrictResponseFailures(t *testing.T) {
	cases := []string{`{}`, `{"embeddings":null}`, `{"embeddings":[]}`, `{"embeddings":[[]]}`, `{"embeddings":[[1],[2]]}`, `{"embeddings":[[1]],"future":1}`, `{"embeddings":[[1]],"embeddings":[[2]]}`, `{"embeddings":[[1]]} {}`, `{"embeddings":[[1]],"usage":{}}`, `{"embeddings":[[1]],"usage":{"tokens":null}}`, `{"embeddings":[[1]],"usage":{"tokens":1.5}}`, `{"embeddings":[[1]],"usage":{"tokens":9223372036854775808}}`, `{"embeddings":[[1]],"warnings":null}`, `{"embeddings":[[1]],"warnings":[{"type":"future"}]}`, `{"embeddings":[[1]],"warnings":[{"type":"other","message":"x","feature":"y"}]}`, `{"embeddings":[[1]],"providerMetadata":{"p":null}}`}
	for i, body := range cases {
		_, err := decodeEmbeddingResult("p/m", 1, rawProviderResponse{statusCode: 200, body: []byte(body)})
		var v *ResponseValidationError
		if !errors.As(err, &v) {
			t.Fatalf("case %d err=%T %v", i, err, err)
		}
	}
}

func TestEmbeddingResponseBodyCapAndCopies(t *testing.T) {
	padding := strings.Repeat(" ", maxDiagnosticBodyBytes+100)
	body := []byte(`{"embeddings":[[1]]}` + padding)
	r, e := decodeEmbeddingResult("p/m", 1, rawProviderResponse{statusCode: 200, headers: http.Header{"X": {"a"}}, body: body})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Response.Body) != maxDiagnosticBodyBytes || r.Response.Body == nil {
		t.Fatalf("body len=%d", len(r.Response.Body))
	}
	body[0] = 'x'
	if r.Response.Body[0] == 'x' {
		t.Fatal("body shared")
	}
	r.Response.Headers.Set("X", "b")
	if reflect.DeepEqual(r.Response.Headers, http.Header{"X": {"a"}}) {
		t.Fatal("header mutation check invalid")
	}
}

func TestEmbeddingWarningsAbsentNonNil(t *testing.T) {
	r, e := decodeEmbeddingResult("p/m", 1, rawProviderResponse{statusCode: 200, body: []byte(`{"embeddings":[[1]]}`)})
	if e != nil || r.Warnings == nil || len(r.Warnings) != 0 {
		t.Fatalf("result=%#v err=%v", r, e)
	}
}

func TestEmbeddingNoRetry(t *testing.T) {
	var calls atomic.Int32
	c := embeddingClient(t, `{"error":{"message":"no"}}`, http.StatusServiceUnavailable, &calls)
	_, err := c.Embed(context.Background(), "p/m", EmbeddingRequest{Values: []string{"x"}})
	var re *ResponseError
	if !errors.As(err, &re) || calls.Load() != 1 {
		t.Fatalf("err=%T calls=%d", err, calls.Load())
	}
}

var _ = json.RawMessage{}
