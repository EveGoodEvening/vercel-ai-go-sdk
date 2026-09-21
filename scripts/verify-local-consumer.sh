#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
consumer_dir=$(mktemp -d "${TMPDIR:-/tmp}/gateway-local-consumer.XXXXXX")
trap 'rm -rf -- "$consumer_dir"' EXIT

cat >"$consumer_dir/go.mod" <<EOF
module example.com/gateway-local-consumer

go 1.26

require github.com/EveGoodEvening/vercel-ai-go-sdk v0.0.0

replace github.com/EveGoodEvening/vercel-ai-go-sdk => $repo_root
EOF

cat >"$consumer_dir/main.go" <<'EOF'
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

type tokenSource struct{}

func (tokenSource) Token(context.Context) (string, error) { return "token", nil }

func compileEmbedding(client *gateway.Client) (*gateway.EmbeddingResult, error) {
	tokens := int64(-1)
	details := "details"
	request := gateway.EmbeddingRequest{Values: []string{"one", "two"}, ProviderOptions: []gateway.ProviderOption{}}
	_ = gateway.EmbeddingResult{
		Embeddings: [][]float64{{1, 2}}, Usage: &gateway.EmbeddingUsage{Tokens: &tokens},
		Warnings: []gateway.ProviderWarning{{Type: gateway.ProviderWarningUnsupported, Feature: "feature", Details: &details}},
		ProviderMetadata: map[string]json.RawMessage{"provider": json.RawMessage(`{"key":true}`)},
		Response: gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.ProviderWarningType{gateway.ProviderWarningUnsupported, gateway.ProviderWarningCompatibility, gateway.ProviderWarningDeprecated, gateway.ProviderWarningOther}
	return client.Embed(context.Background(), "provider/model", request)
}

func compileImage(client *gateway.Client) (*gateway.ImageResult, error) {
	prompt := "draw"
	size, aspect, seed := gateway.ImageSize("1024x1024"), gateway.ImageAspectRatio("1:1"), int64(1)
	var input gateway.ImageInput = gateway.ImageURL{URL: "https://example.com/image.png"}
	input = &gateway.ImageURL{URL: ""}
	input = gateway.ImageBase64{MediaType: "image/png", Data: "opaque"}
	input = &gateway.ImageBase64{}
	input = gateway.ImageBytes{MediaType: "image/png", Data: []byte{1}}
	input = &gateway.ImageBytes{}
	request := gateway.ImageRequest{Prompt: &prompt, Count: 1, Size: &size, AspectRatio: &aspect, Seed: &seed, Files: []gateway.ImageInput{input}, Mask: &input, ProviderOptions: []gateway.ProviderOption{}}
	_ = gateway.ImageResult{Images: [][]byte{{1}}, Retryable: new(false), Warnings: []gateway.ProviderWarning{}, ProviderMetadata: map[string]json.RawMessage{}, Response: gateway.ResponseMetadata{}, Usage: &gateway.ImageUsage{InputTokens: new(-1.5), OutputTokens: new(0.0), TotalTokens: new(2.5)}}
	return client.GenerateImage(context.Background(), "provider/model", request)
}

func compileRerank(client *gateway.Client) (*gateway.RerankResult, error) {
	topN := 2
	var documents gateway.RerankDocuments = gateway.RerankTexts{Values: []string{"one", "two"}}
	documents = &gateway.RerankTexts{Values: []string{"one", "two"}}
	documents = gateway.RerankObjects{Values: []json.RawMessage{json.RawMessage(`{"key":1}`)}}
	documents = &gateway.RerankObjects{Values: []json.RawMessage{json.RawMessage(`{"key":2}`)}}
	var document gateway.RerankDocument = gateway.RerankText{Text: "one"}
	document = &gateway.RerankText{Text: "two"}
	document = gateway.RerankJSON{Value: json.RawMessage(`{"key":1}`)}
	document = &gateway.RerankJSON{Value: json.RawMessage(`{"key":2}`)}
	_ = gateway.RerankResult{Results: []gateway.RerankItem{{OriginalIndex: 0, Score: -1, Document: document}}, Warnings: []gateway.ProviderWarning{}, ProviderMetadata: map[string]json.RawMessage{}, Response: gateway.ResponseMetadata{}}
	return client.Rerank(context.Background(), "provider/model", gateway.RerankRequest{Query: "query", Documents: documents, TopN: &topN, ProviderOptions: []gateway.ProviderOption{}})
}

func compileEvaluation(client *gateway.Client) (*gateway.EvaluationResult, error) {
	zero := 0
	zero64 := int64(0)
	details := "details"
	request := gateway.EvaluationRequest{
		State: map[string]any{"candidate": "answer"},
		Questions: map[string]gateway.Question{
			"boolean": gateway.BooleanQuestion{Instructions: "judge", Criteria: &gateway.BooleanCriteria{True: gateway.OptionalJSON{Set: true, Value: "yes"}, False: gateway.OptionalJSON{Set: true, Value: "no"}}},
			"choice":  gateway.ChoiceQuestion{Instructions: "choose", Criteria: map[string]any{"a": "A"}},
			"score":   gateway.ScoreQuestion{Instructions: "score", Criteria: []any{"low", "high"}},
		},
		ProviderOptions: map[string]map[string]any{"provider": {"flag": true}},
	}
	_ = gateway.RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Second, Multiplier: 2, Jitter: 0}
	_ = gateway.EvaluationResult{
		Answers: map[string]gateway.Answer{
			"boolean": gateway.BooleanAnswer{Probability: 1},
			"choice":  gateway.ChoiceAnswer{Choice: "a", Probabilities: map[string]float64{"a": 1}},
			"score":   gateway.ScoreAnswer{Score: 1, Probabilities: map[string]float64{"1": 1}},
		},
		Rounding: &gateway.Rounding{ProbabilityDecimals: &zero, ScoreDecimals: &zero},
		Usage: &gateway.Usage{InputTokens: &zero64, OutputTokens: &zero64},
		Warnings: []gateway.Warning{{Type: gateway.WarningUnsupported, Feature: "feature", Details: &details}},
		ProviderMetadata: map[string]map[string]any{"provider": {"key": "value"}},
		Response: gateway.ResponseMetadata{ModelID: "provider/model", Headers: http.Header{}, Body: []byte{}},
	}
	_ = []gateway.WarningType{gateway.WarningUnsupported, gateway.WarningCompatibility, gateway.WarningDeprecated, gateway.WarningOther}
	return client.Evaluate(context.Background(), "provider/model", request)
}

