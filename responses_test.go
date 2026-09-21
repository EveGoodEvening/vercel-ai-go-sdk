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
	"testing"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/testserver"
)

func validResponsesRequest() ResponsesRequest {
	return ResponsesRequest{Model: "provider/model", Input: ResponseTextInput("hello")}
}

func TestResponsesRequestExactJSONAndInventory(t *testing.T) {
	description, instructions, summary := "description", "be concise", "detailed"
	strict, parallel, store := true, false, true
	maxTokens, anchor := 123, 2
	temperature, topP, presence, frequency := 0.25, 0.75, -0.5, 0.5
	truncation, previous, caching, ttl, cacheKey := "disabled", "resp_previous", "auto", "1h", "cache-key"
	request := ResponsesRequest{
		Model: "provider/model",
		Input: ResponseItemsInput{
			ResponseMessage{Role: "user", Content: "hello"},
			ResponseFunctionCall{ID: "item-1", CallID: "call-1", Name: "weather", Arguments: `{"city":"Paris"}`},
			ResponseFunctionCallOutput{CallID: "call-1", Output: `{"temp":21}`},
		},
		MaxOutputTokens: &maxTokens, Temperature: &temperature, TopP: &topP,
		PresencePenalty: &presence, FrequencyPenalty: &frequency, Instructions: &instructions,
		Tools:      []ResponseTool{{Name: "weather", Description: &description, Parameters: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}, Strict: &strict}},
		ToolChoice: ResponseSpecificToolChoice{Name: "weather"}, ParallelToolCalls: &parallel,
		AllowedTools: []string{"weather"}, Reasoning: &ResponseReasoning{Effort: "high", Summary: &summary},
		Text:       &ResponseText{Format: ResponseJSONSchemaFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict}},
		Truncation: &truncation, PreviousResponseID: &previous, Store: &store,
		Metadata: map[string]string{"trace": "abc"}, Caching: &caching, CacheAnchorItems: &anchor,
		CacheTTL: &ttl, PromptCacheKey: &cacheKey,
	}
	got, err := encodeResponsesRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"allowed_tools":["weather"],"cache_anchor_items":2,"cache_ttl":"1h","caching":"auto","frequency_penalty":0.5,"input":[{"content":"hello","role":"user"},{"arguments":"{\"city\":\"Paris\"}","call_id":"call-1","id":"item-1","name":"weather","type":"function_call"},{"call_id":"call-1","output":"{\"temp\":21}","type":"function_call_output"}],"instructions":"be concise","max_output_tokens":123,"metadata":{"trace":"abc"},"model":"provider/model","parallel_tool_calls":false,"presence_penalty":-0.5,"previous_response_id":"resp_previous","prompt_cache_key":"cache-key","reasoning":{"effort":"high","summary":"detailed"},"store":true,"stream":false,"temperature":0.25,"text":{"format":{"description":"description","name":"answer","schema":{"type":"object"},"strict":true,"type":"json_schema"}},"tool_choice":{"name":"weather","type":"function"},"tools":[{"description":"description","name":"weather","parameters":{"properties":{"city":{"type":"string"}},"type":"object"},"strict":true,"type":"function"}],"top_p":0.75,"truncation":"disabled"}`
	if string(got) != want {
		t.Fatalf("encoded request\n got: %s\nwant: %s", got, want)
	}
}

func TestResponsesRequestOmissionAndAlternateForms(t *testing.T) {
	for _, test := range []struct {
		name    string
		request ResponsesRequest
		want    string
	}{
		{"minimal", validResponsesRequest(), `{"input":"hello","model":"provider/model","stream":false}`},
		{"empty arrays retained", ResponsesRequest{Model: "provider/model", Input: ResponseItemsInput{}, Tools: []ResponseTool{}, AllowedTools: []string{}, Metadata: map[string]string{}}, `{"allowed_tools":[],"input":[],"metadata":{},"model":"provider/model","stream":false,"tools":[]}`},
		{"mode choice and text", ResponsesRequest{Model: "provider/model", Input: ResponseTextInput("x"), ToolChoice: ResponseToolChoiceNone, Text: &ResponseText{Format: ResponseTextFormatJSONObject}}, `{"input":"x","model":"provider/model","stream":false,"text":{"format":{"type":"json_object"}},"tool_choice":"none"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := encodeResponsesRequest(test.request)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestCreateResponseEndpointHeadersAndRawResult(t *testing.T) {
	clearCredentialEnvironment(t)
	raw := []byte(" {\n  \"unknown\": [null, {\"variant\":\"future\"}]\n} \t")
	fixture := testserver.New(testserver.Response{Status: http.StatusOK, Body: raw})
	defer fixture.Close()
	client, err := NewClient(WithAPIKey("secret"), WithPublicBaseURL(fixture.URL+"/v1///"), WithHeaders(http.Header{"X-Custom": {"one", "two"}}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CreateResponse(context.Background(), validResponsesRequest())
	if err != nil {
		t.Fatal(err)
	}
	requests := fixture.Requests()
	if len(requests) != 1 {
		t.Fatalf("requests = %d", len(requests))
	}
	got := requests[0]
	if got.Method != http.MethodPost || got.URL != "/v1/responses" || string(got.Body) != `{"input":"hello","model":"provider/model","stream":false}` {
		t.Fatalf("request = %#v", got)
	}
	if got.Header.Get("Authorization") != "Bearer secret" || got.Header.Get("Content-Type") != "application/json" || !reflect.DeepEqual(got.Header.Values("X-Custom"), []string{"one", "two"}) {
		t.Fatalf("headers = %#v", got.Header)
	}
	if !bytes.Equal(result.RawJSON(), raw) {
		t.Fatalf("raw = %q", result.RawJSON())
	}
	first := result.RawJSON()
	first[0] = 'x'
	if bytes.Equal(first, result.RawJSON()) {
		t.Fatal("RawJSON did not return a defensive copy")
	}
	if got := (*ResponseResult)(nil).RawJSON(); got != nil {
		t.Fatalf("nil receiver RawJSON = %q", got)
	}
}

func TestCreateResponseValidationBeforeCredentialAndNetwork(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	calls := 0
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("must not send") })}))
	if err != nil {
		t.Fatal(err)
	}
	invalid := []ResponsesRequest{
		{},
		{Model: "provider/model", Input: ResponseTextInput("")},
		{Model: "provider", Input: ResponseTextInput("x")},
		{Model: "provider/model", Input: ResponseItemsInput(nil)},
		{Model: "provider/model", Input: ResponseTextInput("x"), Temperature: new(math.NaN())},
		{Model: "provider/model", Input: ResponseTextInput("x"), TopP: new(1.01)},
	}
	for i, request := range invalid {
		_, err := client.CreateResponse(context.Background(), request)
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("case %d error = %T %v, want ValidationError", i, err, err)
		}
	}
	if _, err := client.CreateResponse(nil, validResponsesRequest()); err == nil {
		t.Fatal("nil context accepted")
	}
	if calls != 0 || source.callCount() != 0 {
		t.Fatalf("network calls=%d credential calls=%d", calls, source.callCount())
	}
}

