package gateway

import (
	"bytes"
	"context"
	"net/http"
	"time"
)

// ChatCompletionRequest is a non-streaming Chat Completions request.
type ChatCompletionRequest struct {
	Model            string
	Messages         []ChatMessage
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	Stop             ChatStop
	SafetyIdentifier *string
	Tools            []ChatTool
	ToolChoice       ChatToolChoice
	ResponseFormat   ChatResponseFormat
	Models           []string
	ProviderOptions  *ChatProviderOptions
	Provider         *ChatProvider
}

// ChatServerToolsRequest adds typed server tools without changing ChatCompletionRequest.
type ChatServerToolsRequest struct {
	Request     ChatCompletionRequest
	ServerTools []ChatServerTool
}

type ChatServerTool interface{ chatServerTool() }

type ChatExaSearchTool struct{ Config ChatExaSearchConfig }
type ChatParallelSearchTool struct{ Config ChatParallelSearchConfig }
type ChatPerplexitySearchTool struct{ Config ChatPerplexitySearchConfig }
type ChatTakoSearchTool struct{ Config ChatTakoSearchConfig }

func (ChatExaSearchTool) chatServerTool()        {}
func (ChatParallelSearchTool) chatServerTool()   {}
func (ChatPerplexitySearchTool) chatServerTool() {}
func (ChatTakoSearchTool) chatServerTool()       {}

type ChatExaSearchType string

const (
	ChatExaSearchAuto    ChatExaSearchType = "auto"
	ChatExaSearchFast    ChatExaSearchType = "fast"
	ChatExaSearchInstant ChatExaSearchType = "instant"
)

type ChatExaCategory string

const (
	ChatExaCategoryCompany         ChatExaCategory = "company"
	ChatExaCategoryPeople          ChatExaCategory = "people"
	ChatExaCategoryResearchPaper   ChatExaCategory = "research paper"
	ChatExaCategoryNews            ChatExaCategory = "news"
	ChatExaCategoryPersonalSite    ChatExaCategory = "personal site"
	ChatExaCategoryFinancialReport ChatExaCategory = "financial report"
)

type ChatExaVerbosity string

const (
	ChatExaVerbosityCompact  ChatExaVerbosity = "compact"
	ChatExaVerbosityStandard ChatExaVerbosity = "standard"
	ChatExaVerbosityFull     ChatExaVerbosity = "full"
)

type ChatExaSection string

const (
	ChatExaSectionHeader     ChatExaSection = "header"
	ChatExaSectionNavigation ChatExaSection = "navigation"
	ChatExaSectionBanner     ChatExaSection = "banner"
	ChatExaSectionBody       ChatExaSection = "body"
	ChatExaSectionSidebar    ChatExaSection = "sidebar"
	ChatExaSectionFooter     ChatExaSection = "footer"
	ChatExaSectionMetadata   ChatExaSection = "metadata"
)

type ChatExaText interface{ chatExaText() }
type ChatExaTextEnabled bool
type ChatExaTextOptions struct {
	MaxCharacters   *int
	IncludeHTMLTags *bool
	Verbosity       *ChatExaVerbosity
	IncludeSections *[]ChatExaSection
	ExcludeSections *[]ChatExaSection
}

func (ChatExaTextEnabled) chatExaText() {}
func (ChatExaTextOptions) chatExaText() {}

type ChatExaHighlights interface{ chatExaHighlights() }
type ChatExaHighlightsEnabled bool
type ChatExaHighlightsOptions struct {
	Query         *string
	MaxCharacters *int
}

func (ChatExaHighlightsEnabled) chatExaHighlights() {}
func (ChatExaHighlightsOptions) chatExaHighlights() {}

type ChatExaExtras struct {
	Links      *int
	ImageLinks *int
}

type ChatExaSubpageTarget interface{ chatExaSubpageTarget() }
type ChatExaSubpageTargetString string
type ChatExaSubpageTargetStrings []string

func (ChatExaSubpageTargetString) chatExaSubpageTarget()  {}
func (ChatExaSubpageTargetStrings) chatExaSubpageTarget() {}