func compileResponses(client *gateway.Client) {
	description, strict, enabled := "description", true, true
	request := gateway.ResponsesRequest{
		Model: "provider/model",
		Input: gateway.ResponseItemsInput{
			gateway.ResponseMessage{Role: "user", Content: "hello"},
			gateway.ResponseFunctionCall{ID: "item", CallID: "call", Name: "lookup", Arguments: `{}`},
			gateway.ResponseFunctionCallOutput{CallID: "call", Output: `{}`},
		},
		Tools: []gateway.ResponseTool{{Name: "lookup", Description: &description, Parameters: map[string]any{"type": "object"}, Strict: &strict}},
		ToolChoice: gateway.ResponseSpecificToolChoice{Name: "lookup"},
		ParallelToolCalls: &enabled,
		Reasoning: &gateway.ResponseReasoning{Effort: "low", Summary: &description},
		Text: &gateway.ResponseText{Format: gateway.ResponseJSONSchemaFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict}},
	}
	_ = gateway.ResponsesRequest{Model: "provider/model", Input: gateway.ResponseTextInput("hello"), ToolChoice: gateway.ResponseToolChoiceAuto, Text: &gateway.ResponseText{Format: gateway.ResponseTextFormatText}}
	_ = []gateway.ResponseToolChoiceMode{gateway.ResponseToolChoiceAuto, gateway.ResponseToolChoiceRequired, gateway.ResponseToolChoiceNone}
	_ = []gateway.ResponseTextFormatType{gateway.ResponseTextFormatText, gateway.ResponseTextFormatJSONObject}
	var input gateway.ResponseInput = gateway.ResponseItemsInput{}
	var item gateway.ResponseInputItem = gateway.ResponseMessage{}
	var choice gateway.ResponseToolChoice = gateway.ResponseToolChoiceNone
	var format gateway.ResponseTextFormat = gateway.ResponseTextFormatJSONObject
	_, _, _, _ = input, item, choice, format
	webSearchTool := gateway.ResponseBuiltInTool(gateway.ResponseWebSearchTool{})
	webSearchToolPointer := gateway.ResponseBuiltInTool(&gateway.ResponseWebSearchTool{})
	xSearchTool := gateway.ResponseBuiltInTool(gateway.ResponseXSearchTool{})
	xSearchToolPointer := gateway.ResponseBuiltInTool(&gateway.ResponseXSearchTool{})
	xSearchOptionsTool := gateway.ResponseBuiltInTool(gateway.ResponseXSearchOptionsTool{})
	xSearchOptionsToolPointer := gateway.ResponseBuiltInTool(&gateway.ResponseXSearchOptionsTool{})
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
		Request: request,
		Tools:   append([]gateway.ResponseBuiltInTool{webSearchTool, webSearchToolPointer, xSearchTool, xSearchToolPointer, xSearchOptionsTool, xSearchOptionsToolPointer}, optionForms...),
	}
	for _, form := range optionForms {
		formRequest := gateway.ResponsesBuiltInToolsRequest{Request: request, Tools: []gateway.ResponseBuiltInTool{form}}
		_, _ = client.CreateResponseWithBuiltInTools(context.Background(), formRequest)
		_, _ = client.StreamResponseWithBuiltInTools(context.Background(), formRequest)
	}
	builtInResult, _ := client.CreateResponseWithBuiltInTools(context.Background(), builtInRequest)
	_ = builtInResult.RawJSON()
	builtInStream, _ := client.StreamResponseWithBuiltInTools(context.Background(), builtInRequest)
	if builtInStream != nil {
		_, _, _, _ = builtInStream.Next(), builtInStream.Event(), builtInStream.Err(), builtInStream.Close()
	}
	result, _ := client.CreateResponse(context.Background(), request)
	_ = result.RawJSON()
	stream, _ := client.StreamResponse(context.Background(), request)
	if stream != nil {
		_, _, _, _ = stream.Next(), stream.Event(), stream.Err(), stream.Close()
	}
	var event gateway.ResponseEvent = gateway.ResponseOutputTextDeltaEvent{Type: "response.output_text.delta", Event: "event", ID: "id", Delta: "text"}
	event = gateway.RawResponseEvent{Type: "other", Event: "event", ID: "id"}
	_ = event.(gateway.RawResponseEvent).RawJSON()
}

