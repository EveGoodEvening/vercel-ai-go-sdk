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
	"testing"
	"time"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/testserver"
)

func validChatRequest() ChatCompletionRequest {
	return ChatCompletionRequest{Model: "provider/model", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("hello")}}}
}

type invalidChatExaText struct{}

func (invalidChatExaText) chatExaText() {}

type invalidChatExaHighlights struct{}

func (invalidChatExaHighlights) chatExaHighlights() {}

type invalidChatExaSubpageTarget struct{}

func (invalidChatExaSubpageTarget) chatExaSubpageTarget() {}

type invalidChatPerplexityQuery struct{}

func (invalidChatPerplexityQuery) chatPerplexityQuery() {}

func TestChatRequestExactJSONAndInventory(t *testing.T) {
	description, safety, sort, schemaName := "weather tool", "user-123", "cost", "legacy"
	strict := true
	temperature, topP, frequency, presence := 0.25, 0.75, -0.5, 0.5
	maxTokens := 123
	request := ChatCompletionRequest{
		Model: "provider/model",
		Messages: []ChatMessage{
			{Role: "system", Content: ChatTextContent("be concise")},
			{Role: "developer", Content: ChatTextContent("return facts")},
			{Role: "user", Content: ChatPartsContent{
				ChatTextPart{Text: "describe"},
				ChatImageURLPart{URL: "https://example.com/image.png"},
				ChatFilePart{Data: "ZmlsZQ==", MediaType: "text/plain", Filename: "note.txt"},
			}},
			{Role: "assistant", Content: ChatTextContent("prior answer")},
		},
		Temperature: &temperature, MaxTokens: &maxTokens, TopP: &topP,
		FrequencyPenalty: &frequency, PresencePenalty: &presence,
		Stop: ChatStopStrings{"END", "STOP"}, SafetyIdentifier: &safety,
		Tools:           []ChatTool{{Name: "weather", Description: &description, Parameters: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}}},
		ToolChoice:      ChatSpecificToolChoice{Name: "weather"},
		ResponseFormat:  ChatJSONSchemaResponseFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict},
		Models:          []string{"provider/backup"},
		ProviderOptions: &ChatProviderOptions{Gateway: ChatGatewayOptions{Order: []string{"provider"}, Models: []string{"provider/model"}, Sort: &sort, ProviderTimeouts: &ChatProviderTimeouts{BYOK: map[string]int{"provider": 1500}}}},
		Provider:        &ChatProvider{Sort: "ttft"},
	}
	got, err := encodeChatCompletionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"frequency_penalty":-0.5,"max_tokens":123,"messages":[{"content":"be concise","role":"system"},{"content":"return facts","role":"developer"},{"content":[{"text":"describe","type":"text"},{"image_url":{"url":"https://example.com/image.png"},"type":"image_url"},{"file":{"data":"ZmlsZQ==","filename":"note.txt","media_type":"text/plain"},"type":"file"}],"role":"user"},{"content":"prior answer","role":"assistant"}],"model":"provider/model","models":["provider/backup"],"presence_penalty":0.5,"provider":{"sort":"ttft"},"providerOptions":{"gateway":{"models":["provider/model"],"order":["provider"],"providerTimeouts":{"byok":{"provider":1500}},"sort":"cost"}},"response_format":{"json_schema":{"description":"weather tool","name":"answer","schema":{"type":"object"},"strict":true},"type":"json_schema"},"safety_identifier":"user-123","stop":["END","STOP"],"stream":false,"temperature":0.25,"tool_choice":{"function":{"name":"weather"},"type":"function"},"tools":[{"function":{"description":"weather tool","name":"weather","parameters":{"properties":{"city":{"type":"string"}},"type":"object"}},"type":"function"}],"top_p":0.75}`
	if string(got) != want {
		t.Fatalf("encoded request\n got: %s\nwant: %s", got, want)
	}
	_ = schemaName
}

func TestChatRequestOmissionAndAlternateForms(t *testing.T) {
	name, description := "legacy", "legacy description"
	for _, test := range []struct {
		name    string
		request ChatCompletionRequest
		want    string
	}{
		{"minimal", validChatRequest(), `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false}`},
		{"empty retained", ChatCompletionRequest{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatPartsContent{}}}, Tools: []ChatTool{}, Models: []string{}}, `{"messages":[{"content":[],"role":"user"}],"model":"p/m","models":[],"stream":false,"tools":[]}`},
		{"string stop auto text", ChatCompletionRequest{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, Stop: ChatStopString("stop"), ToolChoice: ChatToolChoiceAuto, ResponseFormat: ChatResponseFormatText}, `{"messages":[{"content":"x","role":"user"}],"model":"p/m","response_format":{"type":"text"},"stop":"stop","stream":false,"tool_choice":"auto"}`},
		{"legacy json", ChatCompletionRequest{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, ResponseFormat: ChatLegacyJSONResponseFormat{Schema: map[string]any{"type": "object"}, Name: &name, Description: &description}}, `{"messages":[{"content":"x","role":"user"}],"model":"p/m","response_format":{"description":"legacy description","name":"legacy","schema":{"type":"object"},"type":"json"},"stream":false}`},
		{"empty byok retained", ChatCompletionRequest{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, ProviderOptions: &ChatProviderOptions{Gateway: ChatGatewayOptions{ProviderTimeouts: &ChatProviderTimeouts{BYOK: map[string]int{}}}}}, `{"messages":[{"content":"x","role":"user"}],"model":"p/m","providerOptions":{"gateway":{"providerTimeouts":{"byok":{}}}},"stream":false}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := encodeChatCompletionRequest(test.request)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestCreateChatCompletionEndpointHeadersAndOrdinaryResult(t *testing.T) {
	clearCredentialEnvironment(t)
	raw := []byte(" {\n  \"id\":\"chat-1\",\"object\":\"chat.completion\",\"created\":-1,\"model\":\"provider/model\",\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"content\":\"hello\",\"future\":true},\"finish_reason\":\"future_reason\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":3,\"total_tokens\":5,\"future\":null},\"future\":{\"x\":[1]}\n} \t")
	fixture := testserver.New(testserver.Response{Body: raw})
	defer fixture.Close()
	client, err := NewClient(WithAPIKey("secret"), WithPublicBaseURL(fixture.URL+"/v1///"), WithHeaders(http.Header{"X-Custom": {"one", "two"}}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CreateChatCompletion(context.Background(), validChatRequest())
	if err != nil {
		t.Fatal(err)
	}
	requests := fixture.Requests()
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	got := requests[0]
	if got.Method != http.MethodPost || got.URL != "/v1/chat/completions" || string(got.Body) != `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false}` {
		t.Fatalf("request=%#v", got)
	}
	if got.Header.Get("Authorization") != "Bearer secret" || got.Header.Get("Content-Type") != "application/json" || !reflect.DeepEqual(got.Header.Values("X-Custom"), []string{"one", "two"}) {
		t.Fatalf("headers=%#v", got.Header)
	}
	if !result.ID.Present || result.ID.Null || result.ID.Value != "chat-1" || result.Created.Value != -1 || result.Choices.Value[0].FinishReason.Value != "future_reason" || result.Usage.Value.TotalTokens.Value != 5 {
		t.Fatalf("result=%#v", result)
	}
	if !bytes.Equal(result.RawJSON(), raw) {
		t.Fatalf("raw=%q", result.RawJSON())
	}
	copy1 := result.RawJSON()
	copy1[0] ^= 0xff
	if bytes.Equal(copy1, result.RawJSON()) {
		t.Fatal("RawJSON is not defensive")
	}
	if (*ChatCompletionResult)(nil).RawJSON() != nil {
		t.Fatal("nil RawJSON must be nil")
	}
}

func TestChatResultPresenceNullToolCallsAndArguments(t *testing.T) {
	body := []byte(`{"id":null,"object":"chat.completion","choices":[{"index":null,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-1","type":"function","function":{"name":"weather","arguments":"{ \"city\": \"Paris\" }"}}]},"finish_reason":"tool_calls"}],"usage":null}`)
	result, err := decodeChatCompletionResult(body)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ID.Present || !result.ID.Null || result.Model.Present || !result.Usage.Present || !result.Usage.Null {
		t.Fatalf("presence/null=%#v", result)
	}
	choice := result.Choices.Value[0]
	if !choice.Index.Present || !choice.Index.Null || choice.Message.Null {
		t.Fatalf("choice=%#v", choice)
	}
	message := choice.Message.Value
	if !message.Content.Present || !message.Content.Null || !message.ToolCalls.Present || message.ToolCalls.Null {
		t.Fatalf("message=%#v", message)
	}
	call := message.ToolCalls.Value[0]
	if call.Function.Value.Arguments.Value != `{ "city": "Paris" }` {
		t.Fatalf("arguments=%q", call.Function.Value.Arguments.Value)
	}
	empty, err := decodeChatCompletionResult([]byte(`{"choices":[],"usage":{"prompt_tokens":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !empty.Choices.Present || empty.Choices.Null || empty.Choices.Value == nil || len(empty.Choices.Value) != 0 {
		t.Fatalf("empty choices=%#v", empty.Choices)
	}
}

func TestCreateChatCompletionValidationBeforeCredentialAndNetwork(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	calls := 0
	client, err := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("must not send") })}))
	if err != nil {
		t.Fatal(err)
	}
	invalid := []ChatCompletionRequest{{}, {Model: "p/m"}, {Model: "p/m", Messages: []ChatMessage{{Role: "bad", Content: ChatTextContent("x")}}}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: nil}}}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, Temperature: new(math.NaN())}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, TopP: new(1.01)}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, FrequencyPenalty: new(-2.01)}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, ToolChoice: ChatToolChoiceMode("invalid")}}
	for i, r := range invalid {
		_, err := client.CreateChatCompletion(context.Background(), r)
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("case %d error=%T %v", i, err, err)
		}
	}
	if _, err := client.CreateChatCompletion(nil, validChatRequest()); err == nil {
		t.Fatal("nil context accepted")
	}
	if calls != 0 || source.callCount() != 0 {
		t.Fatalf("network=%d credential=%d", calls, source.callCount())
	}
}

func TestChatRequestDocumentedObjectAndTimeoutValidation(t *testing.T) {
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

	nilObject := map[string]any(nil)
	tests := []struct {
		name   string
		mutate func(*ChatCompletionRequest)
		path   string
		reason string
	}{
		{"nil byok", func(r *ChatCompletionRequest) {
			r.ProviderOptions = &ChatProviderOptions{Gateway: ChatGatewayOptions{ProviderTimeouts: &ChatProviderTimeouts{}}}
		}, `$["providerOptions"]["gateway"]["providerTimeouts"]["byok"]`, "must be a non-null object"},
		{"timeout below minimum", func(r *ChatCompletionRequest) {
			r.ProviderOptions = &ChatProviderOptions{Gateway: ChatGatewayOptions{ProviderTimeouts: &ChatProviderTimeouts{BYOK: map[string]int{"p": 999}}}}
		}, `$["providerOptions"]["gateway"]["providerTimeouts"]["byok"]["p"]`, "must be between 1000 and 789000 inclusive"},
		{"timeout above maximum", func(r *ChatCompletionRequest) {
			r.ProviderOptions = &ChatProviderOptions{Gateway: ChatGatewayOptions{ProviderTimeouts: &ChatProviderTimeouts{BYOK: map[string]int{"p": 789001}}}}
		}, `$["providerOptions"]["gateway"]["providerTimeouts"]["byok"]["p"]`, "must be between 1000 and 789000 inclusive"},
		{"tool null", func(r *ChatCompletionRequest) { r.Tools = []ChatTool{{Name: "f", Parameters: nilObject}} }, `$["tools"][0]["function"]["parameters"]`, "must be a non-null object"},
		{"tool scalar", func(r *ChatCompletionRequest) { r.Tools = []ChatTool{{Name: "f", Parameters: "object"}} }, `$["tools"][0]["function"]["parameters"]`, "must be a non-null object"},
		{"tool array", func(r *ChatCompletionRequest) { r.Tools = []ChatTool{{Name: "f", Parameters: []any{}}} }, `$["tools"][0]["function"]["parameters"]`, "must be a non-null object"},
		{"schema null", func(r *ChatCompletionRequest) {
			r.ResponseFormat = ChatJSONSchemaResponseFormat{Name: "f", Schema: nilObject}
		}, `$["response_format"]["json_schema"]["schema"]`, "must be a non-null object"},
		{"schema scalar", func(r *ChatCompletionRequest) {
			r.ResponseFormat = ChatJSONSchemaResponseFormat{Name: "f", Schema: true}
		}, `$["response_format"]["json_schema"]["schema"]`, "must be a non-null object"},
		{"schema array", func(r *ChatCompletionRequest) {
			r.ResponseFormat = ChatJSONSchemaResponseFormat{Name: "f", Schema: []any{}}
		}, `$["response_format"]["json_schema"]["schema"]`, "must be a non-null object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validChatRequest()
			test.mutate(&request)
			_, err := client.CreateChatCompletion(context.Background(), request)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Path() != test.path || validation.Reason() != test.reason {
				t.Fatalf("error=%T %v path=%q reason=%q", err, err, validation.Path(), validation.Reason())
			}
		})
	}
	if calls != 0 || source.callCount() != 0 {
		t.Fatalf("network=%d credential=%d", calls, source.callCount())
	}

	for _, timeout := range []int{1000, 789000} {
		request := validChatRequest()
		request.ProviderOptions = &ChatProviderOptions{Gateway: ChatGatewayOptions{ProviderTimeouts: &ChatProviderTimeouts{BYOK: map[string]int{"p": timeout}}}}
		if err := validateChatCompletionRequest(request); err != nil {
			t.Fatalf("boundary %d rejected: %v", timeout, err)
		}
	}
	request := validChatRequest()
	request.Tools = []ChatTool{{Name: "f", Parameters: map[string]any{"future": []any{map[string]any{"anything": true}}}}}
	request.ResponseFormat = ChatJSONSchemaResponseFormat{Name: "f", Schema: map[string]any{"future": map[string]any{"anything": 1}}}
	request.ProviderOptions = &ChatProviderOptions{Gateway: ChatGatewayOptions{ProviderTimeouts: &ChatProviderTimeouts{BYOK: map[string]int{}}}}
	if err := validateChatCompletionRequest(request); err != nil {
		t.Fatalf("bounded arbitrary objects rejected: %v", err)
	}
}

func TestChatRequestJSONNestingLimit(t *testing.T) {
	nested := func(levels int) any {
		value := any("leaf")
		for range levels - 1 {
			value = []any{value}
		}
		return map[string]any{"value": value}
	}

	level64 := nested(64)
	wrappedLevel64 := any(&level64)
	request := validChatRequest()
	request.Tools = []ChatTool{{Name: "f", Parameters: wrappedLevel64}}
	if _, err := encodeChatCompletionRequest(request); err != nil {
		t.Fatalf("64 JSON container levels rejected: %v", err)
	}
	level65 := nested(65)
	request.Tools[0].Parameters = &level65
	err := validateChatCompletionRequest(request)
	if err == nil || err.Reason() != "maximum depth is 64" {
		t.Fatalf("65 JSON container levels error = %#v", err)
	}
}

func TestChatRequestResourceLimits(t *testing.T) {
	tooDeep := any("leaf")
	for range 65 {
		tooDeep = []any{tooDeep}
	}
	tooManyMessages := make([]ChatMessage, 10001)
	for i := range tooManyMessages {
		tooManyMessages[i] = ChatMessage{Role: "user", Content: ChatTextContent("x")}
	}
	tooManyParts := make(ChatPartsContent, 10001)
	for i := range tooManyParts {
		tooManyParts[i] = ChatTextPart{Text: "x"}
	}
	cases := []ChatCompletionRequest{
		{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent(strings.Repeat("x", (1<<20)+1))}}},
		{Model: "p/m", Messages: tooManyMessages},
		{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: tooManyParts}}},
		{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, Tools: make([]ChatTool, 10001)},
		{Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, Tools: []ChatTool{{Name: "f", Parameters: tooDeep}}},
	}
	for i, r := range cases {
		if err := validateChatCompletionRequest(r); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestChatResultDecoderPolicy(t *testing.T) {
	valid := []string{`{}`, " {\"object\":null,\"choices\":null,\"future\":{\"x\":[1,true,\"s\"]}} \n"}
	for _, body := range valid {
		result, err := decodeChatCompletionResult([]byte(body))
		if err != nil {
			t.Fatalf("valid %q: %v", body, err)
		}
		if string(result.RawJSON()) != body {
			t.Fatalf("raw changed: %q", result.RawJSON())
		}
	}
	deep := strings.Repeat(`{"x":`, 65) + "0" + strings.Repeat("}", 65)
	members := make([]string, 10001)
	for i := range members {
		members[i] = "0"
	}
	invalid := []string{"", `null`, `[]`, `true`, `1`, `{"x":`, `{} {}`, `{"a":1,"a":2}`, `{"nested":{"a":1,"a":2}}`, deep, `{"s":"` + strings.Repeat("x", (1<<20)+1) + `"}`, `{"a":[` + strings.Join(members, ",") + `]}`, `{"object":"future"}`, `{"choices":[{"message":{"role":"future"}}]}`, `{"choices":[{"message":{"tool_calls":[{"type":"future"}]}}]}`, `{"created":1.5}`, `{"usage":[]}`}
	for i, body := range invalid {
		_, err := decodeChatCompletionResult([]byte(body))
		var validation *ResponseValidationError
		if !errors.As(err, &validation) || validation.StatusCode() != http.StatusOK {
			t.Fatalf("case %d error=%T %v", i, err, err)
		}
		raw := validation.RawResponseBody()
		if len(raw) > 0 {
			raw[0] ^= 0xff
			if bytes.Equal(raw, validation.RawResponseBody()) {
				t.Fatalf("case %d raw not defensive", i)
			}
		}
	}
}

func TestCreateChatCompletionSuccessBodyOverflowNon200AndRetry(t *testing.T) {
	for _, test := range []struct {
		name         string
		status       int
		body         []byte
		wantResponse bool
	}{{"overflow", 200, bytes.Repeat([]byte("x"), (1<<20)+1), false}, {"created not success", 201, []byte(`{}`), true}, {"gateway error", 429, []byte(`{"error":{"message":"limited"}}`), true}} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(test.body)), Request: request}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateChatCompletion(context.Background(), validChatRequest())
			if err == nil {
				t.Fatal("expected error")
			}
			if test.status == http.StatusOK {
				validation, ok := err.(*ResponseValidationError)
				if !ok {
					t.Fatalf("error=%T %v", err, err)
				}
				if validation.StatusCode() != http.StatusOK || validation.Path() != "$" || validation.Reason() != "response body exceeds 1 MiB" || !validation.BodyTruncated() {
					t.Fatalf("status=%d path=%q reason=%q truncated=%v", validation.StatusCode(), validation.Path(), validation.Reason(), validation.BodyTruncated())
				}
				raw := validation.RawResponseBody()
				if len(raw) != 1<<20 || !bytes.Equal(raw, test.body[:1<<20]) {
					t.Fatalf("raw length=%d", len(raw))
				}
				raw[0] ^= 0xff
				if bytes.Equal(raw, validation.RawResponseBody()) {
					t.Fatal("raw body is not defensive")
				}
				return
			}
			var responseErr *ResponseError
			if errors.As(err, &responseErr) != test.wantResponse {
				t.Fatalf("ResponseError=%v error=%T %v", errors.As(err, &responseErr), err, err)
			}
		})
	}

	attempts := 0
	client, err := NewClient(WithAPIKey("key"), WithRetryPolicy(RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, Multiplier: 1}), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		status := http.StatusTooManyRequests
		body := `{"error":{"message":"retry"}}`
		if attempts == 2 {
			status = 200
			body = `{}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CreateChatCompletion(context.Background(), validChatRequest()); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
}

func TestCreateChatCompletionSuccessBodyReadAndCloseFailuresAreTransportErrors(t *testing.T) {
	readCause := errors.New("read cause must stay private")
	closeCause := errors.New("close cause must stay private")
	for _, test := range []struct {
		name  string
		body  io.ReadCloser
		cause error
	}{
		{"read", &faultBody{data: []byte(`{}`), readErr: readCause}, readCause},
		{"close", &faultBody{data: []byte(`{}`), closeErr: closeCause}, closeCause},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: test.body, Request: request}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateChatCompletion(context.Background(), validChatRequest())
			transport, ok := err.(*TransportError)
			if !ok || transport.Operation() != "read response body" || !errors.Is(err, test.cause) {
				t.Fatalf("error=%T %v", err, err)
			}
			if got := err.Error(); got != "gateway transport error: read response body" || strings.Contains(got, test.cause.Error()) {
				t.Fatalf("unsafe error text %q", got)
			}
		})
	}
}

func TestCreateChatCompletionCancellationInterruptsSendAndRead(t *testing.T) {
	for _, phase := range []string{"send", "read"} {
		t.Run(phase, func(t *testing.T) {
			started := make(chan struct{})
			var writer *io.PipeWriter
			client, err := NewClient(WithAPIKey("key"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				close(started)
				if phase == "send" {
					<-request.Context().Done()
					return nil, request.Context().Err()
				}
				reader, w := io.Pipe()
				writer = w
				go func() { <-request.Context().Done(); _ = w.CloseWithError(request.Context().Err()) }()
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader, Request: request}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { _, err := client.CreateChatCompletion(ctx, validChatRequest()); done <- err }()
			<-started
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error=%v", err)
				}
			case <-time.After(time.Second):
				if writer != nil {
					_ = writer.Close()
				}
				t.Fatal("cancellation did not stop")
			}
		})
	}
}

func TestCreateChatCompletionDoesNotLeakAcrossSurfaces(t *testing.T) {
	public := testserver.New(testserver.Response{Body: []byte(`{}`)})
	defer public.Close()
	provider := testserver.New(testserver.Response{Body: []byte(`{"answers":{"q":{"type":"boolean","probability":1}}}`)})
	defer provider.Close()
	client, err := NewClient(WithAPIKey("key"), WithPublicBaseURL(public.URL+"/v1"), WithBaseURL(provider.URL+"/v4/ai"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CreateChatCompletion(context.Background(), validChatRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err = client.CreateResponse(context.Background(), validResponsesRequest()); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Evaluate(context.Background(), "provider/model", EvaluationRequest{State: "x", Questions: map[string]Question{"q": BooleanQuestion{Instructions: "yes?"}}}); err != nil {
		t.Fatal(err)
	}
	requests := public.Requests()
	if len(requests) != 2 || requests[0].URL != "/v1/chat/completions" || requests[1].URL != "/v1/responses" {
		t.Fatalf("public requests=%#v", requests)
	}
	if got := provider.Requests()[0].URL; got != "/v4/ai/evaluation-model" {
		t.Fatalf("provider URL=%q", got)
	}
}

func TestChatGatewaySearchMinimumExactJSON(t *testing.T) {
	var nilSubpageTargets ChatExaSubpageTargetStrings
	tests := []struct {
		name string
		tool ChatServerTool
		want string
	}{
		{"exa", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q"}}, `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false,"tools":[{"config":{"query":"q"},"type":"vercel:exa_search"}]}`},
		{"parallel", ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "o"}}, `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false,"tools":[{"config":{"objective":"o"},"type":"vercel:parallel_search"}]}`},
		{"perplexity string", ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("q")}}, `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false,"tools":[{"config":{"query":"q"},"type":"vercel:perplexity_search"}]}`},
		{"perplexity strings", ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryStrings{"q1", "q2"}}}, `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false,"tools":[{"config":{"query":["q1","q2"]},"type":"vercel:perplexity_search"}]}`},
		{"tako", ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q"}}, `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false,"tools":[{"config":{"query":"q"},"type":"vercel:tako_search"}]}`},
		{"exa nil subpage target slice pointer", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: &nilSubpageTargets}}}, `{"messages":[{"content":"hello","role":"user"}],"model":"provider/model","stream":false,"tools":[{"config":{"contents":{"subpage_target":[]},"query":"q"},"type":"vercel:exa_search"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var bodies [][]byte
			bufferedRaw := []byte(` {"choices":[],"gatewayToolCalls":{"opaque":true}} `)
			streamRaw := []byte(`{"object":"chat.completion.chunk","choices":[],"gatewayToolCalls":{"opaque":true}}`)
			client, err := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(request.Body)
				bodies = append(bodies, body)
				if request.Header.Get("Accept") == "text/event-stream" {
					payload := "data: " + string(streamRaw) + "\n\ndata: [DONE]\n\n"
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload)), Request: request}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(bufferedRaw)), Request: request}, nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: validChatRequest(), ServerTools: []ChatServerTool{test.tool}})
			if err != nil {
				t.Fatal(err)
			}
			if string(bodies[0]) != test.want {
				t.Fatalf("body\n got: %s\nwant: %s", bodies[0], test.want)
			}
			if !bytes.Equal(result.RawJSON(), bufferedRaw) {
				t.Fatalf("buffered raw=%q", result.RawJSON())
			}
			stream, err := client.StreamChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: validChatRequest(), ServerTools: []ChatServerTool{test.tool}})
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			if !stream.Next() || !bytes.Equal(stream.Event().RawJSON(), streamRaw) {
				t.Fatalf("stream raw=%q err=%v", stream.Event().RawJSON(), stream.Err())
			}
			if stream.Next() || stream.Err() != nil {
				t.Fatalf("stream terminal err=%v", stream.Err())
			}
			if !bytes.Contains(bodies[1], []byte(`"stream":true`)) {
				t.Fatalf("stream body=%s", bodies[1])
			}
		})
	}
}

func TestChatGatewaySearchAllOptionsAndPresence(t *testing.T) {
	zero, falseValue, trueValue, emptyString := 0, false, true, ""
	emptyStrings := []string(nil)
	emptySections := []ChatExaSection(nil)
	exaType, category, verbosity := ChatExaSearchInstant, ChatExaCategoryFinancialReport, ChatExaVerbosityFull
	parallelMode, recency := ChatParallelModeAgentic, ChatPerplexityRecencyYear
	effort, dataMode, format, webCategory := ChatTakoEffortInstant, ChatTakoDataModeURL, ChatTakoContentFormatJSONRecords, ChatTakoWebCategorySports
	request := ChatServerToolsRequest{Request: validChatRequest(), ServerTools: []ChatServerTool{
		ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "exa", Type: &exaType, NumResults: &zero, Category: &category, UserLocation: &emptyString, IncludeDomains: &emptyStrings, ExcludeDomains: &[]string{}, StartPublishedDate: &emptyString, EndPublishedDate: &emptyString, Contents: &ChatExaContents{Text: ChatExaTextOptions{MaxCharacters: &zero, IncludeHTMLTags: &falseValue, Verbosity: &verbosity, IncludeSections: &emptySections, ExcludeSections: &[]ChatExaSection{}}, Highlights: ChatExaHighlightsOptions{Query: &emptyString, MaxCharacters: &zero}, MaxAgeHours: &zero, LivecrawlTimeout: &zero, Subpages: &zero, SubpageTarget: ChatExaSubpageTargetStrings{}, Extras: &ChatExaExtras{Links: &zero, ImageLinks: &zero}}}},
		ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "unions", Contents: &ChatExaContents{Text: ChatExaTextEnabled(false), Highlights: ChatExaHighlightsEnabled(false), SubpageTarget: ChatExaSubpageTargetString("")}}},
		ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "parallel", SearchQueries: &emptyStrings, Mode: &parallelMode, MaxResults: &zero, SourcePolicy: &ChatParallelSourcePolicy{IncludeDomains: &emptyStrings, ExcludeDomains: &[]string{}, AfterDate: &emptyString}, Excerpts: &ChatParallelExcerpts{MaxCharsPerResult: &zero, MaxCharsTotal: &zero}, FetchPolicy: &ChatParallelFetchPolicy{MaxAgeSeconds: &zero}}},
		ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryStrings{"p"}, MaxResults: &zero, MaxTokensPerPage: &zero, MaxTokens: &zero, Country: &emptyString, SearchDomainFilter: &emptyStrings, SearchLanguageFilter: &[]string{}, SearchAfterDate: &emptyString, SearchBeforeDate: &emptyString, LastUpdatedAfterFilter: &emptyString, LastUpdatedBeforeFilter: &emptyString, SearchRecencyFilter: &recency}},
		ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "tako", Effort: &effort, Sources: &ChatTakoSources{Data: &ChatTakoDataSource{Count: &zero, IncludeContents: &falseValue, Mode: &dataMode, ContentFormat: &format, MaxRows: &zero, NodeIDs: &[]string{"node"}, Strict: &trueValue}, Web: &ChatTakoWebSource{Count: &zero, IncludeContents: &falseValue, Category: &webCategory, IncludeDomains: &emptyStrings, ExcludeDomains: &[]string{}, SnippetMaxChars: &zero, Highlights: &falseValue, ArticleContentMaxChars: &zero, PublishedAfter: &emptyString, PublishedBefore: &emptyString}}, Location: &ChatTakoLocation{Latitude: 0, Longitude: 0}, CountryCode: &emptyString, Locale: &emptyString, Timezone: &emptyString, OutputSettings: &ChatTakoOutputSettings{ImageDarkMode: &falseValue, ForceRefresh: &falseValue}, IncludeRelated: &zero}},
	}}
	var body []byte
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[]}`)), Request: r}, nil
	})}))
	if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	var wantTools any
	if err := json.Unmarshal([]byte(`[
		{"config":{"category":"financial report","contents":{"extras":{"image_links":0,"links":0},"highlights":{"max_characters":0,"query":""},"livecrawl_timeout":0,"max_age_hours":0,"subpage_target":[],"subpages":0,"text":{"exclude_sections":[],"include_html_tags":false,"include_sections":[],"max_characters":0,"verbosity":"full"}},"end_published_date":"","exclude_domains":[],"include_domains":[],"num_results":0,"query":"exa","start_published_date":"","type":"instant","user_location":""},"type":"vercel:exa_search"},
		{"config":{"contents":{"highlights":false,"subpage_target":"","text":false},"query":"unions"},"type":"vercel:exa_search"},
		{"config":{"excerpts":{"max_chars_per_result":0,"max_chars_total":0},"fetch_policy":{"max_age_seconds":0},"max_results":0,"mode":"agentic","objective":"parallel","search_queries":[],"source_policy":{"after_date":"","exclude_domains":[],"include_domains":[]}},"type":"vercel:parallel_search"},
		{"config":{"country":"","last_updated_after_filter":"","last_updated_before_filter":"","max_results":0,"max_tokens":0,"max_tokens_per_page":0,"query":["p"],"search_after_date":"","search_before_date":"","search_domain_filter":[],"search_language_filter":[],"search_recency_filter":"year"},"type":"vercel:perplexity_search"},
		{"config":{"country_code":"","effort":"instant","include_related":0,"locale":"","location":{"latitude":0,"longitude":0},"output_settings":{"force_refresh":false,"image_dark_mode":false},"query":"tako","sources":{"data":{"content_format":"json_records","count":0,"include_contents":false,"max_rows":0,"mode":"url","node_ids":["node"],"strict":true},"web":{"article_content_max_chars":0,"category":"sports","count":0,"exclude_domains":[],"highlights":false,"include_contents":false,"include_domains":[],"published_after":"","published_before":"","snippet_max_chars":0}},"timezone":""},"type":"vercel:tako_search"}
	]`), &wantTools); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded["tools"], wantTools) {
		got, _ := json.Marshal(decoded["tools"])
		want, _ := json.Marshal(wantTools)
		t.Fatalf("tools\n got: %s\nwant: %s", got, want)
	}
}

