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

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/testserver"
)

func validChatRequest() ChatCompletionRequest {
	return ChatCompletionRequest{Model: "provider/model", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("hello")}}}
}

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
	invalid := []ChatCompletionRequest{{}, {Model: "p/m"}, {Model: "p/m", Messages: []ChatMessage{{Role: "bad", Content: ChatTextContent("x")}}}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: nil}}}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, Temperature: new(math.NaN())}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, TopP: new(1.01)}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, FrequencyPenalty: new(-2.01)}, {Model: "p/m", Messages: []ChatMessage{{Role: "user", Content: ChatTextContent("x")}}, ToolChoice: ChatToolChoiceMode("required")}}
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