func compileChat(client *gateway.Client) {
	description, detail, strict := "description", "auto", true
	request := gateway.ChatCompletionRequest{
		Model: "provider/model",
		Messages: []gateway.ChatMessage{
			{Role: "system", Content: gateway.ChatTextContent("help")},
			{Role: "user", Content: gateway.ChatPartsContent{gateway.ChatTextPart{Text: "hello"}, gateway.ChatImageURLPart{URL: "https://example.com/image.png", Detail: &detail}, gateway.ChatFilePart{Data: "ZGF0YQ==", MediaType: "text/plain", Filename: "input.txt"}}},
		},
		Stop: gateway.ChatStopStrings{"stop"},
		Tools: []gateway.ChatTool{{Name: "lookup", Description: &description, Parameters: map[string]any{"type": "object"}}},
		ToolChoice: gateway.ChatSpecificToolChoice{Name: "lookup"},
		ResponseFormat: gateway.ChatJSONSchemaResponseFormat{Name: "answer", Description: &description, Schema: map[string]any{"type": "object"}, Strict: &strict},
		ProviderOptions: &gateway.ChatProviderOptions{Gateway: gateway.ChatGatewayOptions{Order: []string{"provider"}, Models: []string{"provider/model"}, ProviderTimeouts: &gateway.ChatProviderTimeouts{BYOK: map[string]int{"provider": 1000}}}},
		Provider: &gateway.ChatProvider{Sort: "price"},
	}
	_ = gateway.ChatCompletionRequest{Model: "provider/model", Messages: []gateway.ChatMessage{{Role: "user", Content: gateway.ChatTextContent("hello")}}, Stop: gateway.ChatStopString("stop"), ToolChoice: gateway.ChatToolChoiceAuto, ResponseFormat: gateway.ChatResponseFormatText}
	_ = []gateway.ChatToolChoiceMode{gateway.ChatToolChoiceAuto, gateway.ChatToolChoiceNone}
	_ = []gateway.ChatResponseFormatType{gateway.ChatResponseFormatText, gateway.ChatResponseFormatJSON}
	_ = gateway.ChatLegacyJSONResponseFormat{Schema: map[string]any{"type": "object"}, Name: &description, Description: &description}
	var content gateway.ChatMessageContent = gateway.ChatTextContent("hello")
	var part gateway.ChatContentPart = gateway.ChatTextPart{Text: "hello"}
	var stop gateway.ChatStop = gateway.ChatStopString("stop")
	var choice gateway.ChatToolChoice = gateway.ChatToolChoiceNone
	var format gateway.ChatResponseFormat = gateway.ChatResponseFormatJSON
	_, _, _, _, _ = content, part, stop, choice, format
	var _ gateway.ChatToolChoiceMode = gateway.ChatToolChoiceRequired
	var exaText gateway.ChatExaText = gateway.ChatExaTextEnabled(true)
	var exaHighlights gateway.ChatExaHighlights = gateway.ChatExaHighlightsEnabled(true)
	var exaSubpageTarget gateway.ChatExaSubpageTarget = gateway.ChatExaSubpageTargetString("docs")
	var perplexityQuery gateway.ChatPerplexityQuery = gateway.ChatPerplexityQueryString("query")
	_, _, _, _ = exaText, exaHighlights, exaSubpageTarget, perplexityQuery
	exaType, exaCategory, exaVerbosity := gateway.ChatExaSearchFast, gateway.ChatExaCategoryResearchPaper, gateway.ChatExaVerbosityStandard
	exaSections := []gateway.ChatExaSection{gateway.ChatExaSectionHeader, gateway.ChatExaSectionNavigation, gateway.ChatExaSectionBanner, gateway.ChatExaSectionBody, gateway.ChatExaSectionSidebar, gateway.ChatExaSectionFooter, gateway.ChatExaSectionMetadata}
	maxCharacters, includeHTML, links := 100, false, 2
	includeDomains := []string{"example.com"}
	serverRequest := gateway.ChatServerToolsRequest{Request: request, ServerTools: []gateway.ChatServerTool{
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
	_, _ = client.CreateChatCompletionWithServerTools(context.Background(), serverRequest)
	serverStream, _ := client.StreamChatCompletionWithServerTools(context.Background(), serverRequest)
	_ = serverStream
	result, _ := client.CreateChatCompletion(context.Background(), request)
	if result != nil {
		_ = result.RawJSON()
	}
	_ = gateway.ChatCompletionResult{ID: gateway.JSONField[string]{Present: true, Value: "id"}, Object: gateway.JSONField[string]{}, Created: gateway.JSONField[int64]{}, Model: gateway.JSONField[string]{}, Choices: gateway.JSONField[[]gateway.ChatChoice]{Value: []gateway.ChatChoice{{Index: gateway.JSONField[int]{}, Message: gateway.JSONField[gateway.ChatAssistantMessage]{Value: gateway.ChatAssistantMessage{Role: gateway.JSONField[string]{}, Content: gateway.JSONField[string]{}, ToolCalls: gateway.JSONField[[]gateway.ChatToolCall]{Value: []gateway.ChatToolCall{{ID: gateway.JSONField[string]{}, Type: gateway.JSONField[string]{}, Function: gateway.JSONField[gateway.ChatFunctionCall]{Value: gateway.ChatFunctionCall{Name: gateway.JSONField[string]{}, Arguments: gateway.JSONField[string]{}}}}}}}}, FinishReason: gateway.JSONField[string]{}}}}, Usage: gateway.JSONField[gateway.ChatUsage]{Value: gateway.ChatUsage{PromptTokens: gateway.JSONField[int64]{}, CompletionTokens: gateway.JSONField[int64]{}, TotalTokens: gateway.JSONField[int64]{}}}}
	stream, _ := client.StreamChatCompletion(context.Background(), request)
	if stream != nil {
		_, _, _, _ = stream.Next(), stream.Event(), stream.Err(), stream.Close()
	}
	chunk := gateway.ChatCompletionChunk{Object: "chat.completion.chunk", Choices: []gateway.ChatCompletionChunkChoice{{Delta: gateway.ChatCompletionChunkDelta{Content: "text"}}}}
	_ = chunk.RawJSON()
}

func inspectErrors(err error) {
	var configuration *gateway.ConfigurationError
	if errors.As(err, &configuration) {
		_, _, _ = configuration.Error(), configuration.Option(), configuration.Reason()
	}
	var validation *gateway.ValidationError
	if errors.As(err, &validation) {
		_, _, _ = validation.Error(), validation.Path(), validation.Reason()
	}
	var transport *gateway.TransportError
	if errors.As(err, &transport) {
		_, _, _ = transport.Error(), transport.Operation(), transport.Unwrap()
	}
	var response *gateway.ResponseError
	if errors.As(err, &response) {
		_, _, _, _ = response.Error(), response.StatusCode(), response.Message(), response.Type()
		_, _, _, _ = response.Code(), response.Param(), response.GenerationID(), response.RequestID()
		_ = response.ResponseID()
		_, _ = response.RetryAfter()
		_, _, _, _ = response.Retryable(), response.BodyTruncated(), response.RawResponseBody(), response.Unwrap()
	}
	var responseValidation *gateway.ResponseValidationError
	if errors.As(err, &responseValidation) {
		_, _, _, _ = responseValidation.Error(), responseValidation.StatusCode(), responseValidation.Path(), responseValidation.Reason()
		_, _, _, _ = responseValidation.RequestID(), responseValidation.ResponseID(), responseValidation.BodyTruncated(), responseValidation.RawResponseBody()
		_ = responseValidation.Unwrap()
	}
}

func main() {
	client, err := gateway.NewClient(
		gateway.WithAPIKey("key"),
		gateway.WithOIDCToken("token"),
		gateway.WithOIDCTokenSource(tokenSource{}),
		gateway.WithBaseURL("http://127.0.0.1"),
		gateway.WithPublicBaseURL("http://127.0.0.1"),
		gateway.WithHTTPClient(&http.Client{}),
		gateway.WithTeam("team"),
		gateway.WithHeaders(http.Header{"X-Consumer": {"value"}}),
		gateway.WithRetryPolicy(gateway.RetryPolicy{}),
	)
	inspectErrors(err)
	if client != nil {
		_, _, _, _, _ = compileEvaluation, compileEmbedding, compileRerank, compileResponses, compileChat
	}
}
EOF

cat >"$consumer_dir/check_api.go" <<'EOF'
//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var allowedTypes = names(
	"Client", "Option", "TokenSource", "RetryPolicy", "ProviderOption",
	"ConfigurationError", "ValidationError", "TransportError", "ResponseError", "ResponseValidationError",
	"EvaluationRequest", "Question", "BooleanQuestion", "ChoiceQuestion", "ScoreQuestion", "OptionalJSON", "BooleanCriteria", "EvaluationResult", "Rounding", "Usage", "WarningType", "Warning", "ResponseMetadata", "Answer", "BooleanAnswer", "ChoiceAnswer", "ScoreAnswer",
	"EmbeddingRequest", "EmbeddingResult", "EmbeddingUsage", "ProviderWarningType", "ProviderWarning",
	"ImageSize", "ImageAspectRatio", "ImageRequest", "ImageInput", "ImageURL", "ImageBase64", "ImageBytes", "ImageResult", "ImageUsage",
	"RerankRequest", "RerankDocuments", "RerankTexts", "RerankObjects", "RerankDocument", "RerankText", "RerankJSON", "RerankItem", "RerankResult",
	"ResponsesRequest", "ResponsesBuiltInToolsRequest", "ResponseBuiltInTool", "ResponseWebSearchTool", "ResponseXSearchTool", "ResponseXSearchOptionsTool", "ResponseInput", "ResponseTextInput", "ResponseItemsInput", "ResponseInputItem", "ResponseMessage", "ResponseFunctionCall", "ResponseFunctionCallOutput", "ResponseTool", "ResponseToolChoice", "ResponseToolChoiceMode", "ResponseSpecificToolChoice", "ResponseReasoning", "ResponseText", "ResponseTextFormat", "ResponseTextFormatType", "ResponseJSONSchemaFormat", "ResponseResult", "ResponseEvent", "ResponseOutputTextDeltaEvent", "RawResponseEvent", "ResponseStream",
	"ChatCompletionRequest", "ChatServerToolsRequest", "ChatServerTool", "ChatExaSearchTool", "ChatParallelSearchTool", "ChatPerplexitySearchTool", "ChatTakoSearchTool",
	"ChatExaSearchType", "ChatExaCategory", "ChatExaVerbosity", "ChatExaSection", "ChatExaText", "ChatExaTextEnabled", "ChatExaTextOptions", "ChatExaHighlights", "ChatExaHighlightsEnabled", "ChatExaHighlightsOptions", "ChatExaExtras", "ChatExaSubpageTarget", "ChatExaSubpageTargetString", "ChatExaSubpageTargetStrings", "ChatExaContents", "ChatExaSearchConfig",
	"ChatParallelMode", "ChatParallelSourcePolicy", "ChatParallelExcerpts", "ChatParallelFetchPolicy", "ChatParallelSearchConfig",
	"ChatPerplexityQuery", "ChatPerplexityQueryString", "ChatPerplexityQueryStrings", "ChatPerplexityRecency", "ChatPerplexitySearchConfig",
	"ChatTakoEffort", "ChatTakoDataMode", "ChatTakoContentFormat", "ChatTakoWebCategory", "ChatTakoDataSource", "ChatTakoWebSource", "ChatTakoSources", "ChatTakoLocation", "ChatTakoOutputSettings", "ChatTakoSearchConfig",
	"ChatMessage", "ChatMessageContent", "ChatTextContent", "ChatPartsContent", "ChatContentPart", "ChatTextPart", "ChatImageURLPart", "ChatFilePart", "ChatStop", "ChatStopString", "ChatStopStrings", "ChatTool", "ChatToolChoice", "ChatToolChoiceMode", "ChatSpecificToolChoice", "ChatResponseFormat", "ChatResponseFormatType", "ChatJSONSchemaResponseFormat", "ChatLegacyJSONResponseFormat", "ChatProviderOptions", "ChatGatewayOptions", "ChatProviderTimeouts", "ChatProvider", "JSONField", "ChatCompletionResult", "ChatChoice", "ChatAssistantMessage", "ChatToolCall", "ChatFunctionCall", "ChatUsage", "ChatCompletionChunk", "ChatCompletionChunkChoice", "ChatCompletionChunkDelta", "ChatCompletionStream",
)

var allowedFunctions = names("NewClient", "WithAPIKey", "WithOIDCToken", "WithOIDCTokenSource", "WithBaseURL", "WithPublicBaseURL", "WithHTTPClient", "WithTeam", "WithHeaders", "WithRetryPolicy")
var allowedMethods = names(
	"TokenSource.Token",
	"Client.Evaluate", "Client.Embed", "Client.GenerateImage", "Client.Rerank", "Client.CreateResponse", "Client.StreamResponse", "Client.CreateResponseWithBuiltInTools", "Client.StreamResponseWithBuiltInTools", "Client.CreateChatCompletion", "Client.StreamChatCompletion", "Client.CreateChatCompletionWithServerTools", "Client.StreamChatCompletionWithServerTools",
	"ResponseResult.RawJSON", "RawResponseEvent.RawJSON", "ResponseStream.Next", "ResponseStream.Event", "ResponseStream.Err", "ResponseStream.Close",
	"ChatCompletionResult.RawJSON", "ChatCompletionChunk.RawJSON", "ChatCompletionStream.Next", "ChatCompletionStream.Event", "ChatCompletionStream.Err", "ChatCompletionStream.Close",
	"ConfigurationError.Error", "ConfigurationError.Option", "ConfigurationError.Reason",
	"ValidationError.Error", "ValidationError.Path", "ValidationError.Reason",
	"TransportError.Error", "TransportError.Operation", "TransportError.Unwrap",
	"ResponseError.Error", "ResponseError.Unwrap", "ResponseError.StatusCode", "ResponseError.Message", "ResponseError.Type", "ResponseError.Code", "ResponseError.Param", "ResponseError.GenerationID", "ResponseError.RequestID", "ResponseError.ResponseID", "ResponseError.RetryAfter", "ResponseError.Retryable", "ResponseError.BodyTruncated", "ResponseError.RawResponseBody",
	"ResponseValidationError.Error", "ResponseValidationError.Unwrap", "ResponseValidationError.StatusCode", "ResponseValidationError.Path", "ResponseValidationError.Reason", "ResponseValidationError.RequestID", "ResponseValidationError.ResponseID", "ResponseValidationError.BodyTruncated", "ResponseValidationError.RawResponseBody",
)
var expectedInterfaceMethods = map[string]map[string]string{
	"TokenSource":             {},
	"ProviderOption":          {"providerOption": "func()"},
	"Question":                {"questionType": "func() string"},
	"Answer":                  {"answerType": "func() string"},
	"RerankDocuments":         {"rerankDocuments": "func()"},
	"ImageInput":              {"imageInput": "func()"},
	"RerankDocument":          {"rerankDocument": "func()"},
	"ResponseBuiltInTool":     {"responseBuiltInTool": "func()"},
	"ResponseInput":           {"responseInput": "func()"},
	"ResponseInputItem":       {"responseInputItem": "func()"},
	"ResponseToolChoice":      {"responseToolChoice": "func()"},
	"ResponseTextFormat":      {"responseTextFormat": "func()"},
	"ResponseEvent":           {"responseEvent": "func()"},
	"ChatServerTool":          {"chatServerTool": "func()"},
	"ChatExaText":             {"chatExaText": "func()"},
	"ChatExaHighlights":       {"chatExaHighlights": "func()"},
	"ChatExaSubpageTarget":    {"chatExaSubpageTarget": "func()"},
	"ChatPerplexityQuery":     {"chatPerplexityQuery": "func()"},
	"ChatMessageContent":      {"chatMessageContent": "func()"},
	"ChatContentPart":         {"chatContentPart": "func()"},
	"ChatStop":                {"chatStop": "func()"},
	"ChatToolChoice":          {"chatToolChoice": "func()"},
	"ChatResponseFormat":      {"chatResponseFormat": "func()"},
}
var expectedStructFields = map[string][]string{
	"EmbeddingRequest": {"Values []string", "ProviderOptions []ProviderOption"},
	"EmbeddingResult": {"Embeddings [][]float64", "Usage *EmbeddingUsage", "Warnings []ProviderWarning", "ProviderMetadata map[string]json.RawMessage", "Response ResponseMetadata"},
	"EmbeddingUsage": {"Tokens *int64"},
	"ProviderWarning": {"Type ProviderWarningType", "Feature string", "Details *string", "Setting string", "Message string"},
	"ImageRequest": {"Prompt *string", "Count int", "Size *ImageSize", "AspectRatio *ImageAspectRatio", "Seed *int64", "Files []ImageInput", "Mask *ImageInput", "ProviderOptions []ProviderOption"},
	"ImageURL": {"URL string", "ProviderOptions []ProviderOption"},
	"ImageBase64": {"MediaType string", "Data string", "ProviderOptions []ProviderOption"},
	"ImageBytes": {"MediaType string", "Data []byte", "ProviderOptions []ProviderOption"},
	"ImageResult": {"Images [][]byte", "Retryable *bool", "Warnings []ProviderWarning", "ProviderMetadata map[string]json.RawMessage", "Response ResponseMetadata", "Usage *ImageUsage"},
	"ImageUsage": {"InputTokens *float64", "OutputTokens *float64", "TotalTokens *float64"},
	"RerankRequest": {"Query string", "Documents RerankDocuments", "TopN *int", "ProviderOptions []ProviderOption"},
	"RerankTexts": {"Values []string"},
	"RerankObjects": {"Values []json.RawMessage"},
	"RerankText": {"Text string"},
	"RerankJSON": {"Value json.RawMessage"},
	"RerankItem": {"OriginalIndex int", "Score float64", "Document RerankDocument"},
	"RerankResult": {"Results []RerankItem", "Warnings []ProviderWarning", "ProviderMetadata map[string]json.RawMessage", "Response ResponseMetadata"},
	"ResponseWebSearchTool":     {},
	"ResponseXSearchTool":       {},
	"ResponseXSearchOptionsTool": {"AllowedXHandles []string", "ExcludedXHandles []string", "FromDate *string", "ToDate *string", "EnableImageUnderstanding *bool", "EnableVideoUnderstanding *bool"},
}
var allowedValues = names(
	"WarningUnsupported", "WarningCompatibility", "WarningDeprecated", "WarningOther", "ProviderWarningUnsupported", "ProviderWarningCompatibility", "ProviderWarningDeprecated", "ProviderWarningOther", "ResponseToolChoiceAuto", "ResponseToolChoiceRequired", "ResponseToolChoiceNone", "ResponseTextFormatText", "ResponseTextFormatJSONObject",
	"ChatToolChoiceAuto", "ChatToolChoiceNone", "ChatToolChoiceRequired", "ChatResponseFormatText", "ChatResponseFormatJSON",
	"ChatExaSearchAuto", "ChatExaSearchFast", "ChatExaSearchInstant", "ChatExaCategoryCompany", "ChatExaCategoryPeople", "ChatExaCategoryResearchPaper", "ChatExaCategoryNews", "ChatExaCategoryPersonalSite", "ChatExaCategoryFinancialReport", "ChatExaVerbosityCompact", "ChatExaVerbosityStandard", "ChatExaVerbosityFull", "ChatExaSectionHeader", "ChatExaSectionNavigation", "ChatExaSectionBanner", "ChatExaSectionBody", "ChatExaSectionSidebar", "ChatExaSectionFooter", "ChatExaSectionMetadata",
	"ChatParallelModeOneShot", "ChatParallelModeAgentic", "ChatPerplexityRecencyDay", "ChatPerplexityRecencyWeek", "ChatPerplexityRecencyMonth", "ChatPerplexityRecencyYear", "ChatTakoEffortDeep", "ChatTakoEffortFast", "ChatTakoEffortInstant", "ChatTakoDataModeInline", "ChatTakoDataModeURL", "ChatTakoContentFormatCardJSON", "ChatTakoContentFormatCSV", "ChatTakoContentFormatJSONCompact", "ChatTakoContentFormatJSONRecords", "ChatTakoWebCategoryFinance", "ChatTakoWebCategoryNews", "ChatTakoWebCategorySports",
)

func names(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func requireAllowed(kind, name string, allowed, seen map[string]bool) {
	if !allowed[name] {
		fmt.Fprintf(os.Stderr, "unexpected exported %s found: %s\n", kind, name)
		os.Exit(1)
	}
	seen[name] = true
}

func requireAllSeen(kind string, allowed, seen map[string]bool) {
	missing := make([]string, 0)
	for name := range allowed {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	for _, name := range missing {
		fmt.Fprintf(os.Stderr, "expected exported %s not found: %s\n", kind, name)
	}
	os.Exit(1)
}

func exportedReceiver(receiver *ast.FieldList) bool {
	if receiver == nil || len(receiver.List) != 1 {
		return false
	}
	return ast.IsExported(receiverName(receiver.List[0].Type))
}

func receiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	default:
		return ""
	}
}

func embeddedTypeName(expression ast.Expr) (string, string) {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name, typed.Name
	case *ast.SelectorExpr:
		qualifier, _ := embeddedTypeName(typed.X)
		if qualifier == "" {
			return typed.Sel.Name, typed.Sel.Name
		}
		return qualifier + "." + typed.Sel.Name, typed.Sel.Name
	case *ast.StarExpr:
		return embeddedTypeName(typed.X)
	case *ast.IndexExpr:
		return embeddedTypeName(typed.X)
	case *ast.IndexListExpr:
		return embeddedTypeName(typed.X)
	case *ast.ParenExpr:
		return embeddedTypeName(typed.X)
	default:
		return "", ""
	}
}