func TestResponsesRequestResourceLimits(t *testing.T) {
	tooDeep := any("leaf")
	for range 65 {
		tooDeep = []any{tooDeep}
	}
	tooMany := make([]ResponseInputItem, 10001)
	for i := range tooMany {
		tooMany[i] = ResponseMessage{Role: "user", Content: "x"}
	}
	tooManyMembers := make(map[string]any, 10001)
	for i := range 10001 {
		tooManyMembers[string(rune(0x1000+i))] = true
	}
	cases := []ResponsesRequest{
		{Model: "provider/model", Input: ResponseTextInput(strings.Repeat("x", (1<<20)+1))},
		{Model: "provider/model", Input: ResponseItemsInput(tooMany)},
		{Model: "provider/model", Input: ResponseTextInput("x"), Tools: make([]ResponseTool, 10001)},
		{Model: "provider/model", Input: ResponseTextInput("x"), Tools: []ResponseTool{{Name: "f", Parameters: tooDeep}}},
		{Model: "provider/model", Input: ResponseTextInput("x"), Tools: []ResponseTool{{Name: "f", Parameters: tooManyMembers}}},
	}
	for i, request := range cases {
		if err := validateResponsesRequest(request); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestResponsesRequestStringByteCeilings(t *testing.T) {
	exactModel := "p/" + strings.Repeat("m", maxResponseValueBytes-2)
	overModel := "p/" + strings.Repeat("m", maxResponseValueBytes-1)
	exactName := strings.Repeat("n", maxResponseValueBytes)
	overName := strings.Repeat("n", maxResponseValueBytes+1)
	tests := []struct {
		name    string
		request ResponsesRequest
		valid   bool
	}{
		{"model at limit", ResponsesRequest{Model: exactModel, Input: ResponseTextInput("x")}, true},
		{"model over limit", ResponsesRequest{Model: overModel, Input: ResponseTextInput("x")}, false},
		{"specific tool at limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), ToolChoice: ResponseSpecificToolChoice{Name: exactName}}, true},
		{"specific tool over limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), ToolChoice: ResponseSpecificToolChoice{Name: overName}}, false},
		{"schema name at limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Text: &ResponseText{Format: ResponseJSONSchemaFormat{Name: exactName, Schema: map[string]any{}}}}, true},
		{"schema name over limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Text: &ResponseText{Format: ResponseJSONSchemaFormat{Name: overName, Schema: map[string]any{}}}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateResponsesRequest(test.request)
			if (err == nil) != test.valid {
				t.Fatalf("validation error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestResponsesRequestCharacterLimitsUseCodePoints(t *testing.T) {
	promptAtLimit := strings.Repeat("界", 64)
	promptOverLimit := promptAtLimit + "界"
	keyAtLimit := strings.Repeat("鍵", 64)
	keyOverLimit := keyAtLimit + "鍵"
	valueAtLimit := strings.Repeat("値", 512)
	valueOverLimit := valueAtLimit + "値"
	tests := []struct {
		name    string
		request ResponsesRequest
		valid   bool
	}{
		{"prompt cache key at limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), PromptCacheKey: &promptAtLimit}, true},
		{"prompt cache key over limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), PromptCacheKey: &promptOverLimit}, false},
		{"metadata key at limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Metadata: map[string]string{keyAtLimit: "x"}}, true},
		{"metadata key over limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Metadata: map[string]string{keyOverLimit: "x"}}, false},
		{"metadata value at limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Metadata: map[string]string{"k": valueAtLimit}}, true},
		{"metadata value over limit", ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Metadata: map[string]string{"k": valueOverLimit}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateResponsesRequest(test.request)
			if (err == nil) != test.valid {
				t.Fatalf("validation error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestResponsesRequestPointerCyclesAndDepth(t *testing.T) {
	var direct any
	direct = &direct
	var first, second any
	first = &second
	second = &first
	chain := any("leaf")
	for range 65 {
		value := chain
		chain = &value
	}
	tests := []struct {
		name   string
		value  any
		reason string
	}{
		{"direct cycle", direct, "cycle detected"},
		{"multi-pointer cycle", first, "cycle detected"},
		{"pointer chain over depth limit", chain, "maximum depth is 64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateResponsesRequest(ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Tools: []ResponseTool{{Name: "f", Parameters: test.value}}})
			if err == nil || err.Path() != `$["tools"][0]["parameters"]` || err.Reason() != test.reason {
				t.Fatalf("validation error = %#v", err)
			}
		})
	}
}

func TestResponsesResultDecoderPolicy(t *testing.T) {
	valid := []string{`{}`, " {\"a\":null,\"future\":{\"x\":[1,true,\"s\"]}} \n"}
	for _, body := range valid {
		result, err := decodeResponseResult([]byte(body))
		if err != nil {
			t.Fatalf("valid %q: %v", body, err)
		}
		if string(result.RawJSON()) != body {
			t.Fatalf("raw changed: %q", result.RawJSON())
		}
	}
	deep := strings.Repeat("{\"x\":", 65) + "0" + strings.Repeat("}", 65)
	members := make([]string, 10001)
	for i := range members {
		members[i] = "0"
	}
	objectMembers := make([]string, 10001)
	for i := range objectMembers {
		objectMembers[i] = `"` + string(rune(0x1000+i)) + `":0`
	}
	invalid := []string{"", `null`, `[]`, `true`, `1`, `{"x":`, `{} {}`, `{"a":1,"a":2}`, `{"nested":{"a":1,"a":2}}`, deep, `{"s":"` + strings.Repeat("x", (1<<20)+1) + `"}`, `{"a":[` + strings.Join(members, ",") + `]}`, `{` + strings.Join(objectMembers, ",") + `}`}
	for i, body := range invalid {
		_, err := decodeResponseResult([]byte(body))
		var validation *ResponseValidationError
		if !errors.As(err, &validation) || validation.StatusCode() != http.StatusOK || validation.Path() != "$" {
			t.Fatalf("case %d error = %T %v", i, err, err)
		}
		raw := validation.RawResponseBody()
		if len(raw) > 0 {
			raw[0] ^= 0xff
			if bytes.Equal(raw, validation.RawResponseBody()) {
				t.Fatalf("case %d raw body is not defensive", i)
			}
		}
	}
}

func TestResponsesResultRejectsInvalidUTF8(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"top-level key", []byte{'{', '"', 0xff, '"', ':', '0', '}'}},
		{"nested key", []byte{'{', '"', 'x', '"', ':', '{', '"', 0xff, '"', ':', '0', '}', '}'}},
		{"string value", []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeResponseResult(test.body)
			var validation *ResponseValidationError
			if !errors.As(err, &validation) || validation.StatusCode() != http.StatusOK || validation.Path() != "$" || validation.Reason() != "invalid UTF-8" {
				t.Fatalf("error = %T %v", err, err)
			}
			if !bytes.Equal(validation.RawResponseBody(), test.body) {
				t.Fatalf("raw body = %q, want %q", validation.RawResponseBody(), test.body)
			}
			raw := validation.RawResponseBody()
			raw[0] ^= 0xff
			if bytes.Equal(raw, validation.RawResponseBody()) {
				t.Fatal("raw body is not defensive")
			}
		})
	}
}

func TestCreateResponseSuccessBodyOverflowAndNon200(t *testing.T) {
	for _, test := range []struct {
		name         string
		status       int
		body         []byte
		wantResponse bool
	}{
		{"success overflow", 200, bytes.Repeat([]byte("x"), (1<<20)+1), false},
		{"created is not success", 201, []byte(`{"future":true}`), true},
		{"gateway error", 429, []byte(`{"error":{"message":"limited","type":"rate_limit"},"requestId":"req"}`), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(test.body)), Request: request}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateResponse(context.Background(), validResponsesRequest())
			if err == nil {
				t.Fatal("expected error")
			}
			if test.status == http.StatusOK {
				var validation *ResponseValidationError
				if !errors.As(err, &validation) {
					t.Fatalf("error = %T %v, want ResponseValidationError", err, err)
				}
				if validation.StatusCode() != http.StatusOK || validation.Path() != "$" || validation.Reason() != "response body exceeds 1 MiB" || !validation.BodyTruncated() {
					t.Fatalf("status=%d path=%q reason=%q truncated=%v", validation.StatusCode(), validation.Path(), validation.Reason(), validation.BodyTruncated())
				}
				if raw := validation.RawResponseBody(); len(raw) != 1<<20 || !bytes.Equal(raw, test.body[:1<<20]) {
					t.Fatalf("raw body length = %d", len(validation.RawResponseBody()))
				}
				if !errors.Is(err, httpx.ErrResponseBodyTooLarge) {
					t.Fatalf("overflow cause not retained: %v", err)
				}
				var transportErr *TransportError
				if errors.As(err, &transportErr) {
					t.Fatalf("overflow returned TransportError: %v", err)
				}
				return
			}
			var responseErr *ResponseError
			if errors.As(err, &responseErr) != test.wantResponse {
				t.Fatalf("ResponseError presence=%v error=%T %v", errors.As(err, &responseErr), err, err)
			}
			if responseErr != nil && responseErr.StatusCode() != test.status {
				t.Fatalf("status=%d", responseErr.StatusCode())
			}
		})
	}
}

func TestCreateResponseCancellationInterruptsSendAndRead(t *testing.T) {
	t.Run("send", func(t *testing.T) {
		started := make(chan struct{})
		client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			close(started)
			<-request.Context().Done()
			return nil, request.Context().Err()
		})}))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { _, err := client.CreateResponse(ctx, validResponsesRequest()); done <- err }()
		<-started
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("send did not stop")
		}
	})
	t.Run("read", func(t *testing.T) {
		reader, writer := io.Pipe()
		started := make(chan struct{})
		client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			close(started)
			go func() { <-request.Context().Done(); _ = writer.CloseWithError(request.Context().Err()) }()
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader, Request: request}, nil
		})}))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { _, err := client.CreateResponse(ctx, validResponsesRequest()); done <- err }()
		<-started
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("read did not stop")
		}
	})
}

