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
	"strings"
	"sync/atomic"
	"testing"
)

type unsupportedRerankDocuments struct{}

func (unsupportedRerankDocuments) rerankDocuments() {}

func TestRerankRequestValidationBeforeCredentialsAndNetwork(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	var calls atomic.Int32
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("sent") })}))
	if err != nil {
		t.Fatal(err)
	}
	tooMany := make([]string, maxCollectionItems+1)
	one := 1
	zero := 0
	cases := []struct {
		name, model string
		ctx         context.Context
		req         RerankRequest
		path        string
	}{
		{"nil context", "p/m", nil, RerankRequest{Documents: RerankTexts{Values: []string{"x"}}}, `$["context"]`},
		{"model", "bad", context.Background(), RerankRequest{Documents: RerankTexts{Values: []string{"x"}}}, `$["modelID"]`},
		{"nil documents", "p/m", context.Background(), RerankRequest{}, `$["documents"]`},
		{"typed nil texts", "p/m", context.Background(), RerankRequest{Documents: (*RerankTexts)(nil)}, `$["documents"]`},
		{"unsupported", "p/m", context.Background(), RerankRequest{Documents: unsupportedRerankDocuments{}}, `$["documents"]`},
		{"empty", "p/m", context.Background(), RerankRequest{Documents: RerankTexts{}}, `$["documents"]["values"]`},
		{"too many", "p/m", context.Background(), RerankRequest{Documents: RerankTexts{Values: tooMany}}, `$["documents"]["values"]`},
		{"query", "p/m", context.Background(), RerankRequest{Query: strings.Repeat("q", maxStringBytes+1), Documents: RerankTexts{Values: []string{"x"}}}, `$["query"]`},
		{"text", "p/m", context.Background(), RerankRequest{Documents: RerankTexts{Values: []string{strings.Repeat("x", maxStringBytes+1)}}}, `$["documents"]["values"][0]`},
		{"object kind", "p/m", context.Background(), RerankRequest{Documents: RerankObjects{Values: []json.RawMessage{json.RawMessage(`[]`)}}}, `$["documents"]["values"][0]`},
		{"object duplicate", "p/m", context.Background(), RerankRequest{Documents: RerankObjects{Values: []json.RawMessage{json.RawMessage(`{"x":1,"x":2}`)}}}, `$["documents"]["values"][0]["x"]`},
		{"top zero", "p/m", context.Background(), RerankRequest{Documents: RerankTexts{Values: []string{"x"}}, TopN: &zero}, `$["topN"]`},
		{"top high", "p/m", context.Background(), RerankRequest{Documents: RerankTexts{Values: []string{"x"}}, TopN: new(2)}, `$["topN"]`},
		{"option", "p/m", context.Background(), RerankRequest{Documents: RerankTexts{Values: []string{"x"}}, TopN: &one, ProviderOptions: []ProviderOption{nil}}, `$["providerOptions"][0]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc, nc := source.callCount(), calls.Load()
			_, err := client.Rerank(tc.ctx, tc.model, tc.req)
			var v *ValidationError
			if !errors.As(err, &v) || v.Path() != tc.path {
				t.Fatalf("err=%T %v", err, err)
			}
			if source.callCount() != sc || calls.Load() != nc {
				t.Fatal("performed side effect")
			}
		})
	}
}

func TestRerankExactTextWireOrderAndResult(t *testing.T) {
	var got []byte
	var calls atomic.Int32
	client := httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Path != "/v4/ai/reranking-model" {
			t.Errorf("path=%s", r.URL.Path)
		}
		got, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: http.Header{"X-Test": {"yes"}}, Body: io.NopCloser(strings.NewReader(`{"ranking":[{"index":1.0,"relevanceScore":-3.5},{"index":0e0,"relevanceScore":2}],"warnings":[],"providerMetadata":{"p":{"x":1}}}`)), Request: r}, nil
	}))
	top := 2
	result, err := client.Rerank(context.Background(), "p/m", RerankRequest{Query: "q", Documents: RerankTexts{Values: []string{"a", "b"}}, TopN: &top})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"query":"q","documents":{"type":"text","values":["a","b"]},"topN":2}` || calls.Load() != 1 {
		t.Fatalf("body=%s calls=%d", got, calls.Load())
	}
	if len(result.Results) != 2 || result.Results[0].OriginalIndex != 1 || result.Results[0].Score != -3.5 || result.Results[0].Document != (RerankText{Text: "b"}) || result.Results[1].Document != (RerankText{Text: "a"}) {
		t.Fatalf("result=%#v", result)
	}
	if result.Warnings == nil || result.Response.Headers.Get("X-Test") != "yes" || string(result.ProviderMetadata["p"]) != `{"x":1}` {
		t.Fatalf("metadata=%#v", result)
	}
}

func TestRerankObjectCompactionLexemesAndCopies(t *testing.T) {
	source := json.RawMessage(` { "z" : 1e+09 , "a" : 9007199254740993 , "s" : "\u003c&" } `)
	request := RerankRequest{Query: "", Documents: RerankObjects{Values: []json.RawMessage{source}}}
	prepared, err := prepareRerankRequest("p/m", request)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"query":"","documents":{"type":"object","values":[{"z":1e+09,"a":9007199254740993,"s":"\u003c&"}]}}`
	if string(prepared.payload) != want {
		t.Fatalf("got=%s", prepared.payload)
	}
	source[2] = 'X'
	if string(prepared.objects[0]) != `{"z":1e+09,"a":9007199254740993,"s":"\u003c&"}` {
		t.Fatal("caller alias")
	}
	result, err := decodeRerankResult("p/m", prepared, rawProviderResponse{statusCode: 200, body: []byte(`{"ranking":[{"index":0,"relevanceScore":1}]}`)})
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := result.Results[0].Document.(RerankJSON)
	if !ok {
		t.Fatalf("document=%T", result.Results[0].Document)
	}
	doc.Value[0] = 'X'
	if prepared.objects[0][0] == 'X' {
		t.Fatal("result alias")
	}
}

func requireRerankFailure(t *testing.T, body, path string) {
	t.Helper()
	p, _ := prepareRerankRequest("p/m", RerankRequest{Documents: RerankTexts{Values: []string{"a", "b"}}, TopN: new(1)})
	_, err := decodeRerankResult("p/m", p, rawProviderResponse{statusCode: 200, body: []byte(body)})
	var v *ResponseValidationError
	if !errors.As(err, &v) || v.Path() != path {
		t.Fatalf("err=%T %v", err, err)
	}
}
func TestRerankStrictResponseFailures(t *testing.T) {
	cases := []struct{ body, path string }{
		{`{`, `$`}, {`{"ranking":[]} {}`, `$`}, {`{"ranking":[],"ranking":[]}`, `$["ranking"]`}, {`{}`, `$["ranking"]`}, {`{"ranking":null}`, `$["ranking"]`}, {`{"ranking":[],"x":1}`, `$["x"]`},
		{`{"ranking":[null]}`, `$["ranking"][0]`}, {`{"ranking":[{"index":0,"relevanceScore":1,"x":2}]}`, `$["ranking"][0]["x"]`}, {`{"ranking":[{"relevanceScore":1}]}`, `$["ranking"][0]["index"]`},
		{`{"ranking":[{"index":0.5,"relevanceScore":1}]}`, `$["ranking"][0]["index"]`}, {`{"ranking":[{"index":2,"relevanceScore":1}]}`, `$["ranking"][0]["index"]`}, {`{"ranking":[{"index":0,"relevanceScore":1},{"index":0,"relevanceScore":2}]}`, `$["ranking"]`},
		{`{"ranking":[{"index":0,"relevanceScore":1e9999}]}`, `$["ranking"][0]["relevanceScore"]`}, {`{"ranking":[],"warnings":null}`, `$["warnings"]`}, {`{"ranking":[],"providerMetadata":{"p":null}}`, `$["providerMetadata"]["p"]`},
	}
	for _, c := range cases {
		requireRerankFailure(t, c.body, c.path)
	}
}

func TestRerankUnrestrictedFiniteScoresAndEmptyRanking(t *testing.T) {
	p, _ := prepareRerankRequest("p/m", RerankRequest{Documents: RerankTexts{Values: []string{"a", "b"}}})
	for _, score := range []float64{-math.MaxFloat64, 0, math.MaxFloat64} {
		body := `{"ranking":[{"index":1e0,"relevanceScore":` + jsonNumber(score) + `}]}`
		r, err := decodeRerankResult("p/m", p, rawProviderResponse{body: []byte(body)})
		if err != nil || r.Results[0].Score != score {
			t.Fatalf("score=%g err=%v", score, err)
		}
	}
	r, err := decodeRerankResult("p/m", p, rawProviderResponse{body: []byte(`{"ranking":[]}`)})
	if err != nil || !reflect.DeepEqual(r.Results, []RerankItem{}) {
		t.Fatalf("result=%#v err=%v", r, err)
	}
}
func jsonNumber(v float64) string { return strings.ToLower(strings.TrimSpace(string(mustJSON(v)))) }
func mustJSON(v any) []byte       { b, _ := json.Marshal(v); return b }

func TestRerankSuccessBodyLimitAndCancellation(t *testing.T) {
	var calls atomic.Int32
	client := httptestServer(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(io.MultiReader(strings.NewReader(`{"ranking":[]}`), bytes.NewReader(make([]byte, maxRerankingSuccessBodyBytes)))), Request: r}, nil
	}))
	_, err := client.Rerank(context.Background(), "p/m", RerankRequest{Documents: RerankTexts{Values: []string{"a"}}})
	var tr *TransportError
	if !errors.As(err, &tr) || calls.Load() != 1 {
		t.Fatalf("err=%T %v calls=%d", err, err, calls.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Rerank(ctx, "p/m", RerankRequest{Documents: RerankTexts{Values: []string{"a"}}})
	if !errors.As(err, &tr) {
		t.Fatalf("err=%T %v", err, err)
	}
}