func verifyEmbeddedTypeInspection() {
	checks := map[string]string{
		"io.Closer":                 "io.Closer",
		"*fmt.Stringer":             "fmt.Stringer",
		"constraints.Ordered[int]":  "constraints.Ordered",
		"constraints.Pair[int, int]": "constraints.Pair",
	}
	for expression, expected := range checks {
		parsed, err := parser.ParseExpr(expression)
		if err != nil {
			panic(err)
		}
		name, selected := embeddedTypeName(parsed)
		if name != expected || !ast.IsExported(selected) {
			fmt.Fprintf(os.Stderr, "embedded interface inspection self-check failed for %s\n", expression)
			os.Exit(1)
		}
	}
}

func methodSignature(expression ast.Expr) (string, error) {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expression); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func inspectInterfaceMethods(interfaceName string, interfaceType *ast.InterfaceType, expected map[string]string, allowed, seen map[string]bool) error {
	seenUnexported := make(map[string]bool, len(expected))
	for _, field := range interfaceType.Methods.List {
		if len(field.Names) == 0 {
			name, _ := embeddedTypeName(field.Type)
			if name == "" {
				name = fmt.Sprintf("%T", field.Type)
			}
			return fmt.Errorf("unexpected interface embedding found: %s.%s", interfaceName, name)
		}
		for _, name := range field.Names {
			if ast.IsExported(name.Name) {
				requireAllowed("method", interfaceName+"."+name.Name, allowed, seen)
				continue
			}
			expectedSignature, ok := expected[name.Name]
			if !ok {
				return fmt.Errorf("unexpected unexported interface method found: %s.%s", interfaceName, name.Name)
			}
			signature, err := methodSignature(field.Type)
			if err != nil {
				return fmt.Errorf("inspect signature for %s.%s: %w", interfaceName, name.Name, err)
			}
			if signature != expectedSignature {
				return fmt.Errorf("unexpected signature for unexported interface method %s.%s: got %s, want %s", interfaceName, name.Name, signature, expectedSignature)
			}
			seenUnexported[name.Name] = true
		}
	}
	for name := range expected {
		if !seenUnexported[name] {
			return fmt.Errorf("expected unexported interface method not found: %s.%s", interfaceName, name)
		}
	}
	return nil
}

