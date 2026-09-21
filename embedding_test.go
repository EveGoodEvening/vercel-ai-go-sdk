package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type unsupportedEmbeddingOption struct{}

func (unsupportedEmbeddingOption) providerOption() {}

type trackingBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *trackingBody) Close() error { b.closed.Store(true); return nil }

func embeddingClient(t *testing.T, body string, status int, calls *atomic.Int32) *Client {
	t.Helper()
	s := httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Path != "/v4/ai/embedding-model" {
			t.Errorf("path=%q", r.URL.Path)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}))
	return s
}

func httptestServer(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	client, err := NewClient(WithAPIKey("key"), WithBaseURL("https://example.test/v4/ai"), WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func requestOfEncodedSize(t *testing.T, size int) EmbeddingRequest {
	t.Helper()
	values := make([]string, 16)
	base, err := prepareEmbeddingRequest("p/m", EmbeddingRequest{Values: values})
	if err != nil {
		t.Fatal(err)
	}
	remaining := size - len(base)
	if remaining < 0 {
		t.Fatalf("encoded target %d below base %d", size, len(base))
	}
	for i := range values {
		n := remaining
		if n > maxStringBytes {
			n = maxStringBytes
		}
		values[i] = strings.Repeat("x", n)
		remaining -= n
	}
	if remaining != 0 {
		t.Fatalf("cannot construct encoded size %d", size)
	}
	request := EmbeddingRequest{Values: values}
	body, err := prepareEmbeddingRequest("p/m", request)
	if size > maxRequestBodyBytes {
		if err == nil {
			t.Fatalf("oversized request accepted at %d", len(body))
		}
		return request
	}
	if err != nil || len(body) != size {
		t.Fatalf("encoded size=%d err=%v", len(body), err)
	}
	return request
}

func TestEmbeddingRequestValidationBeforeCredentialsAndNetwork(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	var calls atomic.Int32
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("must not send")
	})}))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, model  string
		ctx          context.Context
		req          EmbeddingRequest
		path, reason string
	}{
		{"nil context", "p/m", nil, EmbeddingRequest{Values: []string{"x"}}, `$["context"]`, "must not be nil"},
		{"model", "bad", context.Background(), EmbeddingRequest{Values: []string{"x"}}, `$["modelID"]`, "must be a nonempty provider/model string"},
		{"nil values", "p/m", context.Background(), EmbeddingRequest{}, `$["values"]`, "must contain 1..4096 values"},
		{"empty values", "p/m", context.Background(), EmbeddingRequest{Values: []string{}}, `$["values"]`, "must contain 1..4096 values"},
		{"too many", "p/m", context.Background(), EmbeddingRequest{Values: make([]string, 4097)}, `$["values"]`, "must contain 1..4096 values"},
		{"value bytes", "p/m", context.Background(), EmbeddingRequest{Values: []string{strings.Repeat("x", maxStringBytes+1)}}, `$["values"][0]`, "must not exceed 1048576 bytes"},
		{"nil option", "p/m", context.Background(), EmbeddingRequest{Values: []string{"x"}, ProviderOptions: []ProviderOption{nil}}, `$["providerOptions"][0]`, "must be a non-nil provider option"},
		{"unsupported option first", "p/m", context.Background(), EmbeddingRequest{Values: []string{"x"}, ProviderOptions: []ProviderOption{unsupportedEmbeddingOption{}, nil}}, `$["providerOptions"][0]`, "provider option type is unsupported"},
		{"unsupported option second", "p/m", context.Background(), EmbeddingRequest{Values: []string{"x"}, ProviderOptions: []ProviderOption{nil, unsupportedEmbeddingOption{}}}, `$["providerOptions"][0]`, "must be a non-nil provider option"},
		{"encoded body", "p/m", context.Background(), requestOfEncodedSize(t, maxRequestBodyBytes+1), "$", "encoded request exceeds 16777216 bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeSource, beforeCalls := source.callCount(), calls.Load()
			_, err := client.Embed(tc.ctx, tc.model, tc.req)
			var v *ValidationError
			if !errors.As(err, &v) || v.Path() != tc.path || v.Reason() != tc.reason {
				t.Fatalf("err=%T %v", err, err)
			}
			if source.callCount() != beforeSource || calls.Load() != beforeCalls {
				t.Fatalf("credentials=%d network=%d", source.callCount(), calls.Load())
			}
		})
	}
}

