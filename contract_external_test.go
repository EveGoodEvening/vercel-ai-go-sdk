package gateway_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

type tokenSource struct{}

func (tokenSource) Token(context.Context) (string, error) { return "token", nil }

func compilePublicContract() {
	var _ *gateway.Client
	var _ gateway.Option
	var _ gateway.TokenSource = tokenSource{}
	var _ func(...gateway.Option) (*gateway.Client, error) = gateway.NewClient
	var _ func(*gateway.Client, context.Context, string, gateway.EvaluationRequest) (*gateway.EvaluationResult, error) = (*gateway.Client).Evaluate
	var _ func(*gateway.Client, context.Context, gateway.ResponsesRequest) (*gateway.ResponseResult, error) = (*gateway.Client).CreateResponse
	var _ func(*gateway.Client, context.Context, gateway.ResponsesRequest) (*gateway.ResponseStream, error) = (*gateway.Client).StreamResponse
	var _ func(*gateway.Client, context.Context, gateway.ResponsesBuiltInToolsRequest) (*gateway.ResponseResult, error) = (*gateway.Client).CreateResponseWithBuiltInTools
	var _ func(*gateway.Client, context.Context, gateway.ResponsesBuiltInToolsRequest) (*gateway.ResponseStream, error) = (*gateway.Client).StreamResponseWithBuiltInTools
	var _ func(string) gateway.Option = gateway.WithAPIKey
	var _ func(*gateway.Client, context.Context, gateway.ChatCompletionRequest) (*gateway.ChatCompletionResult, error) = (*gateway.Client).CreateChatCompletion
	var _ func(*gateway.Client, context.Context, gateway.ChatCompletionRequest) (*gateway.ChatCompletionStream, error) = (*gateway.Client).StreamChatCompletion
	var _ func(*gateway.Client, context.Context, gateway.ChatServerToolsRequest) (*gateway.ChatCompletionResult, error) = (*gateway.Client).CreateChatCompletionWithServerTools
	var _ func(*gateway.Client, context.Context, gateway.ChatServerToolsRequest) (*gateway.ChatCompletionStream, error) = (*gateway.Client).StreamChatCompletionWithServerTools
	var _ func(string) gateway.Option = gateway.WithOIDCToken
	var _ func(gateway.TokenSource) gateway.Option = gateway.WithOIDCTokenSource
	var _ func(string) gateway.Option = gateway.WithBaseURL
	var _ func(string) gateway.Option = gateway.WithPublicBaseURL
	var _ func(*http.Client) gateway.Option = gateway.WithHTTPClient
	var _ func(string) gateway.Option = gateway.WithTeam
	var _ func(http.Header) gateway.Option = gateway.WithHeaders
	var _ func(gateway.RetryPolicy) gateway.Option = gateway.WithRetryPolicy
	description := "description"
	strict := true
	_ = gateway.ResponsesRequest{
		Model: "provider/model",
		Input: gateway.ResponseItemsInput{
			gateway.ResponseMessage{Role: "user", Content: "hello"},
			gateway.ResponseFunctionCall{ID: "item", CallID: "call", Name: "tool", Arguments: `{}`},
			gateway.ResponseFunctionCallOutput{CallID: "call", Output: `{}`},
		},
		MaxOutputTokens: new(128), Temperature: new(0.5), TopP: new(0.9),
		PresencePenalty: new(0.1), FrequencyPenalty: new(-0.1), Instructions: new("instruction"),
		Tools:      []gateway.ResponseTool{{Name: "tool", Description: &description, Parameters: map[string]any{"type": "object"}, Strict: &strict}},
		ToolChoice: gateway.ResponseSpecificToolChoice{Name: "tool"}, ParallelToolCalls: new(true),
		AllowedTools: []string{"tool"}, Reasoning: &gateway.ResponseReasoning{Effort: "high", Summary: new("auto")},
		Text:       &gateway.ResponseText{Format: gateway.ResponseJSONSchemaFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict}},
		Truncation: new("disabled"), PreviousResponseID: new("response"), Store: new(true),
		Metadata: map[string]string{"key": "value"}, Caching: new("auto"), CacheAnchorItems: new(1), CacheTTL: new("5m"), PromptCacheKey: new("cache"),
	}
	_ = gateway.ResponsesRequest{Model: "provider/model", Input: gateway.ResponseTextInput("hello"), ToolChoice: gateway.ResponseToolChoiceAuto, Text: &gateway.ResponseText{Format: gateway.ResponseTextFormatText}}
	var builtInTool gateway.ResponseBuiltInTool = gateway.ResponseWebSearchTool{}
	_ = gateway.ResponsesBuiltInToolsRequest{
		Request: gateway.ResponsesRequest{Model: "provider/model", Input: gateway.ResponseTextInput("hello")},
		Tools:   []gateway.ResponseBuiltInTool{builtInTool},
	}
	// Keep an external unkeyed literal as an exact wrapper-field compile check.
	_ = gateway.ResponsesBuiltInToolsRequest{gateway.ResponsesRequest{}, []gateway.ResponseBuiltInTool{gateway.ResponseWebSearchTool{}}}
	_ = []gateway.ResponseToolChoiceMode{gateway.ResponseToolChoiceAuto, gateway.ResponseToolChoiceRequired, gateway.ResponseToolChoiceNone}
	description = "description"
	strict = true
	safetyIdentifier := "user-hash"
	maxTokens := 128
	temperature := 0.5
	topP := 0.9
	frequencyPenalty := -0.1
	presencePenalty := 0.1
	sort := "price"
	// Keep an external unkeyed literal as a source-compatibility compile check.
	_ = gateway.ChatCompletionRequest{"provider/model", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}
	_ = gateway.ChatCompletionRequest{
		Model: "provider/model",
		Messages: []gateway.ChatMessage{
			{Role: "system", Content: gateway.ChatTextContent("be concise")},
			{Role: "user", Content: gateway.ChatPartsContent{
				gateway.ChatTextPart{Text: "describe"},
				gateway.ChatImageURLPart{URL: "https://example.com/image.png"},
				gateway.ChatFilePart{Data: "ZmlsZQ==", MediaType: "text/plain", Filename: "file.txt"},
			}},
		},
		Temperature: &temperature, MaxTokens: &maxTokens, TopP: &topP,
		FrequencyPenalty: &frequencyPenalty, PresencePenalty: &presencePenalty,
		Stop: gateway.ChatStopStrings{"stop"}, SafetyIdentifier: &safetyIdentifier,
		Tools:           []gateway.ChatTool{{Name: "tool", Description: &description, Parameters: map[string]any{"type": "object"}}},
		ToolChoice:      gateway.ChatSpecificToolChoice{Name: "tool"},
		ResponseFormat:  gateway.ChatJSONSchemaResponseFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict},
		Models:          []string{"provider/fallback"},
		ProviderOptions: &gateway.ChatProviderOptions{Gateway: gateway.ChatGatewayOptions{Order: []string{"provider"}, Models: []string{"provider/model"}, Sort: &sort, ProviderTimeouts: &gateway.ChatProviderTimeouts{BYOK: map[string]int{"provider": 1000}}}},
		Provider:        &gateway.ChatProvider{Sort: "latency"},
	}
	_ = gateway.ChatCompletionRequest{Model: "provider/model", Messages: []gateway.ChatMessage{{Role: "user", Content: gateway.ChatTextContent("hello")}}, Stop: gateway.ChatStopString("stop"), ToolChoice: gateway.ChatToolChoiceAuto, ResponseFormat: gateway.ChatResponseFormatText}
	_ = []gateway.ChatToolChoiceMode{gateway.ChatToolChoiceAuto, gateway.ChatToolChoiceNone}
	_ = []gateway.ChatResponseFormatType{gateway.ChatResponseFormatText, gateway.ChatResponseFormatJSON}
	_ = gateway.ChatLegacyJSONResponseFormat{Schema: map[string]any{"type": "object"}, Name: &description, Description: &description}
	var _ gateway.ChatToolChoiceMode = gateway.ChatToolChoiceRequired
	_ = [3]gateway.ChatToolChoiceMode{gateway.ChatToolChoiceAuto, gateway.ChatToolChoiceNone, gateway.ChatToolChoiceRequired}
	if string(gateway.ChatToolChoiceRequired) != "required" {
		panic("ChatToolChoiceRequired changed")
	}
	exaType, exaCategory, exaVerbosity := gateway.ChatExaSearchFast, gateway.ChatExaCategoryResearchPaper, gateway.ChatExaVerbosityStandard
	exaSections := []gateway.ChatExaSection{gateway.ChatExaSectionHeader, gateway.ChatExaSectionNavigation, gateway.ChatExaSectionBanner, gateway.ChatExaSectionBody, gateway.ChatExaSectionSidebar, gateway.ChatExaSectionFooter, gateway.ChatExaSectionMetadata}
	maxCharacters, includeHTML, links := 100, false, 2
	includeDomains := []string{"example.com"}
	_ = gateway.ChatServerToolsRequest{Request: gateway.ChatCompletionRequest{Model: "provider/model"}, ServerTools: []gateway.ChatServerTool{
		gateway.ChatExaSearchTool{Config: gateway.ChatExaSearchConfig{Query: "query", Type: &exaType, NumResults: &maxCharacters, Category: &exaCategory, UserLocation: new("US"), IncludeDomains: &includeDomains, ExcludeDomains: &[]string{}, StartPublishedDate: new("2026-01-01"), EndPublishedDate: new("2026-09-21"), Contents: &gateway.ChatExaContents{Text: gateway.ChatExaTextEnabled(true), Highlights: gateway.ChatExaHighlightsEnabled(false), MaxAgeHours: new(0), LivecrawlTimeout: new(0), Subpages: new(0), SubpageTarget: gateway.ChatExaSubpageTargetString("docs"), Extras: &gateway.ChatExaExtras{Links: &links, ImageLinks: new(0)}}}},
		gateway.ChatExaSearchTool{Config: gateway.ChatExaSearchConfig{Query: "query", Contents: &gateway.ChatExaContents{Text: gateway.ChatExaTextOptions{MaxCharacters: &maxCharacters, IncludeHTMLTags: &includeHTML, Verbosity: &exaVerbosity, IncludeSections: &exaSections, ExcludeSections: &[]gateway.ChatExaSection{}}, Highlights: gateway.ChatExaHighlightsOptions{Query: new("focus"), MaxCharacters: &maxCharacters}, SubpageTarget: gateway.ChatExaSubpageTargetStrings{"docs", "blog"}}}},
		gateway.ChatParallelSearchTool{Config: gateway.ChatParallelSearchConfig{Objective: "objective", SearchQueries: &[]string{"one"}, Mode: new(gateway.ChatParallelModeAgentic), MaxResults: new(0), SourcePolicy: &gateway.ChatParallelSourcePolicy{IncludeDomains: &includeDomains, ExcludeDomains: &[]string{}, AfterDate: new("2026-01-01")}, Excerpts: &gateway.ChatParallelExcerpts{MaxCharsPerResult: new(0), MaxCharsTotal: new(0)}, FetchPolicy: &gateway.ChatParallelFetchPolicy{MaxAgeSeconds: new(0)}}},
		gateway.ChatPerplexitySearchTool{Config: gateway.ChatPerplexitySearchConfig{Query: gateway.ChatPerplexityQueryString("query"), MaxResults: new(0), MaxTokensPerPage: new(0), MaxTokens: new(0), Country: new("US"), SearchDomainFilter: &includeDomains, SearchLanguageFilter: &[]string{}, SearchAfterDate: new("2026-01-01"), SearchBeforeDate: new("2026-09-21"), LastUpdatedAfterFilter: new("2026-01-01"), LastUpdatedBeforeFilter: new("2026-09-21"), SearchRecencyFilter: new(gateway.ChatPerplexityRecencyWeek)}},
		gateway.ChatPerplexitySearchTool{Config: gateway.ChatPerplexitySearchConfig{Query: gateway.ChatPerplexityQueryStrings{"one", "two"}}},
		gateway.ChatTakoSearchTool{Config: gateway.ChatTakoSearchConfig{Query: "query", Effort: new(gateway.ChatTakoEffortDeep), Sources: &gateway.ChatTakoSources{Data: &gateway.ChatTakoDataSource{Count: new(0), IncludeContents: new(false), Mode: new(gateway.ChatTakoDataModeInline), ContentFormat: new(gateway.ChatTakoContentFormatJSONCompact), MaxRows: new(0), NodeIDs: &[]string{"node"}, Strict: new(true)}, Web: &gateway.ChatTakoWebSource{Count: new(0), IncludeContents: new(false), Category: new(gateway.ChatTakoWebCategoryNews), IncludeDomains: &includeDomains, ExcludeDomains: &[]string{}, SnippetMaxChars: new(0), Highlights: new(false), ArticleContentMaxChars: new(0), PublishedAfter: new("2026-01-01"), PublishedBefore: new("2026-09-21")}}, Location: &gateway.ChatTakoLocation{Latitude: 1, Longitude: 2}, CountryCode: new("US"), Locale: new("en"), Timezone: new("UTC"), OutputSettings: &gateway.ChatTakoOutputSettings{ImageDarkMode: new(false), ForceRefresh: new(false)}, IncludeRelated: new(0)}},
	}}
	_ = []gateway.ChatExaSearchType{gateway.ChatExaSearchAuto, gateway.ChatExaSearchFast, gateway.ChatExaSearchInstant}
	_ = []gateway.ChatExaCategory{gateway.ChatExaCategoryCompany, gateway.ChatExaCategoryPeople, gateway.ChatExaCategoryResearchPaper, gateway.ChatExaCategoryNews, gateway.ChatExaCategoryPersonalSite, gateway.ChatExaCategoryFinancialReport}
	_ = []gateway.ChatExaVerbosity{gateway.ChatExaVerbosityCompact, gateway.ChatExaVerbosityStandard, gateway.ChatExaVerbosityFull}
	_ = []gateway.ChatParallelMode{gateway.ChatParallelModeOneShot, gateway.ChatParallelModeAgentic}
	_ = []gateway.ChatPerplexityRecency{gateway.ChatPerplexityRecencyDay, gateway.ChatPerplexityRecencyWeek, gateway.ChatPerplexityRecencyMonth, gateway.ChatPerplexityRecencyYear}
	_ = []gateway.ChatTakoEffort{gateway.ChatTakoEffortDeep, gateway.ChatTakoEffortFast, gateway.ChatTakoEffortInstant}
	_ = []gateway.ChatTakoDataMode{gateway.ChatTakoDataModeInline, gateway.ChatTakoDataModeURL}
	_ = []gateway.ChatTakoContentFormat{gateway.ChatTakoContentFormatCardJSON, gateway.ChatTakoContentFormatCSV, gateway.ChatTakoContentFormatJSONCompact, gateway.ChatTakoContentFormatJSONRecords}
	_ = []gateway.ChatTakoWebCategory{gateway.ChatTakoWebCategoryFinance, gateway.ChatTakoWebCategoryNews, gateway.ChatTakoWebCategorySports}
	_ = gateway.JSONField[string]{Present: true, Value: "value"}
	var chatResult *gateway.ChatCompletionResult
	_ = chatResult.RawJSON()
	_ = gateway.ChatChoice{}
	_ = gateway.ChatAssistantMessage{}
	_ = gateway.ChatToolCall{}
	_ = gateway.ChatFunctionCall{}
	_ = gateway.ChatUsage{}
	var chatStream *gateway.ChatCompletionStream
	var chatChunk *gateway.ChatCompletionChunk
	_ = chatStream.Next()
	chatChunk = chatStream.Event()
	_, _ = chatStream.Err(), chatStream.Close()
	_ = chatChunk.Object
	_ = []gateway.ChatCompletionChunkChoice{{Delta: gateway.ChatCompletionChunkDelta{Content: "text"}}}
	_ = chatChunk.RawJSON()
	_ = []gateway.ResponseTextFormatType{gateway.ResponseTextFormatText, gateway.ResponseTextFormatJSONObject}
	var responseResult *gateway.ResponseResult
	_ = responseResult.RawJSON()
	var responseStream *gateway.ResponseStream
	var responseEvent gateway.ResponseEvent
	_ = responseStream.Next()
	responseEvent = responseStream.Event()
	_, _ = responseStream.Err(), responseStream.Close()
	_ = gateway.ResponseOutputTextDeltaEvent{Type: "response.output_text.delta", Event: "event", ID: "id", Delta: "text"}
	rawResponseEvent := gateway.RawResponseEvent{Type: "future", Event: "event", ID: "id"}
	_ = rawResponseEvent.RawJSON()
	_ = responseEvent

	probabilityDecimals := 2
	scoreDecimals := 3
	inputTokens := int64(4)
	outputTokens := int64(5)
	details := "details"
	_ = gateway.RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Second,
		Multiplier:   2,
		Jitter:       0.5,
	}
	_ = gateway.EvaluationRequest{
		State: "state",
		Questions: map[string]gateway.Question{
			"boolean": gateway.BooleanQuestion{Instructions: "instruction"},
			"choice": gateway.ChoiceQuestion{
				Instructions: "instruction",
				Criteria:     map[string]any{"a": "A"},
			},
			"score": gateway.ScoreQuestion{
				Instructions: "instruction",
				Criteria:     []any{"low", "high"},
			},
		},
		ProviderOptions: map[string]map[string]any{"provider": {"key": "value"}},
	}
	_ = gateway.OptionalJSON{Set: true, Value: nil}
	_ = gateway.BooleanCriteria{}
	_ = gateway.EvaluationResult{
		Answers: map[string]gateway.Answer{
			"boolean": gateway.BooleanAnswer{Probability: 0.5},
			"choice":  gateway.ChoiceAnswer{Choice: "a", Probabilities: map[string]float64{"a": 1}},
			"score":   gateway.ScoreAnswer{Score: 1, Probabilities: map[string]float64{"0": 0, "1": 1}},
		},
		Rounding: &gateway.Rounding{
			ProbabilityDecimals: &probabilityDecimals,
			ScoreDecimals:       &scoreDecimals,
		},
		Usage: &gateway.Usage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
		Warnings: []gateway.Warning{{
			Type:    gateway.WarningUnsupported,
			Feature: "feature",
			Details: &details,
			Setting: "setting",
			Message: "message",
		}},
		ProviderMetadata: map[string]map[string]any{"provider": {}},
		Response:         gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.WarningType{
		gateway.WarningUnsupported,
		gateway.WarningCompatibility,
		gateway.WarningDeprecated,
		gateway.WarningOther,
	}

	var configurationError *gateway.ConfigurationError
	_, _, _ = configurationError.Error(), configurationError.Option(), configurationError.Reason()
	var validationError *gateway.ValidationError
	_, _, _ = validationError.Error(), validationError.Path(), validationError.Reason()
	var transportError *gateway.TransportError
	_, _, _ = transportError.Error(), transportError.Operation(), transportError.Unwrap()
	var responseError *gateway.ResponseError
	_, _, _, _, _, _ = responseError.Error(), responseError.Unwrap(), responseError.StatusCode(), responseError.Message(), responseError.Type(), responseError.Code()
	_, _, _, _, _, _ = responseError.Param(), responseError.GenerationID(), responseError.RequestID(), responseError.ResponseID(), responseError.Retryable(), responseError.BodyTruncated()
	_, _ = responseError.RetryAfter()
	_ = responseError.RawResponseBody()
	var responseValidationError *gateway.ResponseValidationError
	_, _, _, _, _, _ = responseValidationError.Error(), responseValidationError.Unwrap(), responseValidationError.StatusCode(), responseValidationError.Path(), responseValidationError.Reason(), responseValidationError.RequestID()
	_, _, _ = responseValidationError.ResponseID(), responseValidationError.BodyTruncated(), responseValidationError.RawResponseBody()
}