func inspectStructFields(structName string, structType *ast.StructType, expected []string) error {
	got := make([]string, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		if len(field.Names) != 1 || !ast.IsExported(field.Names[0].Name) {
			return fmt.Errorf("unexpected field shape found: %s", structName)
		}
		typ, err := methodSignature(field.Type)
		if err != nil {
			return fmt.Errorf("inspect field type for %s.%s: %w", structName, field.Names[0].Name, err)
		}
		got = append(got, field.Names[0].Name+" "+typ)
	}
	if len(got) != len(expected) {
		return fmt.Errorf("unexpected field count for %s: got %d, want %d", structName, len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			return fmt.Errorf("unexpected field %d for %s: got %s, want %s", i, structName, got[i], expected[i])
		}
	}
	return nil
}

func parseInterface(expression string) *ast.InterfaceType {
	parsed, err := parser.ParseExpr(expression)
	if err != nil {
		panic(err)
	}
	interfaceType, ok := parsed.(*ast.InterfaceType)
	if !ok {
		panic("self-check expression is not an interface")
	}
	return interfaceType
}

func verifyInterfaceInspection() {
	expected := map[string]string{"marker": "func()"}
	if err := inspectInterfaceMethods("Fixture", parseInterface("interface { marker() }"), expected, map[string]bool{}, map[string]bool{}); err != nil {
		fmt.Fprintf(os.Stderr, "interface inspection self-check rejected exact marker: %v\n", err)
		os.Exit(1)
	}
	checks := map[string]string{
		"marker removal":          "interface {}",
		"marker signature change": "interface { marker(string) }",
		"private embedding":       "interface { hidden }",
	}
	for name, expression := range checks {
		if err := inspectInterfaceMethods("Fixture", parseInterface(expression), expected, map[string]bool{}, map[string]bool{}); err == nil {
			fmt.Fprintf(os.Stderr, "interface inspection self-check failed to reject %s\n", name)
			os.Exit(1)
		}
	}
}