func TestEmbeddingRequestExactBoundaries(t *testing.T) {
	for _, request := range []EmbeddingRequest{{Values: []string{""}}, {Values: make([]string, 4096)}, requestOfEncodedSize(t, maxRequestBodyBytes)} {
		body, err := prepareEmbeddingRequest("p/m", request)
		if err != nil {
			t.Fatalf("boundary rejected: %v", err)
		}
		if len(request.Values) == 16 && len(body) != maxRequestBodyBytes {
			t.Fatalf("body=%d", len(body))
		}
	}
	if _, err := prepareEmbeddingRequest("p/m", EmbeddingRequest{Values: []string{strings.Repeat("x", maxStringBytes)}}); err != nil {
		t.Fatal(err)
	}
}

func TestEmbedExactWireAndResultCopies(t *testing.T) {
	var gotBody []byte
	sourceBody := []byte(`{"embeddings":[[1,2],[-3]],"usage":{"tokens":-4},"warnings":[{"type":"unsupported","feature":"f","details":"d"},{"type":"deprecated","setting":"s","message":"m"},{"type":"other","message":"o"}],"providerMetadata":{"acme":{"x":1}}}`)
	sourceHeaders := http.Header{"X-Test": {"yes"}}
	client := httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotBody, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: sourceHeaders, Body: io.NopCloser(bytes.NewReader(sourceBody)), Request: r}, nil
	}))
	result, err := client.Embed(context.Background(), "p/m", EmbeddingRequest{Values: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != `{"values":["a","b"]}` || !reflect.DeepEqual(result.Embeddings, [][]float64{{1, 2}, {-3}}) || *result.Usage.Tokens != -4 || len(result.Warnings) != 3 || result.Response.Headers.Get("X-Test") != "yes" {
		t.Fatalf("result=%#v body=%s", result, gotBody)
	}
	sourceBody[0] = 'x'
	sourceHeaders.Set("X-Test", "changed")
	if result.Response.Body[0] == 'x' || result.Response.Headers.Get("X-Test") != "yes" || string(result.ProviderMetadata["acme"]) != `{"x":1}` {
		t.Fatal("source/result storage shared")
	}
	result.Response.Body[0] = 'y'
	result.Response.Headers.Set("X-Test", "result")
	result.ProviderMetadata["acme"][0] = 'z'
	if sourceBody[0] != 'x' || sourceHeaders.Get("X-Test") != "changed" {
		t.Fatal("result/source storage shared")
	}
}

func requireEmbeddingFailure(t *testing.T, body string, count int, path, reason string) {
	t.Helper()
	_, err := decodeEmbeddingResult("p/m", count, rawProviderResponse{statusCode: 200, body: []byte(body)})
	var v *ResponseValidationError
	if !errors.As(err, &v) || v.Path() != path || v.Reason() != reason {
		t.Fatalf("body prefix %.80q: err=%T %v path=%q reason=%q", body, err, err, v.Path(), v.Reason())
	}
}

func TestEmbeddingStrictResponseCanonicalFailures(t *testing.T) {
	cases := []struct {
		name, body, path, reason string
		count                    int
	}{
		{"malformed", `{`, "$", "malformed JSON", 1}, {"trailing", `{"embeddings":[[1]]} {}`, "$", "trailing JSON value", 1},
		{"duplicate top", `{"embeddings":[[1]],"embeddings":[[2]]}`, `$["embeddings"]`, "duplicate field", 1}, {"unknown top", `{"embeddings":[[1]],"future":1}`, `$["future"]`, "unknown field", 1},
		{"missing embeddings", `{}`, `$["embeddings"]`, "required", 1}, {"null embeddings", `{"embeddings":null}`, `$["embeddings"]`, "must be an array", 1}, {"cardinality", `{"embeddings":[[1]]}`, `$["embeddings"]`, "must contain exactly 2 vectors", 2},
		{"usage duplicate", `{"embeddings":[[1]],"usage":{"tokens":1,"tokens":2}}`, `$["usage"]["tokens"]`, "duplicate field", 1}, {"usage unknown", `{"embeddings":[[1]],"usage":{"tokens":1,"x":2}}`, `$["usage"]["x"]`, "unknown field", 1},
		{"usage missing", `{"embeddings":[[1]],"usage":{}}`, `$["usage"]["tokens"]`, "required", 1}, {"usage null", `{"embeddings":[[1]],"usage":{"tokens":null}}`, `$["usage"]["tokens"]`, "required", 1},
		{"usage string", `{"embeddings":[[1]],"usage":{"tokens":"1"}}`, `$["usage"]["tokens"]`, "must be an integer", 1}, {"usage fractional", `{"embeddings":[[1]],"usage":{"tokens":1.5}}`, `$["usage"]["tokens"]`, "must be an integer", 1},
		{"usage overflow", `{"embeddings":[[1]],"usage":{"tokens":9223372036854775808}}`, `$["usage"]["tokens"]`, "must be a signed 64-bit integer", 1}, {"usage nonfinite", `{"embeddings":[[1]],"usage":{"tokens":1e999999}}`, `$["usage"]["tokens"]`, "must be a signed 64-bit integer", 1},
		{"warnings null", `{"embeddings":[[1]],"warnings":null}`, `$["warnings"]`, "must be an array", 1}, {"warning duplicate", `{"embeddings":[[1]],"warnings":[{"type":"other","type":"other","message":"x"}]}`, `$["warnings"][0]["type"]`, "duplicate field", 1},
		{"warning type", `{"embeddings":[[1]],"warnings":[{"type":"future"}]}`, `$["warnings"][0]["type"]`, "unsupported warning type", 1}, {"unsupported missing", `{"embeddings":[[1]],"warnings":[{"type":"unsupported"}]}`, `$["warnings"][0]["feature"]`, "required string", 1},
		{"compatibility unknown", `{"embeddings":[[1]],"warnings":[{"type":"compatibility","feature":"f","x":1}]}`, `$["warnings"][0]["x"]`, "unknown field", 1}, {"deprecated missing", `{"embeddings":[[1]],"warnings":[{"type":"deprecated","setting":"s"}]}`, `$["warnings"][0]["message"]`, "required string", 1},
		{"other unknown", `{"embeddings":[[1]],"warnings":[{"type":"other","message":"x","feature":"y"}]}`, `$["warnings"][0]["feature"]`, "unknown field", 1},
		{"metadata duplicate nested", `{"embeddings":[[1]],"providerMetadata":{"p":{"nested":{"x":1,"x":2}}}}`, `$["providerMetadata"]["p"]["nested"]["x"]`, "duplicate field", 1},
		{"metadata null provider", `{"embeddings":[[1]],"providerMetadata":{"p":null}}`, `$["providerMetadata"]["p"]`, "must be an object", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { requireEmbeddingFailure(t, tc.body, tc.count, tc.path, tc.reason) })
	}
}

func TestEmbeddingMetadataBoundsAndArbitraryNestedAcceptance(t *testing.T) {
	body := `{"embeddings":[[1]],"providerMetadata":{"p":{"object":{"array":[null,true,3.5,"x",{"future":"ok"}]}}}}`
	r, err := decodeEmbeddingResult("p/m", 1, rawProviderResponse{body: []byte(body)})
	if err != nil || string(r.ProviderMetadata["p"]) != `{"object":{"array":[null,true,3.5,"x",{"future":"ok"}]}}` {
		t.Fatalf("result=%#v err=%v", r, err)
	}
	requireEmbeddingFailure(t, `{"embeddings":[[1]],"providerMetadata":{"p":{"x":"`+strings.Repeat("a", maxStringBytes+1)+`"}}}`, 1, `$["providerMetadata"]["p"]["x"]`, "string exceeds 1 MiB")
	deep := strings.Repeat(`{"x":`, maxJSONDepth) + `0` + strings.Repeat(`}`, maxJSONDepth)
	requireEmbeddingFailure(t, `{"embeddings":[[1]],"providerMetadata":{"p":`+deep+`}}`, 1, `$["providerMetadata"]["p"]`+strings.Repeat(`["x"]`, 62), "maximum depth is 64")
}

func metadataObjectJSON(n int) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := range n {
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(strconv.Itoa(i)))
		b.WriteString(`:null`)
	}
	b.WriteByte('}')
	return b.String()
}