func TestCreateResponseUsesPublicBaseWithoutChangingEvaluate(t *testing.T) {
	public := testserver.New(testserver.Response{Body: []byte(`{}`)})
	defer public.Close()
	provider := testserver.New(testserver.Response{Body: []byte(`{"answers":{"q":{"type":"boolean","probability":1}}}`)})
	defer provider.Close()
	client, err := NewClient(WithAPIKey("key"), WithPublicBaseURL(public.URL+"/v1"), WithBaseURL(provider.URL+"/v4/ai"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CreateResponse(context.Background(), validResponsesRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Evaluate(context.Background(), "provider/model", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "yes?"}}}); err != nil {
		t.Fatal(err)
	}
	if got := public.Requests()[0].URL; got != "/v1/responses" {
		t.Fatalf("public URL=%q", got)
	}
	if got := provider.Requests()[0].URL; got != "/v4/ai/evaluation-model" {
		t.Fatalf("provider URL=%q", got)
	}
}

type unsupportedResponseBuiltInTool struct{}

func (unsupportedResponseBuiltInTool) responseBuiltInTool() {}

func TestResponsesBuiltInXSearchExactWireAndRawBoundaries(t *testing.T) {
	function := ResponseTool{Name: "lookup", Parameters: map[string]any{"type": "object"}}
	request := ResponsesBuiltInToolsRequest{
		Request: ResponsesRequest{Model: "spacexai/grok-4.6", Input: ResponseTextInput("news"), Tools: []ResponseTool{function}},
		Tools: []ResponseBuiltInTool{
			&ResponseWebSearchTool{}, ResponseXSearchTool{}, &ResponseXSearchTool{}, ResponseWebSearchTool{}, ResponseXSearchTool{},
		},
	}
	buffered, err := encodeResponsesBuiltInToolsRequest(request, false)
	if err != nil {
		t.Fatal(err)
	}
	wantBuffered := `{"input":"news","model":"spacexai/grok-4.6","stream":false,"tools":[{"name":"lookup","parameters":{"type":"object"},"type":"function"},{"search_context_size":"low","type":"web_search"},{"type":"x_search"},{"type":"x_search"},{"search_context_size":"low","type":"web_search"},{"type":"x_search"}]}`
	if string(buffered) != wantBuffered {
		t.Fatalf("buffered wire\n got: %s\nwant: %s", buffered, wantBuffered)
	}
	streaming, err := encodeResponsesBuiltInToolsRequest(request, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(streaming) != strings.Replace(wantBuffered, `"stream":false`, `"stream":true`, 1) {
		t.Fatalf("streaming wire = %s", streaming)
	}

	minimal, err := encodeResponsesBuiltInToolsRequest(ResponsesBuiltInToolsRequest{Request: validResponsesRequest(), Tools: []ResponseBuiltInTool{ResponseXSearchTool{}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	wantMinimal := `{"input":"hello","model":"provider/model","stream":false,"tools":[{"type":"x_search"}]}`
	if got := string(minimal); got != wantMinimal {
		t.Fatalf("minimal wire\n got: %s\nwant: %s", got, wantMinimal)
	}
	webSearch, err := encodeResponsesBuiltInToolsRequest(ResponsesBuiltInToolsRequest{Request: validResponsesRequest(), Tools: []ResponseBuiltInTool{ResponseWebSearchTool{}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(webSearch), `{"input":"hello","model":"provider/model","stream":false,"tools":[{"search_context_size":"low","type":"web_search"}]}`; got != want {
		t.Fatalf("web_search wire = %s, want %s", got, want)
	}
	omitted, err := encodeResponsesBuiltInToolsRequest(ResponsesBuiltInToolsRequest{Request: validResponsesRequest()}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(omitted), `{"input":"hello","model":"provider/model","stream":false}`; got != want {
		t.Fatalf("omitted tools wire = %s, want %s", got, want)
	}

	rawResult := []byte(` {"output":[{"type":"x_search_call","future":{"opaque":true}}]} `)
	fixture := testserver.New(testserver.Response{Status: http.StatusOK, Body: rawResult})
	defer fixture.Close()
	client, err := NewClient(WithAPIKey("key"), WithPublicBaseURL(fixture.URL))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CreateResponseWithBuiltInTools(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.RawJSON(), rawResult) {
		t.Fatalf("raw result = %q", result.RawJSON())
	}
	if got := string(fixture.Requests()[0].Body); got != wantBuffered {
		t.Fatalf("sent buffered wire = %s", got)
	}

	rawEvent := []byte(`{"type":"response.x_search_call.future","opaque":[1,true]}`)
	var streamBody []byte
	streamClient, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		streamBody, _ = io.ReadAll(req.Body)
		body := "event: future\nid: search-1\ndata: " + string(rawEvent) + "\n\n"
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := streamClient.StreamResponseWithBuiltInTools(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if !stream.Next() {
		t.Fatalf("stream next failed: %v", stream.Err())
	}
	event, ok := stream.Event().(RawResponseEvent)
	if !ok || event.Type != "response.x_search_call.future" || event.Event != "future" || event.ID != "search-1" || !bytes.Equal(event.RawJSON(), rawEvent) {
		t.Fatalf("raw event = %#v / %q", stream.Event(), event.RawJSON())
	}
	if got, want := string(streamBody), strings.Replace(wantBuffered, `"stream":false`, `"stream":true`, 1); got != want {
		t.Fatalf("sent streaming wire\n got: %s\nwant: %s", got, want)
	}
}

func TestResponsesBuiltInValidationBeforeCredentialAndNetwork(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	calls := 0
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("must not send")
	})}))
	if err != nil {
		t.Fatal(err)
	}
	var typedNil *ResponseWebSearchTool
	var typedNilXSearch *ResponseXSearchTool
	tooManyFunctions := make([]ResponseTool, maxResponseMembers)
	for i := range tooManyFunctions {
		tooManyFunctions[i] = ResponseTool{Name: "f", Parameters: map[string]any{}}
	}
	cases := []struct {
		name string
		req  ResponsesBuiltInToolsRequest
		path string
	}{
		{"invalid base request", ResponsesBuiltInToolsRequest{}, `$["model"]`},
		{"nil tool", ResponsesBuiltInToolsRequest{Request: validResponsesRequest(), Tools: []ResponseBuiltInTool{nil}}, `$["tools"][0]`},
		{"typed nil tool", ResponsesBuiltInToolsRequest{Request: validResponsesRequest(), Tools: []ResponseBuiltInTool{typedNil}}, `$["tools"][0]`},
		{"typed nil x_search tool", ResponsesBuiltInToolsRequest{Request: validResponsesRequest(), Tools: []ResponseBuiltInTool{typedNilXSearch}}, `$["tools"][0]`},
		{"unsupported tool", ResponsesBuiltInToolsRequest{Request: validResponsesRequest(), Tools: []ResponseBuiltInTool{unsupportedResponseBuiltInTool{}}}, `$["tools"][0]`},
		{"combined tool limit", ResponsesBuiltInToolsRequest{Request: ResponsesRequest{Model: "p/m", Input: ResponseTextInput("x"), Tools: tooManyFunctions}, Tools: []ResponseBuiltInTool{ResponseXSearchTool{}}}, `$["tools"]`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for _, call := range []func() error{
				func() error {
					_, err := client.CreateResponseWithBuiltInTools(context.Background(), test.req)
					return err
				},
				func() error {
					_, err := client.StreamResponseWithBuiltInTools(context.Background(), test.req)
					return err
				},
			} {
				err := call()
				var validation *ValidationError
				if !errors.As(err, &validation) || validation.Path() != test.path {
					t.Fatalf("error = %T %v, want ValidationError at %s", err, err, test.path)
				}
			}
		})
	}
	for _, call := range []func() error{
		func() error {
			_, err := client.CreateResponseWithBuiltInTools(nil, ResponsesBuiltInToolsRequest{Request: validResponsesRequest()})
			return err
		},
		func() error {
			_, err := client.StreamResponseWithBuiltInTools(nil, ResponsesBuiltInToolsRequest{Request: validResponsesRequest()})
			return err
		},
	} {
		var validation *ValidationError
		if err := call(); !errors.As(err, &validation) || validation.Path() != `$["context"]` {
			t.Fatalf("nil context error = %T %v", err, err)
		}
	}
	if calls != 0 || source.callCount() != 0 {
		t.Fatalf("network calls=%d credential calls=%d", calls, source.callCount())
	}
}
func TestResponsesBuiltInPublicContractAndLegacyCompatibility(t *testing.T) {
	var _ ResponseBuiltInTool = ResponseWebSearchTool{}
	var _ ResponseBuiltInTool = ResponseXSearchTool{}
	var _ ResponseBuiltInTool = (*ResponseXSearchTool)(nil)
	_ = ResponsesBuiltInToolsRequest{ResponsesRequest{}, []ResponseBuiltInTool{ResponseWebSearchTool{}, ResponseXSearchTool{}}}
	var create func(*Client, context.Context, ResponsesBuiltInToolsRequest) (*ResponseResult, error) = (*Client).CreateResponseWithBuiltInTools
	var stream func(*Client, context.Context, ResponsesBuiltInToolsRequest) (*ResponseStream, error) = (*Client).StreamResponseWithBuiltInTools
	_ = create
	_ = stream
	if typ := reflect.TypeOf(ResponseXSearchTool{}); typ.NumField() != 0 || typ.Size() != 0 {
		t.Fatalf("ResponseXSearchTool must remain fieldless, got %d fields and size %d", typ.NumField(), typ.Size())
	}
	wrapper := reflect.TypeOf(ResponsesBuiltInToolsRequest{})
	if wrapper.NumField() != 2 || wrapper.Field(0).Name != "Request" || wrapper.Field(0).Type != reflect.TypeOf(ResponsesRequest{}) || wrapper.Field(1).Name != "Tools" || wrapper.Field(1).Type != reflect.TypeOf([]ResponseBuiltInTool(nil)) {
		t.Fatalf("ResponsesBuiltInToolsRequest contract changed: %v", wrapper)
	}
	var _ func(*ResponseResult) []byte = (*ResponseResult).RawJSON
	var _ func(RawResponseEvent) []byte = RawResponseEvent.RawJSON

	typeOfRequest := reflect.TypeOf(ResponsesRequest{})
	wantFields := []string{"Model", "Input", "MaxOutputTokens", "Temperature", "TopP", "PresencePenalty", "FrequencyPenalty", "Instructions", "Tools", "ToolChoice", "ParallelToolCalls", "AllowedTools", "Reasoning", "Text", "Truncation", "PreviousResponseID", "Store", "Metadata", "Caching", "CacheAnchorItems", "CacheTTL", "PromptCacheKey"}
	if typeOfRequest.NumField() != len(wantFields) {
		t.Fatalf("ResponsesRequest fields = %d, want %d", typeOfRequest.NumField(), len(wantFields))
	}
	for i, name := range wantFields {
		if got := typeOfRequest.Field(i).Name; got != name {
			t.Fatalf("ResponsesRequest field %d = %s, want %s", i, got, name)
		}
	}
	legacy, err := encodeResponsesRequest(validResponsesRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(legacy), `{"input":"hello","model":"provider/model","stream":false}`; got != want {
		t.Fatalf("legacy request = %s, want %s", got, want)
	}
}