type ChatExaContents struct {
	Text             ChatExaText
	Highlights       ChatExaHighlights
	MaxAgeHours      *int
	LivecrawlTimeout *int
	Subpages         *int
	SubpageTarget    ChatExaSubpageTarget
	Extras           *ChatExaExtras
}

type ChatExaSearchConfig struct {
	Query              string
	Type               *ChatExaSearchType
	NumResults         *int
	Category           *ChatExaCategory
	UserLocation       *string
	IncludeDomains     *[]string
	ExcludeDomains     *[]string
	StartPublishedDate *string
	EndPublishedDate   *string
	Contents           *ChatExaContents
}

type ChatParallelMode string

const (
	ChatParallelModeOneShot ChatParallelMode = "one-shot"
	ChatParallelModeAgentic ChatParallelMode = "agentic"
)

type ChatParallelSourcePolicy struct {
	IncludeDomains *[]string
	ExcludeDomains *[]string
	AfterDate      *string
}
type ChatParallelExcerpts struct {
	MaxCharsPerResult *int
	MaxCharsTotal     *int
}
type ChatParallelFetchPolicy struct{ MaxAgeSeconds *int }
type ChatParallelSearchConfig struct {
	Objective     string
	SearchQueries *[]string
	Mode          *ChatParallelMode
	MaxResults    *int
	SourcePolicy  *ChatParallelSourcePolicy
	Excerpts      *ChatParallelExcerpts
	FetchPolicy   *ChatParallelFetchPolicy
}

type ChatPerplexityQuery interface{ chatPerplexityQuery() }
type ChatPerplexityQueryString string
type ChatPerplexityQueryStrings []string

func (ChatPerplexityQueryString) chatPerplexityQuery()  {}
func (ChatPerplexityQueryStrings) chatPerplexityQuery() {}

type ChatPerplexityRecency string

const (
	ChatPerplexityRecencyDay   ChatPerplexityRecency = "day"
	ChatPerplexityRecencyWeek  ChatPerplexityRecency = "week"
	ChatPerplexityRecencyMonth ChatPerplexityRecency = "month"
	ChatPerplexityRecencyYear  ChatPerplexityRecency = "year"
)

type ChatPerplexitySearchConfig struct {
	Query                   ChatPerplexityQuery
	MaxResults              *int
	MaxTokensPerPage        *int
	MaxTokens               *int
	Country                 *string
	SearchDomainFilter      *[]string
	SearchLanguageFilter    *[]string
	SearchAfterDate         *string
	SearchBeforeDate        *string
	LastUpdatedAfterFilter  *string
	LastUpdatedBeforeFilter *string
	SearchRecencyFilter     *ChatPerplexityRecency
}

type ChatTakoEffort string

const (
	ChatTakoEffortDeep    ChatTakoEffort = "deep"
	ChatTakoEffortFast    ChatTakoEffort = "fast"
	ChatTakoEffortInstant ChatTakoEffort = "instant"
)

type ChatTakoDataMode string

const (
	ChatTakoDataModeInline ChatTakoDataMode = "inline"
	ChatTakoDataModeURL    ChatTakoDataMode = "url"
)

type ChatTakoContentFormat string

const (
	ChatTakoContentFormatCardJSON    ChatTakoContentFormat = "card_json"
	ChatTakoContentFormatCSV         ChatTakoContentFormat = "csv"
	ChatTakoContentFormatJSONCompact ChatTakoContentFormat = "json_compact"
	ChatTakoContentFormatJSONRecords ChatTakoContentFormat = "json_records"
)

type ChatTakoWebCategory string

const (
	ChatTakoWebCategoryFinance ChatTakoWebCategory = "finance"
	ChatTakoWebCategoryNews    ChatTakoWebCategory = "news"
	ChatTakoWebCategorySports  ChatTakoWebCategory = "sports"
)