func repeatedJSON(value string, n int) string {
	if n == 0 {
		return "[]"
	}
	return "[" + strings.Repeat(value+",", n-1) + value + "]"
}

func providerMetadataJSON(n int) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := range n {
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(strconv.Itoa(i)))
		b.WriteString(`:{}`)
	}
	b.WriteByte('}')
	return b.String()
}

func TestEmbeddingMetadataCollectionBounds(t *testing.T) {
	object := metadataObjectJSON(maxCollectionItems)
	array := vectorJSON(maxCollectionItems)
	providerRaw := `{"nestedObject":` + object + `,"nestedArray":` + array + `}`
	body := `{"embeddings":[[1]],"providerMetadata":{"p":` + providerRaw + `}}`
	r, err := decodeEmbeddingResult("p/m", 1, rawProviderResponse{body: []byte(body)})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(r.ProviderMetadata["p"]); got != providerRaw {
		t.Fatalf("provider metadata raw value changed: got %.80q want %.80q", got, providerRaw)
	}

	requireEmbeddingFailure(t, `{"embeddings":[[1]],"providerMetadata":{"p":{"nestedObject":`+metadataObjectJSON(maxCollectionItems+1)+`}}}`, 1, `$["providerMetadata"]["p"]["nestedObject"]`, "object exceeds 4096 members")
	requireEmbeddingFailure(t, `{"embeddings":[[1]],"providerMetadata":{"p":{"nestedArray":`+vectorJSON(maxCollectionItems+1)+`}}}`, 1, `$["providerMetadata"]["p"]["nestedArray"]`, "array exceeds 4096 members")
}