func TestChatGatewaySearchEnumValidation(t *testing.T) {
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[]}`)), Request: r}, nil
	})}))
	request := ChatServerToolsRequest{Request: validChatRequest()}
	documentedExaType := ChatExaSearchAuto
	documentedExaCategory := ChatExaCategoryCompany
	documentedExaVerbosity := ChatExaVerbosityCompact
	documentedParallelMode := ChatParallelModeOneShot
	documentedPerplexityRecency := ChatPerplexityRecencyDay
	documentedTakoEffort := ChatTakoEffortDeep
	documentedTakoDataMode := ChatTakoDataModeInline
	documentedTakoContentFormat := ChatTakoContentFormatCardJSON
	documentedTakoWebCategory := ChatTakoWebCategoryFinance
	for _, enumTool := range []ChatServerTool{
		ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Type: &documentedExaType, Category: &documentedExaCategory, Contents: &ChatExaContents{Text: ChatExaTextOptions{Verbosity: &documentedExaVerbosity, IncludeSections: &[]ChatExaSection{ChatExaSectionHeader, ChatExaSectionNavigation, ChatExaSectionBanner, ChatExaSectionBody, ChatExaSectionSidebar, ChatExaSectionFooter, ChatExaSectionMetadata}}}}},
		ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "o", Mode: &documentedParallelMode}},
		ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("q"), SearchRecencyFilter: &documentedPerplexityRecency}},
		ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Effort: &documentedTakoEffort, Sources: &ChatTakoSources{Data: &ChatTakoDataSource{Mode: &documentedTakoDataMode, ContentFormat: &documentedTakoContentFormat}, Web: &ChatTakoWebSource{Category: &documentedTakoWebCategory}}}},
	} {
		request.ServerTools = []ChatServerTool{enumTool}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("documented enum rejected: %v", err)
		}
	}
	for _, value := range []ChatExaSearchType{ChatExaSearchAuto, ChatExaSearchFast, ChatExaSearchInstant} {
		request.ServerTools = []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Type: &value}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("exa type %q: %v", value, err)
		}
	}
	for _, value := range []ChatExaCategory{ChatExaCategoryCompany, ChatExaCategoryPeople, ChatExaCategoryResearchPaper, ChatExaCategoryNews, ChatExaCategoryPersonalSite, ChatExaCategoryFinancialReport} {
		request.ServerTools = []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Category: &value}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("exa category %q: %v", value, err)
		}
	}
	for _, value := range []ChatExaVerbosity{ChatExaVerbosityCompact, ChatExaVerbosityStandard, ChatExaVerbosityFull} {
		request.ServerTools = []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: ChatExaTextOptions{Verbosity: &value}}}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("exa verbosity %q: %v", value, err)
		}
	}
	for _, value := range []ChatParallelMode{ChatParallelModeOneShot, ChatParallelModeAgentic} {
		request.ServerTools = []ChatServerTool{ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "o", Mode: &value}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("parallel mode %q: %v", value, err)
		}
	}
	for _, value := range []ChatPerplexityRecency{ChatPerplexityRecencyDay, ChatPerplexityRecencyWeek, ChatPerplexityRecencyMonth, ChatPerplexityRecencyYear} {
		request.ServerTools = []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("q"), SearchRecencyFilter: &value}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("perplexity recency %q: %v", value, err)
		}
	}
	for _, value := range []ChatTakoEffort{ChatTakoEffortDeep, ChatTakoEffortFast, ChatTakoEffortInstant} {
		request.ServerTools = []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Effort: &value}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("tako effort %q: %v", value, err)
		}
	}
	for _, value := range []ChatTakoDataMode{ChatTakoDataModeInline, ChatTakoDataModeURL} {
		request.ServerTools = []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Data: &ChatTakoDataSource{Mode: &value}}}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("tako data mode %q: %v", value, err)
		}
	}
	for _, value := range []ChatTakoContentFormat{ChatTakoContentFormatCardJSON, ChatTakoContentFormatCSV, ChatTakoContentFormatJSONCompact, ChatTakoContentFormatJSONRecords} {
		request.ServerTools = []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Data: &ChatTakoDataSource{ContentFormat: &value}}}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("tako format %q: %v", value, err)
		}
	}
	for _, value := range []ChatTakoWebCategory{ChatTakoWebCategoryFinance, ChatTakoWebCategoryNews, ChatTakoWebCategorySports} {
		request.ServerTools = []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Web: &ChatTakoWebSource{Category: &value}}}}}
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err != nil {
			t.Fatalf("tako category %q: %v", value, err)
		}
	}
}

func TestChatGatewaySearchPointerUnionVariantsMatchValues(t *testing.T) {
	var body []byte
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[]}`)), Request: r}, nil
	})}))
	encodeTools := func(t *testing.T, tools []ChatServerTool) any {
		t.Helper()
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: validChatRequest(), ServerTools: tools}); err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded["tools"]
	}

	exaTool := ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "exa"}}
	parallelTool := ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "parallel"}}
	perplexityTool := ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("perplexity")}}
	takoTool := ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "tako"}}
	textEnabled := ChatExaTextEnabled(false)
	textOptions := ChatExaTextOptions{}
	highlightsEnabled := ChatExaHighlightsEnabled(false)
	highlightsOptions := ChatExaHighlightsOptions{}
	subpageString := ChatExaSubpageTargetString("")
	subpageStrings := ChatExaSubpageTargetStrings{}
	queryString := ChatPerplexityQueryString("query")
	queryStrings := ChatPerplexityQueryStrings{"query"}
	tests := []struct {
		name    string
		value   ChatServerTool
		pointer ChatServerTool
	}{
		{"exa tool", exaTool, &exaTool},
		{"parallel tool", parallelTool, &parallelTool},
		{"perplexity tool", perplexityTool, &perplexityTool},
		{"tako tool", takoTool, &takoTool},
		{"exa text enabled", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: textEnabled}}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: &textEnabled}}}},
		{"exa text options", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: textOptions}}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: &textOptions}}}},
		{"exa highlights enabled", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Highlights: highlightsEnabled}}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Highlights: &highlightsEnabled}}}},
		{"exa highlights options", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Highlights: highlightsOptions}}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Highlights: &highlightsOptions}}}},
		{"exa subpage string", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: subpageString}}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: &subpageString}}}},
		{"exa subpage strings", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: subpageStrings}}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: &subpageStrings}}}},
		{"perplexity query string", ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: queryString}}, ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: &queryString}}},
		{"perplexity query strings", ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: queryStrings}}, ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: &queryStrings}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := encodeTools(t, []ChatServerTool{test.value})
			got := encodeTools(t, []ChatServerTool{test.pointer})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pointer tools = %#v, value tools = %#v", got, want)
			}
		})
	}
}