type ChatTakoDataSource struct {
	Count           *int
	IncludeContents *bool
	Mode            *ChatTakoDataMode
	ContentFormat   *ChatTakoContentFormat
	MaxRows         *int
	NodeIDs         *[]string
	Strict          *bool
}
type ChatTakoWebSource struct {
	Count                  *int
	IncludeContents        *bool
	Category               *ChatTakoWebCategory
	IncludeDomains         *[]string
	ExcludeDomains         *[]string
	SnippetMaxChars        *int
	Highlights             *bool
	ArticleContentMaxChars *int
	PublishedAfter         *string
	PublishedBefore        *string
}
type ChatTakoSources struct {
	Data *ChatTakoDataSource
	Web  *ChatTakoWebSource
}
type ChatTakoLocation struct {
	Latitude  float64
	Longitude float64
}
type ChatTakoOutputSettings struct {
	ImageDarkMode *bool
	ForceRefresh  *bool
}
type ChatTakoSearchConfig struct {
	Query          string
	Effort         *ChatTakoEffort
	Sources        *ChatTakoSources
	Location       *ChatTakoLocation
	CountryCode    *string
	Locale         *string
	Timezone       *string
	OutputSettings *ChatTakoOutputSettings
	IncludeRelated *int
}

// ChatMessage is one input message. Content must be ChatTextContent or ChatPartsContent.
type ChatMessage struct {
	Role    string
	Content ChatMessageContent
}

type ChatMessageContent interface{ chatMessageContent() }
type ChatTextContent string

func (ChatTextContent) chatMessageContent() {}

type ChatPartsContent []ChatContentPart

func (ChatPartsContent) chatMessageContent() {}

type ChatContentPart interface{ chatContentPart() }
type ChatTextPart struct{ Text string }

func (ChatTextPart) chatContentPart() {}

type ChatImageURLPart struct {
	URL    string
	Detail *string
}

func (ChatImageURLPart) chatContentPart() {}

type ChatFilePart struct {
	Data      string
	MediaType string
	Filename  string
}

func (ChatFilePart) chatContentPart() {}

type ChatStop interface{ chatStop() }
type ChatStopString string

func (ChatStopString) chatStop() {}

type ChatStopStrings []string

func (ChatStopStrings) chatStop() {}

type ChatTool struct {
	Name        string
	Description *string
	Parameters  any
}

type ChatToolChoice interface{ chatToolChoice() }
type ChatToolChoiceMode string

func (ChatToolChoiceMode) chatToolChoice() {}

const (
	ChatToolChoiceAuto     ChatToolChoiceMode = "auto"
	ChatToolChoiceNone     ChatToolChoiceMode = "none"
	ChatToolChoiceRequired ChatToolChoiceMode = "required"
)

type ChatSpecificToolChoice struct{ Name string }

func (ChatSpecificToolChoice) chatToolChoice() {}

type ChatResponseFormat interface{ chatResponseFormat() }
type ChatResponseFormatType string

func (ChatResponseFormatType) chatResponseFormat() {}

const (
	ChatResponseFormatText ChatResponseFormatType = "text"
	ChatResponseFormatJSON ChatResponseFormatType = "json"
)

type ChatJSONSchemaResponseFormat struct {
	Name        string
	Description *string
	Schema      any
	Strict      *bool
}

func (ChatJSONSchemaResponseFormat) chatResponseFormat() {}

type ChatLegacyJSONResponseFormat struct {
	Schema      any
	Name        *string
	Description *string
}

func (ChatLegacyJSONResponseFormat) chatResponseFormat() {}

// ChatProviderOptions contains documented AI Gateway routing options.
type ChatProviderOptions struct{ Gateway ChatGatewayOptions }
type ChatGatewayOptions struct {
	Order            []string
	Models           []string
	Sort             *string
	ProviderTimeouts *ChatProviderTimeouts
}
type ChatProviderTimeouts struct{ BYOK map[string]int }
type ChatProvider struct{ Sort string }

// JSONField preserves absence, explicit null, and a non-null JSON value.
type JSONField[T any] struct {
	Present bool
	Null    bool
	Value   T
}