func main() {
	verifyEmbeddedTypeInspection()
	verifyInterfaceInspection()
	entries, err := os.ReadDir(os.Args[1])
	if err != nil {
		panic(err)
	}
	seenTypes := make(map[string]bool, len(allowedTypes))
	seenFunctions := make(map[string]bool, len(allowedFunctions))
	seenMethods := make(map[string]bool, len(allowedMethods))
	seenValues := make(map[string]bool, len(allowedValues))
	seenStructs := make(map[string]bool, len(expectedStructFields))
	seenInterfaces := make(map[string]bool, len(expectedInterfaceMethods))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(os.Args[1], entry.Name()), nil, 0)
		if err != nil {
			panic(err)
		}
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range typed.Specs {
					switch spec := specification.(type) {
					case *ast.TypeSpec:
						if !ast.IsExported(spec.Name.Name) {
							continue
						}
						if spec.Assign.IsValid() {
							fmt.Fprintf(os.Stderr, "exported compatibility alias found: %s\n", spec.Name.Name)
							os.Exit(1)
						}
						requireAllowed("type", spec.Name.Name, allowedTypes, seenTypes)
						if interfaceType, ok := spec.Type.(*ast.InterfaceType); ok {
							expected, modeled := expectedInterfaceMethods[spec.Name.Name]
							if !modeled {
								fmt.Fprintf(os.Stderr, "unexpected exported interface found: %s\n", spec.Name.Name)
								os.Exit(1)
							}
							if err := inspectInterfaceMethods(spec.Name.Name, interfaceType, expected, allowedMethods, seenMethods); err != nil {
								fmt.Fprintln(os.Stderr, err)
								os.Exit(1)
							}
							seenInterfaces[spec.Name.Name] = true
						}
						if structType, modeled := spec.Type.(*ast.StructType); modeled {
							expected, exact := expectedStructFields[spec.Name.Name]
							if exact {
								if err := inspectStructFields(spec.Name.Name, structType, expected); err != nil {
									fmt.Fprintln(os.Stderr, err)
									os.Exit(1)
								}
								seenStructs[spec.Name.Name] = true
							}
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if ast.IsExported(name.Name) {
							requireAllowed("value", name.Name, allowedValues, seenValues)
							}
						}
					}
				}
			case *ast.FuncDecl:
				if !ast.IsExported(typed.Name.Name) {
					continue
				}
				if typed.Recv == nil {
					requireAllowed("function", typed.Name.Name, allowedFunctions, seenFunctions)
				} else if exportedReceiver(typed.Recv) {
					receiver := receiverName(typed.Recv.List[0].Type)
					requireAllowed("method", receiver+"."+typed.Name.Name, allowedMethods, seenMethods)
				}
			}
		}
	}
	requireAllSeen("type", allowedTypes, seenTypes)
	for interfaceName := range expectedInterfaceMethods {
		if !seenInterfaces[interfaceName] {
			fmt.Fprintf(os.Stderr, "expected exported interface not found: %s\n", interfaceName)
			os.Exit(1)
		}
	}
	for structName := range expectedStructFields {
		if !seenStructs[structName] {
			fmt.Fprintf(os.Stderr, "expected exact struct not found: %s\n", structName)
			os.Exit(1)
		}
	}
	requireAllSeen("function", allowedFunctions, seenFunctions)
	requireAllSeen("method", allowedMethods, seenMethods)
	requireAllSeen("value", allowedValues, seenValues)
}
EOF

run_with_scrubbed_gateway_env() {
	env \
		-u AI_GATEWAY_API_KEY \
		-u VERCEL_OIDC_TOKEN \
		-u AI_GATEWAY_LIVE_COST_ACK \
		-u AI_GATEWAY_PUBLIC_LIVE_COST_ACK \
		-u AI_GATEWAY_X_SEARCH_LIVE_COST_ACK \
		-u AI_GATEWAY_X_SEARCH_PROBE_INPUT \
		-u AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A \
		-u AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B \
		-u AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE \
		-u AI_GATEWAY_X_SEARCH_PROBE_TO_DATE \
		GOWORK=off GOPROXY=off "$@"
}

run_with_scrubbed_gateway_env go run "$consumer_dir/check_api.go" "$repo_root"

cd "$consumer_dir"
run_with_scrubbed_gateway_env go build ./...