func TestEmbeddingResponseCollectionBounds(t *testing.T) {
	embeddings := repeatedJSON(`[0]`, maxCollectionItems)
	r, err := decodeEmbeddingResult("p/m", maxCollectionItems, rawProviderResponse{body: []byte(`{"embeddings":` + embeddings + `}`)})
	if err != nil || len(r.Embeddings) != maxCollectionItems {
		t.Fatalf("embeddings boundary: len=%d err=%v", len(r.Embeddings), err)
	}
	requireEmbeddingFailure(t, `{"embeddings":`+repeatedJSON(`[0]`, maxCollectionItems+1)+`}`, maxCollectionItems+1, `$["embeddings"]`, "array exceeds 4096 members")

	warnings := repeatedJSON(`{"type":"other","message":"x"}`, maxCollectionItems)
	r, err = decodeEmbeddingResult("p/m", 1, rawProviderResponse{body: []byte(`{"embeddings":[[0]],"warnings":` + warnings + `}`)})
	if err != nil || len(r.Warnings) != maxCollectionItems {
		t.Fatalf("warnings boundary: len=%d err=%v", len(r.Warnings), err)
	}
	requireEmbeddingFailure(t, `{"embeddings":[[0]],"warnings":`+repeatedJSON(`{"type":"other","message":"x"}`, maxCollectionItems+1)+`}`, 1, `$["warnings"]`, "array exceeds 4096 members")

	metadata := providerMetadataJSON(maxCollectionItems)
	r, err = decodeEmbeddingResult("p/m", 1, rawProviderResponse{body: []byte(`{"embeddings":[[0]],"providerMetadata":` + metadata + `}`)})
	if err != nil || len(r.ProviderMetadata) != maxCollectionItems {
		t.Fatalf("providerMetadata boundary: len=%d err=%v", len(r.ProviderMetadata), err)
	}
	requireEmbeddingFailure(t, `{"embeddings":[[0]],"providerMetadata":`+providerMetadataJSON(maxCollectionItems+1)+`}`, 1, `$["providerMetadata"]`, "object exceeds 4096 members")
}

func vectorJSON(n int) string {
	if n == 0 {
		return "[]"
	}
	return "[" + strings.Repeat("0,", n-1) + "0]"
}

func TestEmbeddingVectorBounds(t *testing.T) {
	for _, n := range []int{1, 65536} {
		body := `{"embeddings":[` + vectorJSON(n) + `]}`
		r, err := decodeEmbeddingResult("p/m", 1, rawProviderResponse{body: []byte(body)})
		if err != nil || len(r.Embeddings[0]) != n {
			t.Fatalf("n=%d err=%v", n, err)
		}
	}
	requireEmbeddingFailure(t, `{"embeddings":[[]]}`, 1, `$["embeddings"][0]`, "must contain 1..65536 numbers")
	requireEmbeddingFailure(t, `{"embeddings":[`+vectorJSON(65537)+`]}`, 1, `$["embeddings"][0]`, "must contain 1..65536 numbers")
	requireEmbeddingFailure(t, `{"embeddings":[[1e999]]}`, 1, `$["embeddings"][0]`, "must be an array")
	r, err := decodeEmbeddingResult("p/m", 2, rawProviderResponse{body: []byte(`{"embeddings":[[1],[2,3]]}`)})
	if err != nil || !reflect.DeepEqual(r.Embeddings, [][]float64{{1}, {2, 3}}) {
		t.Fatalf("unequal dimensions: %#v %v", r, err)
	}
	vectors := make([]string, 64)
	for i := range vectors {
		vectors[i] = vectorJSON(65536)
	}
	body := `{"embeddings":[` + strings.Join(vectors, ",") + `]}`
	if r, err := decodeEmbeddingResult("p/m", 64, rawProviderResponse{body: []byte(body)}); err != nil || len(r.Embeddings) != 64 {
		t.Fatalf("aggregate boundary err=%v", err)
	}
	body = `{"embeddings":[` + strings.Join(append(vectors, `[0]`), ",") + `]}`
	requireEmbeddingFailure(t, body, 65, `$["embeddings"]`, "aggregate elements exceed 4194304")
}