type ChatCompletionResult struct {
	ID      JSONField[string]
	Object  JSONField[string]
	Created JSONField[int64]
	Model   JSONField[string]
	Choices JSONField[[]ChatChoice]
	Usage   JSONField[ChatUsage]
	rawJSON []byte
}

func (r *ChatCompletionResult) RawJSON() []byte {
	if r == nil {
		return nil
	}
	out := make([]byte, len(r.rawJSON))
	copy(out, r.rawJSON)
	return out
}

type ChatChoice struct {
	Index        JSONField[int]
	Message      JSONField[ChatAssistantMessage]
	FinishReason JSONField[string]
}
type ChatAssistantMessage struct {
	Role      JSONField[string]
	Content   JSONField[string]
	ToolCalls JSONField[[]ChatToolCall]
}
type ChatToolCall struct {
	ID       JSONField[string]
	Type     JSONField[string]
	Function JSONField[ChatFunctionCall]
}
type ChatFunctionCall struct {
	Name      JSONField[string]
	Arguments JSONField[string]
}
type ChatUsage struct {
	PromptTokens     JSONField[int64]
	CompletionTokens JSONField[int64]
	TotalTokens      JSONField[int64]
}

// CreateChatCompletion validates and submits one non-streaming Chat Completions request.
func (client *Client) CreateChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResult, error) {
	return client.createChatCompletion(ctx, request, nil)
}

// CreateChatCompletionWithServerTools validates and submits a request with typed Gateway server tools.
func (client *Client) CreateChatCompletionWithServerTools(ctx context.Context, request ChatServerToolsRequest) (*ChatCompletionResult, error) {
	return client.createChatCompletion(ctx, request.Request, request.ServerTools)
}

func (client *Client) createChatCompletion(ctx context.Context, request ChatCompletionRequest, serverTools []ChatServerTool) (*ChatCompletionResult, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "required")
	}
	if err := validateChatCompletionWithServerTools(request, serverTools); err != nil {
		return nil, err
	}
	payload, err := encodeChatCompletionWithServerTools(request, serverTools)
	if err != nil {
		return nil, &TransportError{operation: "encode request", cause: err}
	}
	for attempt := 1; ; attempt++ {
		raw, err := client.executeChatCompletionRequest(ctx, payload)
		if err != nil {
			return nil, err
		}
		if raw.statusCode == http.StatusOK {
			if raw.bodyTruncated {
				return nil, &ResponseValidationError{cause: raw.bodyErr, statusCode: http.StatusOK, path: "$", reason: "response body exceeds 1 MiB", bodyTruncated: true, rawResponseBody: append([]byte(nil), raw.body...)}
			}
			if raw.bodyErr != nil {
				return nil, &TransportError{operation: "read response body", cause: raw.bodyErr}
			}
			return decodeChatCompletionResult(raw.body)
		}
		now := time.Now()
		if client.config.retryHooks.now != nil {
			now = client.config.retryHooks.now()
		}
		responseErr := composeResponseError(raw, now)
		if !client.canRetry(ctx, attempt, responseErr) {
			return nil, responseErr
		}
		if err := client.waitForRetry(ctx, attempt, responseErr); err != nil {
			return nil, err
		}
	}
}

func (client *Client) executeChatCompletionRequest(ctx context.Context, payload []byte) (rawEvaluationResponse, error) {
	if err := ctx.Err(); err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}

	authorization, _, err := client.config.credential.authorization(ctx)
	if err != nil {
		return rawEvaluationResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.publicEndpoint("/chat/completions"), bytes.NewReader(payload))
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "create request", cause: err}
	}
	req.Header = client.config.buildPublicHeaders(authorization)
	httpClient := &http.Client{Transport: client.config.httpClient.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Jar: client.config.httpClient.Jar, Timeout: client.config.httpClient.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}
	capture := readAndCloseResponse(ctx, resp.Body)
	return rawEvaluationResponse{statusCode: resp.StatusCode, headers: resp.Header.Clone(), body: capture.Body, bodyTruncated: capture.Truncated, bodyErr: capture.Err}, nil
}