func TestChatGatewaySearchOrderingChoicesOmissionAndRawCompatibility(t *testing.T) {
	bufferedRaw := []byte(` {"choices":[],"gatewayToolCalls":[{"future":true}],"cost":{"future":1}} `)
	var bodies [][]byte
	responses := 0
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		responses++
		if r.Header.Get("Accept") == "text/event-stream" {
			raw := "data: {\"object\":\"chat.completion.chunk\",\"choices\":[],\"gatewayToolCalls\":[{\"future\":true}]}\n\ndata: [DONE]\n\n"
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(raw)), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(bufferedRaw)), Request: r}, nil
	})}))
	description := "ordinary"
	base := validChatRequest()
	base.Tools = []ChatTool{{Name: "first", Description: &description, Parameters: map[string]any{"type": "object"}}, {Name: "second", Parameters: map[string]any{}}}
	base.ToolChoice = ChatToolChoiceRequired
	server := []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "one"}}, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "two"}}, ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "three"}}}
	result, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: base, ServerTools: server})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.RawJSON(), bufferedRaw) {
		t.Fatalf("raw=%q", result.RawJSON())
	}
	var decoded map[string]any
	if err := json.Unmarshal(bodies[0], &decoded); err != nil {
		t.Fatal(err)
	}
	tools := decoded["tools"].([]any)
	gotTypes := make([]string, len(tools))
	for i, tool := range tools {
		m := tool.(map[string]any)
		gotTypes[i], _ = m["type"].(string)
		if gotTypes[i] == "function" {
			gotTypes[i] = m["function"].(map[string]any)["name"].(string)
		}
	}
	if !reflect.DeepEqual(gotTypes, []string{"first", "second", "vercel:tako_search", "vercel:exa_search", "vercel:tako_search"}) {
		t.Fatalf("order=%v", gotTypes)
	}
	stream, err := client.StreamChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "o"}}}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if !stream.Next() || !bytes.Contains(stream.Event().RawJSON(), []byte(`"gatewayToolCalls"`)) {
		t.Fatalf("event=%#v err=%v", stream.Event(), stream.Err())
	}
	if stream.Next() || stream.Err() != nil {
		t.Fatalf("terminal err=%v", stream.Err())
	}
	for _, tc := range []struct {
		name      string
		request   ChatServerToolsRequest
		wantTools bool
	}{
		{"both nil", ChatServerToolsRequest{Request: validChatRequest()}, false},
		{"ordinary empty retained", func() ChatServerToolsRequest {
			r := validChatRequest()
			r.Tools = []ChatTool{}
			return ChatServerToolsRequest{Request: r}
		}(), true},
		{"server only", ChatServerToolsRequest{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q"}}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(bodies)
			if _, err := client.CreateChatCompletionWithServerTools(context.Background(), tc.request); err != nil {
				t.Fatal(err)
			}
			has := bytes.Contains(bodies[before], []byte(`"tools"`))
			if has != tc.wantTools {
				t.Fatalf("body=%s", bodies[before])
			}
		})
	}
	_ = responses
}

