package gateway_test

import (
	"context"
	"encoding/json"
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
	var _ gateway.ProviderOption
	var _ func(*gateway.Client, context.Context, string, gateway.EmbeddingRequest) (*gateway.EmbeddingResult, error) = (*gateway.Client).Embed
	_ = gateway.EmbeddingRequest{Values: []string{"one", "two"}, ProviderOptions: []gateway.ProviderOption{}}
	tokens := int64(-1)
	providerDetails := "details"
	_ = gateway.EmbeddingResult{
		Embeddings:       [][]float64{{1, 2}},
		Usage:            &gateway.EmbeddingUsage{Tokens: &tokens},
		Warnings:         []gateway.ProviderWarning{{Type: gateway.ProviderWarningUnsupported, Feature: "feature", Details: &providerDetails}},
		ProviderMetadata: map[string]json.RawMessage{"provider": json.RawMessage(`{"key":"value"}`)},
		Response:         gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.ProviderWarningType{gateway.ProviderWarningUnsupported, gateway.ProviderWarningCompatibility, gateway.ProviderWarningDeprecated, gateway.ProviderWarningOther}
	var _ func(*gateway.Client, context.Context, string, gateway.ImageRequest) (*gateway.ImageResult, error) = (*gateway.Client).GenerateImage
	prompt := "draw"
	imageSize := gateway.ImageSize("1024x1024")
	imageAspect := gateway.ImageAspectRatio("1:1")
	imageSeed := int64(1)
	var imageInput gateway.ImageInput = gateway.ImageURL{URL: "https://example.com/image.png", ProviderOptions: []gateway.ProviderOption{}}
	imageInput = &gateway.ImageURL{URL: ""}
	imageInput = gateway.ImageBase64{MediaType: "image/png", Data: "opaque"}
	imageInput = &gateway.ImageBase64{MediaType: "", Data: ""}
	imageInput = gateway.ImageBytes{MediaType: "image/png", Data: []byte{1}}
	imageInput = &gateway.ImageBytes{MediaType: "", Data: nil}
	_ = gateway.ImageRequest{Prompt: &prompt, Count: 1, Size: &imageSize, AspectRatio: &imageAspect, Seed: &imageSeed, Files: []gateway.ImageInput{imageInput}, Mask: &imageInput, ProviderOptions: []gateway.ProviderOption{}}
	_ = gateway.ImageResult{Images: [][]byte{{1}}, Retryable: new(false), Warnings: []gateway.ProviderWarning{}, ProviderMetadata: map[string]json.RawMessage{}, Response: gateway.ResponseMetadata{}, Usage: &gateway.ImageUsage{InputTokens: new(-1.5), OutputTokens: new(0.0), TotalTokens: new(2.5)}}
	var _ func(*gateway.Client, context.Context, string, gateway.RerankRequest) (*gateway.RerankResult, error) = (*gateway.Client).Rerank
	topN := 2
	var requestDocuments gateway.RerankDocuments = gateway.RerankTexts{Values: []string{"one", "two"}}
	requestDocuments = &gateway.RerankTexts{Values: []string{"one"}}
	requestDocuments = gateway.RerankObjects{Values: []json.RawMessage{json.RawMessage(`{"key":1}`)}}
	requestDocuments = &gateway.RerankObjects{Values: []json.RawMessage{json.RawMessage(`{"key":2}`)}}
	_ = gateway.RerankRequest{Query: "query", Documents: requestDocuments, TopN: &topN, ProviderOptions: []gateway.ProviderOption{}}
	var resultDocument gateway.RerankDocument = gateway.RerankText{Text: "one"}
	resultDocument = &gateway.RerankText{Text: "two"}
	resultDocument = gateway.RerankJSON{Value: json.RawMessage(`{"key":1}`)}
	resultDocument = &gateway.RerankJSON{Value: json.RawMessage(`{"key":2}`)}
	_ = gateway.RerankResult{Results: []gateway.RerankItem{{OriginalIndex: 0, Score: -1.5, Document: resultDocument}}, Warnings: []gateway.ProviderWarning{}, ProviderMetadata: map[string]json.RawMessage{}, Response: gateway.ResponseMetadata{}}
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
	var webSearchTool gateway.ResponseBuiltInTool = gateway.ResponseWebSearchTool{}
	var webSearchToolPointer gateway.ResponseBuiltInTool = &gateway.ResponseWebSearchTool{}
	var xSearchTool gateway.ResponseBuiltInTool = gateway.ResponseXSearchTool{}
	var xSearchToolPointer gateway.ResponseBuiltInTool = &gateway.ResponseXSearchTool{}
	var xSearchOptionsTool gateway.ResponseBuiltInTool = gateway.ResponseXSearchOptionsTool{}
	var xSearchOptionsToolPointer gateway.ResponseBuiltInTool = &gateway.ResponseXSearchOptionsTool{}
	_, _, _, _, _, _ = webSearchTool, webSearchToolPointer, xSearchTool, xSearchToolPointer, xSearchOptionsTool, xSearchOptionsToolPointer
	falseValue := false
	fromDate, toDate, emptyDate := "2026-01-02", "2026-03-04", ""
	optionForms := []gateway.ResponseBuiltInTool{
		gateway.ResponseXSearchOptionsTool{},
		gateway.ResponseXSearchOptionsTool{AllowedXHandles: []string{}, ExcludedXHandles: []string{}, EnableImageUnderstanding: &falseValue, EnableVideoUnderstanding: &falseValue},
		gateway.ResponseXSearchOptionsTool{FromDate: &fromDate, ToDate: &toDate},
		gateway.ResponseXSearchOptionsTool{FromDate: &emptyDate, ToDate: &emptyDate},
		gateway.ResponseXSearchOptionsTool{AllowedXHandles: []string{"alice", "alice"}, FromDate: &fromDate, ToDate: &toDate, EnableImageUnderstanding: new(true), EnableVideoUnderstanding: new(true)},
		&gateway.ResponseXSearchOptionsTool{ExcludedXHandles: []string{"blocked", "blocked"}, FromDate: &fromDate, ToDate: &toDate, EnableImageUnderstanding: new(true), EnableVideoUnderstanding: new(true)},
	}
	builtInRequest := gateway.ResponsesBuiltInToolsRequest{
		Request: gateway.ResponsesRequest{Model: "provider/model", Input: gateway.ResponseTextInput("hello")},
		Tools:   append([]gateway.ResponseBuiltInTool{webSearchTool, xSearchTool}, optionForms...),
	}
	_, _ = (*gateway.Client).CreateResponseWithBuiltInTools(nil, context.Background(), builtInRequest)
	_, _ = (*gateway.Client).StreamResponseWithBuiltInTools(nil, context.Background(), builtInRequest)
	// Keep an external unkeyed literal as an exact wrapper-field compile check.
	_ = gateway.ResponsesBuiltInToolsRequest{gateway.ResponsesRequest{}, []gateway.ResponseBuiltInTool{gateway.ResponseWebSearchTool{}, gateway.ResponseXSearchTool{}, gateway.ResponseXSearchOptionsTool{}}}
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
func TestExternalContractImageTypes(t *testing.T) {
	input := reflect.TypeOf((*gateway.ImageInput)(nil)).Elem()
	if input.Kind() != reflect.Interface || input.NumMethod() != 1 || input.Method(0).Name != "imageInput" || input.Method(0).IsExported() {
		t.Fatalf("ImageInput=%v methods=%v", input, input.NumMethod())
	}
	for _, value := range []any{gateway.ImageURL{}, &gateway.ImageURL{}, gateway.ImageBase64{}, &gateway.ImageBase64{}, gateway.ImageBytes{}, &gateway.ImageBytes{}} {
		if !reflect.TypeOf(value).Implements(input) {
			t.Fatalf("%T does not implement ImageInput", value)
		}
	}
	want := map[reflect.Type][]struct {
		name string
		typ  reflect.Type
	}{
		reflect.TypeOf(gateway.ImageRequest{}): {{"Prompt", reflect.TypeOf((*string)(nil))}, {"Count", reflect.TypeOf(int(0))}, {"Size", reflect.TypeOf((*gateway.ImageSize)(nil))}, {"AspectRatio", reflect.TypeOf((*gateway.ImageAspectRatio)(nil))}, {"Seed", reflect.TypeOf((*int64)(nil))}, {"Files", reflect.TypeOf([]gateway.ImageInput{})}, {"Mask", reflect.TypeOf((*gateway.ImageInput)(nil))}, {"ProviderOptions", reflect.TypeOf([]gateway.ProviderOption{})}},
		reflect.TypeOf(gateway.ImageURL{}):     {{"URL", reflect.TypeOf("")}, {"ProviderOptions", reflect.TypeOf([]gateway.ProviderOption{})}},
		reflect.TypeOf(gateway.ImageBase64{}):  {{"MediaType", reflect.TypeOf("")}, {"Data", reflect.TypeOf("")}, {"ProviderOptions", reflect.TypeOf([]gateway.ProviderOption{})}},
		reflect.TypeOf(gateway.ImageBytes{}):   {{"MediaType", reflect.TypeOf("")}, {"Data", reflect.TypeOf([]byte{})}, {"ProviderOptions", reflect.TypeOf([]gateway.ProviderOption{})}},
		reflect.TypeOf(gateway.ImageResult{}):  {{"Images", reflect.TypeOf([][]byte{})}, {"Retryable", reflect.TypeOf((*bool)(nil))}, {"Warnings", reflect.TypeOf([]gateway.ProviderWarning{})}, {"ProviderMetadata", reflect.TypeOf(map[string]json.RawMessage{})}, {"Response", reflect.TypeOf(gateway.ResponseMetadata{})}, {"Usage", reflect.TypeOf((*gateway.ImageUsage)(nil))}},
		reflect.TypeOf(gateway.ImageUsage{}):   {{"InputTokens", reflect.TypeOf((*float64)(nil))}, {"OutputTokens", reflect.TypeOf((*float64)(nil))}, {"TotalTokens", reflect.TypeOf((*float64)(nil))}},
	}
	for typ, fields := range want {
		if typ.NumField() != len(fields) {
			t.Fatalf("%s field count=%d", typ, typ.NumField())
		}
		for i, field := range fields {
			got := typ.Field(i)
			if got.Name != field.name || got.Type != field.typ {
				t.Fatalf("%s field %d=%s %s, want %s %s", typ, i, got.Name, got.Type, field.name, field.typ)
			}
		}
	}
	if _, ok := reflect.TypeOf(gateway.ImageResult{}).FieldByName("MediaType"); ok {
		t.Fatal("ImageResult unexpectedly exports MediaType")
	}
}

func TestExternalContractResponseBuiltInToolMethods(t *testing.T) {
	typ := reflect.TypeOf((*gateway.ResponseBuiltInTool)(nil)).Elem()
	if typ.Kind() != reflect.Interface {
		t.Fatalf("ResponseBuiltInTool kind = %s, want interface", typ.Kind())
	}
	if typ.NumMethod() != 1 {
		t.Fatalf("ResponseBuiltInTool method count = %d, want 1", typ.NumMethod())
	}
	method := typ.Method(0)
	if method.Name != "responseBuiltInTool" || method.IsExported() {
		t.Fatalf("ResponseBuiltInTool method = %q (exported=%t), want unexported responseBuiltInTool", method.Name, method.IsExported())
	}
	if method.Type.NumIn() != 0 || method.Type.NumOut() != 0 {
		t.Fatalf("ResponseBuiltInTool.%s signature = %s, want func()", method.Name, method.Type)
	}
}

func TestExternalContractResponsesRequestFields(t *testing.T) {
	want := []struct {
		name string
		typ  reflect.Type
	}{
		{"Model", reflect.TypeOf("")},
		{"Input", reflect.TypeOf((*gateway.ResponseInput)(nil)).Elem()},
		{"MaxOutputTokens", reflect.TypeOf((*int)(nil))},
		{"Temperature", reflect.TypeOf((*float64)(nil))},
		{"TopP", reflect.TypeOf((*float64)(nil))},
		{"PresencePenalty", reflect.TypeOf((*float64)(nil))},
		{"FrequencyPenalty", reflect.TypeOf((*float64)(nil))},
		{"Instructions", reflect.TypeOf((*string)(nil))},
		{"Tools", reflect.TypeOf([]gateway.ResponseTool(nil))},
		{"ToolChoice", reflect.TypeOf((*gateway.ResponseToolChoice)(nil)).Elem()},
		{"ParallelToolCalls", reflect.TypeOf((*bool)(nil))},
		{"AllowedTools", reflect.TypeOf([]string(nil))},
		{"Reasoning", reflect.TypeOf((*gateway.ResponseReasoning)(nil))},
		{"Text", reflect.TypeOf((*gateway.ResponseText)(nil))},
		{"Truncation", reflect.TypeOf((*string)(nil))},
		{"PreviousResponseID", reflect.TypeOf((*string)(nil))},
		{"Store", reflect.TypeOf((*bool)(nil))},
		{"Metadata", reflect.TypeOf(map[string]string(nil))},
		{"Caching", reflect.TypeOf((*string)(nil))},
		{"CacheAnchorItems", reflect.TypeOf((*int)(nil))},
		{"CacheTTL", reflect.TypeOf((*string)(nil))},
		{"PromptCacheKey", reflect.TypeOf((*string)(nil))},
	}

	typ := reflect.TypeOf(gateway.ResponsesRequest{})
	if typ.NumField() != len(want) {
		t.Fatalf("ResponsesRequest field count = %d, want %d", typ.NumField(), len(want))
	}
	for i := range want {
		field := typ.Field(i)
		if !field.IsExported() || field.Name != want[i].name || field.Type != want[i].typ {
			t.Fatalf("ResponsesRequest field %d = %s %s, want exported %s %s", i, field.Name, field.Type, want[i].name, want[i].typ)
		}
	}
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

func TestExternalContractResponseXSearchToolIsFieldless(t *testing.T) {
	if got := reflect.TypeOf(gateway.ResponseXSearchTool{}).NumField(); got != 0 {
		t.Fatalf("ResponseXSearchTool field count = %d, want 0", got)
	}
}

func TestExternalContractResponseXSearchOptionsToolFields(t *testing.T) {
	want := []struct {
		name string
		typ  reflect.Type
	}{
		{"AllowedXHandles", reflect.TypeOf([]string(nil))},
		{"ExcludedXHandles", reflect.TypeOf([]string(nil))},
		{"FromDate", reflect.TypeOf((*string)(nil))},
		{"ToDate", reflect.TypeOf((*string)(nil))},
		{"EnableImageUnderstanding", reflect.TypeOf((*bool)(nil))},
		{"EnableVideoUnderstanding", reflect.TypeOf((*bool)(nil))},
	}

	typ := reflect.TypeOf(gateway.ResponseXSearchOptionsTool{})
	if typ.NumField() != len(want) {
		t.Fatalf("ResponseXSearchOptionsTool field count = %d, want %d", typ.NumField(), len(want))
	}
	for i := range want {
		field := typ.Field(i)
		if !field.IsExported() || field.Name != want[i].name || field.Type != want[i].typ {
			t.Fatalf("ResponseXSearchOptionsTool field %d = %s %s, want exported %s %s", i, field.Name, field.Type, want[i].name, want[i].typ)
		}
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

func TestExternalContractEmbeddingFields(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []struct {
			name string
			typ  reflect.Type
		}
	}{
		{"EmbeddingRequest", gateway.EmbeddingRequest{}, []struct {
			name string
			typ  reflect.Type
		}{{"Values", reflect.TypeOf([]string(nil))}, {"ProviderOptions", reflect.TypeOf([]gateway.ProviderOption(nil))}}},
		{"EmbeddingResult", gateway.EmbeddingResult{}, []struct {
			name string
			typ  reflect.Type
		}{{"Embeddings", reflect.TypeOf([][]float64(nil))}, {"Usage", reflect.TypeOf((*gateway.EmbeddingUsage)(nil))}, {"Warnings", reflect.TypeOf([]gateway.ProviderWarning(nil))}, {"ProviderMetadata", reflect.TypeOf(map[string]json.RawMessage(nil))}, {"Response", reflect.TypeOf(gateway.ResponseMetadata{})}}},
		{"EmbeddingUsage", gateway.EmbeddingUsage{}, []struct {
			name string
			typ  reflect.Type
		}{{"Tokens", reflect.TypeOf((*int64)(nil))}}},
		{"ProviderWarning", gateway.ProviderWarning{}, []struct {
			name string
			typ  reflect.Type
		}{{"Type", reflect.TypeOf(gateway.ProviderWarningType(""))}, {"Feature", reflect.TypeOf("")}, {"Details", reflect.TypeOf((*string)(nil))}, {"Setting", reflect.TypeOf("")}, {"Message", reflect.TypeOf("")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			typ := reflect.TypeOf(test.value)
			if typ.NumField() != len(test.want) {
				t.Fatalf("field count=%d want=%d", typ.NumField(), len(test.want))
			}
			for i, w := range test.want {
				f := typ.Field(i)
				if !f.IsExported() || f.Name != w.name || f.Type != w.typ {
					t.Fatalf("field %d=%s %s want %s %s", i, f.Name, f.Type, w.name, w.typ)
				}
			}
		})
	}
}

func TestExternalContractRerankFieldsAndInterfaces(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		fields []string
	}{
		{"RerankRequest", gateway.RerankRequest{}, []string{"Query string", "Documents gateway.RerankDocuments", "TopN *int", "ProviderOptions []gateway.ProviderOption"}},
		{"RerankTexts", gateway.RerankTexts{}, []string{"Values []string"}},
		{"RerankObjects", gateway.RerankObjects{}, []string{"Values []json.RawMessage"}},
		{"RerankText", gateway.RerankText{}, []string{"Text string"}},
		{"RerankJSON", gateway.RerankJSON{}, []string{"Value json.RawMessage"}},
		{"RerankItem", gateway.RerankItem{}, []string{"OriginalIndex int", "Score float64", "Document gateway.RerankDocument"}},
		{"RerankResult", gateway.RerankResult{}, []string{"Results []gateway.RerankItem", "Warnings []gateway.ProviderWarning", "ProviderMetadata map[string]json.RawMessage", "Response gateway.ResponseMetadata"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			typ := reflect.TypeOf(test.value)
			if typ.NumField() != len(test.fields) {
				t.Fatalf("fields=%d", typ.NumField())
			}
			for i, w := range test.fields {
				f := typ.Field(i)
				got := f.Name + " " + f.Type.String()
				if !f.IsExported() || got != w {
					t.Fatalf("field %d=%q want %q", i, got, w)
				}
			}
		})
	}
	for _, value := range []any{gateway.RerankTexts{}, (*gateway.RerankTexts)(nil), gateway.RerankObjects{}, (*gateway.RerankObjects)(nil)} {
		if !reflect.TypeOf(value).Implements(reflect.TypeOf((*gateway.RerankDocuments)(nil)).Elem()) {
			t.Fatalf("%T does not implement RerankDocuments", value)
		}
	}
	for _, value := range []any{gateway.RerankText{}, (*gateway.RerankText)(nil), gateway.RerankJSON{}, (*gateway.RerankJSON)(nil)} {
		if !reflect.TypeOf(value).Implements(reflect.TypeOf((*gateway.RerankDocument)(nil)).Elem()) {
			t.Fatalf("%T does not implement RerankDocument", value)
		}
	}
	for _, iface := range []reflect.Type{reflect.TypeOf((*gateway.RerankDocuments)(nil)).Elem(), reflect.TypeOf((*gateway.RerankDocument)(nil)).Elem()} {
		if iface.NumMethod() != 1 || iface.Method(0).IsExported() || iface.Method(0).Type.NumIn() != 0 || iface.Method(0).Type.NumOut() != 0 {
			t.Fatalf("interface=%s", iface)
		}
	}
}

func TestExternalContractProviderOptionMethods(t *testing.T) {
	typ := reflect.TypeOf((*gateway.ProviderOption)(nil)).Elem()
	if typ.Kind() != reflect.Interface || typ.NumMethod() != 1 {
		t.Fatalf("ProviderOption shape=%s methods=%d", typ.Kind(), typ.NumMethod())
	}
	m := typ.Method(0)
	if m.Name != "providerOption" || m.IsExported() || m.Type.NumIn() != 0 || m.Type.NumOut() != 0 {
		t.Fatalf("method=%s %s", m.Name, m.Type)
	}
}