func TestEmbeddingUsageExactInt64AndWarningsAbsent(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		nilUsage bool
		want     int64
	}{{``, true, 0}, {`null`, true, 0}, {`{"tokens":-9223372036854775808}`, false, math.MinInt64}, {`{"tokens":9223372036854775807}`, false, math.MaxInt64}, {`{"tokens":1e3}`, false, 1000}, {`{"tokens":1.0}`, false, 1}} {
		top := `{"embeddings":[[1]]}`
		if tc.raw != "" {
			top = `{"embeddings":[[1]],"usage":` + tc.raw + `}`
		}
		r, err := decodeEmbeddingResult("p/m", 1, rawProviderResponse{body: []byte(top)})
		if err != nil || (r.Usage == nil) != tc.nilUsage || (!tc.nilUsage && *r.Usage.Tokens != tc.want) || r.Warnings == nil {
			t.Fatalf("result=%#v err=%v", r, err)
		}
	}
}

func TestEmbedCancellationAndBodyClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	client := httptestServer(t, roundTripFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("must not run") }))
	if _, err := client.Embed(ctx, "p/m", EmbeddingRequest{Values: []string{"x"}}); !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
	body := &trackingBody{Reader: strings.NewReader(`{"embeddings":[[1]]}`)}
	client = httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: r}, nil
	}))
	if _, err := client.Embed(context.Background(), "p/m", EmbeddingRequest{Values: []string{"x"}}); err != nil || !body.closed.Load() {
		t.Fatalf("err=%v closed=%v", err, body.closed.Load())
	}
}

func TestEmbeddingSuccessBodyReaderBoundary(t *testing.T) {
	for _, extra := range []int{0, 1} {
		t.Run(strconv.Itoa(extra), func(t *testing.T) {
			prefix := `{"embeddings":[[1]]}`
			body := &trackingBody{Reader: io.MultiReader(strings.NewReader(prefix), io.LimitReader(zeroReader{}, int64(maxEmbeddingSuccessBodyBytes-len(prefix)+extra)))}
			client := httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body, Request: r, ContentLength: -1}, nil
			}))
			_, err := client.Embed(context.Background(), "p/m", EmbeddingRequest{Values: []string{"x"}})
			if extra == 0 && err != nil {
				t.Fatal(err)
			}
			if extra == 1 {
				var te *TransportError
				if !errors.As(err, &te) || te.Operation() != "read response body" {
					t.Fatalf("err=%T %v", err, err)
				}
			}
			if !body.closed.Load() {
				t.Fatal("body not closed")
			}
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestEmbeddingNon200PreservesEnvelopeAndBody(t *testing.T) {
	body := []byte(`{"error":{"message":"no","type":"upstream","code":429,"param":{"x":[1]}},"requestId":"req","responseId":"res","generationId":"gen"}`)
	client := httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"2"}}, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
	}))
	_, err := client.Embed(context.Background(), "p/m", EmbeddingRequest{Values: []string{"x"}})
	var re *ResponseError
	if !errors.As(err, &re) || re.StatusCode() != 429 || re.Message() != "no" || re.Type() != "upstream" || re.Code() != "429" || re.RequestID() != "req" || re.ResponseID() != "res" || re.GenerationID() != "gen" || !re.Retryable() {
		t.Fatalf("err=%T %#v", err, re)
	}
	if d, ok := re.RetryAfter(); !ok || d != 2*time.Second {
		t.Fatalf("retry-after=%v %v", d, ok)
	}
	got := re.RawResponseBody()
	if !bytes.Equal(got, body) {
		t.Fatal("body not preserved")
	}
	got[0] = 'x'
	if re.RawResponseBody()[0] == 'x' {
		t.Fatal("body accessor aliases")
	}
}

var _ = json.RawMessage{}