func TestChatGatewaySearchConditionalCollisionsAndChoices(t *testing.T) {
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[]}`)), Request: r}, nil
	})}))
	identifiers := []struct {
		short, full string
		tool        ChatServerTool
	}{{"exa_search", "vercel:exa_search", ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q"}}}, {"parallel_search", "vercel:parallel_search", ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "o"}}}, {"perplexity_search", "vercel:perplexity_search", ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("q")}}}, {"tako_search", "vercel:tako_search", ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q"}}}}
	for _, id := range identifiers {
		t.Run(id.short, func(t *testing.T) {
			ordinary := validChatRequest()
			ordinary.Tools = []ChatTool{{Name: id.short, Parameters: map[string]any{}}}
			ordinary.ToolChoice = ChatSpecificToolChoice{Name: id.short}
			if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: ordinary}); err != nil {
				t.Fatalf("absent server rejected: %v", err)
			}
			for _, choice := range []string{id.short, id.full} {
				r := validChatRequest()
				r.ToolChoice = ChatSpecificToolChoice{Name: choice}
				if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: r, ServerTools: []ChatServerTool{id.tool}}); err == nil {
					t.Fatalf("choice %q accepted", choice)
				}
			}
			if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: ordinary, ServerTools: []ChatServerTool{id.tool}}); err == nil {
				t.Fatal("collision accepted")
			}
			different := validChatRequest()
			different.Tools = []ChatTool{{Name: "ordinary", Parameters: map[string]any{}}}
			different.ToolChoice = ChatSpecificToolChoice{Name: "ordinary"}
			if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: different, ServerTools: []ChatServerTool{id.tool}}); err != nil {
				t.Fatalf("different choice rejected: %v", err)
			}
		})
	}
	for _, mode := range []ChatToolChoiceMode{ChatToolChoiceAuto, ChatToolChoiceRequired, ChatToolChoiceNone} {
		for _, tools := range [][]ChatServerTool{nil, {ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q"}}}} {
			r := validChatRequest()
			r.ToolChoice = mode
			if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: r, ServerTools: tools}); err != nil {
				t.Fatalf("mode %q tools=%d: %v", mode, len(tools), err)
			}
		}
	}
}

func TestChatGatewaySearchInvalidBeforeCredentialOrDispatch(t *testing.T) {
	clearCredentialEnvironment(t)
	source := &transportTokenSource{token: "token"}
	calls := 0
	client, _ := NewClient(WithOIDCTokenSource(source), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("must not send") })}))
	var nilTool *ChatExaSearchTool
	var nilText *ChatExaTextOptions
	var nilHighlights *ChatExaHighlightsOptions
	var nilTarget *ChatExaSubpageTargetStrings
	var nilQuery *ChatPerplexityQueryStrings
	badExaType := ChatExaSearchType("bad")
	badCategory := ChatExaCategory("bad")
	badVerbosity := ChatExaVerbosity("bad")
	badParallel := ChatParallelMode("bad")
	badRecency := ChatPerplexityRecency("bad")
	badEffort := ChatTakoEffort("bad")
	badMode := ChatTakoDataMode("bad")
	badFormat := ChatTakoContentFormat("bad")
	badWeb := ChatTakoWebCategory("bad")
	strict := true
	tooLong := strings.Repeat("x", (1<<20)+1)
	cases := []ChatServerToolsRequest{
		{Request: validChatRequest(), ServerTools: []ChatServerTool{nil}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{nilTool}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Type: &badExaType}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Category: &badCategory}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: nilText}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Highlights: nilHighlights}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: nilTarget}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: ChatExaTextOptions{Verbosity: &badVerbosity}}}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Text: invalidChatExaText{}}}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{Highlights: invalidChatExaHighlights{}}}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q", Contents: &ChatExaContents{SubpageTarget: invalidChatExaSubpageTarget{}}}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatParallelSearchTool{Config: ChatParallelSearchConfig{}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: "o", Mode: &badParallel}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: nilQuery}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("")}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryStrings{}}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString("q"), SearchRecencyFilter: &badRecency}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatExaSearchTool{Config: ChatExaSearchConfig{Query: tooLong}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatParallelSearchTool{Config: ChatParallelSearchConfig{Objective: tooLong}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: ChatPerplexityQueryString(tooLong)}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatPerplexitySearchTool{Config: ChatPerplexitySearchConfig{Query: invalidChatPerplexityQuery{}}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: tooLong}}}},
		{Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Effort: &badEffort}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Data: &ChatTakoDataSource{Mode: &badMode}}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Data: &ChatTakoDataSource{ContentFormat: &badFormat}}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Web: &ChatTakoWebSource{Category: &badWeb}}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Sources: &ChatTakoSources{Data: &ChatTakoDataSource{Strict: &strict}}}}}}, {Request: validChatRequest(), ServerTools: []ChatServerTool{ChatTakoSearchTool{Config: ChatTakoSearchConfig{Query: "q", Location: &ChatTakoLocation{Latitude: math.Inf(1)}}}}},
	}
	for i, request := range cases {
		if _, err := client.CreateChatCompletionWithServerTools(context.Background(), request); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if _, err := client.StreamChatCompletionWithServerTools(nil, ChatServerToolsRequest{Request: validChatRequest()}); err == nil {
		t.Error("nil context accepted")
	}
	if calls != 0 || source.callCount() != 0 {
		t.Fatalf("network=%d credential=%d", calls, source.callCount())
	}
}

func TestChatGatewaySearchCombinedToolLimit(t *testing.T) {
	client, _ := NewClient(WithAPIKey("secret"), WithHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[]}`)), Request: r}, nil
	})}))
	server := make([]ChatServerTool, 9999)
	for i := range server {
		server[i] = ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q"}}
	}
	r := validChatRequest()
	r.Tools = []ChatTool{{Name: "ordinary", Parameters: map[string]any{}}}
	if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: r, ServerTools: server}); err != nil {
		t.Fatalf("10000 tools rejected: %v", err)
	}
	server = append(server, ChatExaSearchTool{Config: ChatExaSearchConfig{Query: "q"}})
	if _, err := client.CreateChatCompletionWithServerTools(context.Background(), ChatServerToolsRequest{Request: r, ServerTools: server}); err == nil {
		t.Fatal("10001 tools accepted")
	}
}