func TestExternalContractResponsesBuiltInToolsRequestFields(t *testing.T) {
	want := []struct {
		name string
		typ  reflect.Type
	}{
		{"Request", reflect.TypeOf(gateway.ResponsesRequest{})},
		{"Tools", reflect.TypeOf([]gateway.ResponseBuiltInTool(nil))},
	}

	typ := reflect.TypeOf(gateway.ResponsesBuiltInToolsRequest{})
	if typ.NumField() != len(want) {
		t.Fatalf("ResponsesBuiltInToolsRequest field count = %d, want %d", typ.NumField(), len(want))
	}
	for i := range want {
		field := typ.Field(i)
		if !field.IsExported() || field.Name != want[i].name || field.Type != want[i].typ {
			t.Fatalf("ResponsesBuiltInToolsRequest field %d = %s %s, want exported %s %s", i, field.Name, field.Type, want[i].name, want[i].typ)
		}
	}
}

func TestExternalContractResponseWebSearchToolIsFieldless(t *testing.T) {
	if got := reflect.TypeOf(gateway.ResponseWebSearchTool{}).NumField(); got != 0 {
		t.Fatalf("ResponseWebSearchTool field count = %d, want 0", got)
	}
}

func TestExternalContractChatCompletionRequestFields(t *testing.T) {
	want := []struct {
		name string
		typ  reflect.Type
	}{
		{"Model", reflect.TypeOf("")},
		{"Messages", reflect.TypeOf([]gateway.ChatMessage(nil))},
		{"Temperature", reflect.TypeOf((*float64)(nil))},
		{"MaxTokens", reflect.TypeOf((*int)(nil))},
		{"TopP", reflect.TypeOf((*float64)(nil))},
		{"FrequencyPenalty", reflect.TypeOf((*float64)(nil))},
		{"PresencePenalty", reflect.TypeOf((*float64)(nil))},
		{"Stop", reflect.TypeOf((*gateway.ChatStop)(nil)).Elem()},
		{"SafetyIdentifier", reflect.TypeOf((*string)(nil))},
		{"Tools", reflect.TypeOf([]gateway.ChatTool(nil))},
		{"ToolChoice", reflect.TypeOf((*gateway.ChatToolChoice)(nil)).Elem()},
		{"ResponseFormat", reflect.TypeOf((*gateway.ChatResponseFormat)(nil)).Elem()},
		{"Models", reflect.TypeOf([]string(nil))},
		{"ProviderOptions", reflect.TypeOf((*gateway.ChatProviderOptions)(nil))},
		{"Provider", reflect.TypeOf((*gateway.ChatProvider)(nil))},
	}

	typ := reflect.TypeOf(gateway.ChatCompletionRequest{})
	got := make([]reflect.StructField, 0, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.IsExported() {
			got = append(got, field)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("ChatCompletionRequest exported field count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].name || got[i].Type != want[i].typ {
			t.Fatalf("ChatCompletionRequest exported field %d = %s %s, want %s %s", i, got[i].Name, got[i].Type, want[i].name, want[i].typ)
		}
	}
}
