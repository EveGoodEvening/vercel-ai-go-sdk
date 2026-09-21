package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func encodeChatCompletionRequest(r ChatCompletionRequest) ([]byte, error) {
	return encodeChatCompletionWithServerTools(r, nil)
}

func encodeChatCompletionWithServerTools(r ChatCompletionRequest, serverTools []ChatServerTool) ([]byte, error) {
	wire := make(map[string]any, 18)
	wire["model"] = r.Model
	wire["stream"] = false
	messages := make([]any, len(r.Messages))
	for i, message := range r.Messages {
		m := map[string]any{"role": message.Role}
		switch content := message.Content.(type) {
		case ChatTextContent:
			m["content"] = string(content)
		case ChatPartsContent:
			parts := make([]any, len(content))
			for j, part := range content {
				switch part := part.(type) {
				case ChatTextPart:
					parts[j] = map[string]any{"type": "text", "text": part.Text}
				case ChatImageURLPart:
					imageURL := map[string]any{"url": part.URL}
					if part.Detail != nil {
						imageURL["detail"] = *part.Detail
					}
					parts[j] = map[string]any{"type": "image_url", "image_url": imageURL}
				case ChatFilePart:
					parts[j] = map[string]any{"type": "file", "file": map[string]any{"data": part.Data, "media_type": part.MediaType, "filename": part.Filename}}
				}
			}
			m["content"] = parts
		}
		messages[i] = m
	}
	wire["messages"] = messages
	if r.Temperature != nil {
		wire["temperature"] = *r.Temperature
	}
	if r.MaxTokens != nil {
		wire["max_tokens"] = *r.MaxTokens
	}
	if r.TopP != nil {
		wire["top_p"] = *r.TopP
	}
	if r.FrequencyPenalty != nil {
		wire["frequency_penalty"] = *r.FrequencyPenalty
	}
	if r.PresencePenalty != nil {
		wire["presence_penalty"] = *r.PresencePenalty
	}
	switch stop := r.Stop.(type) {
	case ChatStopString:
		wire["stop"] = string(stop)
	case ChatStopStrings:
		wire["stop"] = []string(stop)
	}
	if r.SafetyIdentifier != nil {
		wire["safety_identifier"] = *r.SafetyIdentifier
	}
	if r.Tools != nil || serverTools != nil {
		tools := make([]any, 0, len(r.Tools)+len(serverTools))
		for _, tool := range r.Tools {
			function := map[string]any{"name": tool.Name, "parameters": normalizeJSONValue(tool.Parameters)}
			if tool.Description != nil {
				function["description"] = *tool.Description
			}
			tools = append(tools, map[string]any{"type": "function", "function": function})
		}
		for _, tool := range serverTools {
			identifier, config := encodeChatServerTool(tool)
			tools = append(tools, map[string]any{"type": "vercel:" + identifier, "config": config})
		}
		wire["tools"] = tools
	}
	switch choice := r.ToolChoice.(type) {
	case ChatToolChoiceMode:
		wire["tool_choice"] = string(choice)
	case ChatSpecificToolChoice:
		wire["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": choice.Name}}
	}
	switch format := r.ResponseFormat.(type) {
	case ChatResponseFormatType:
		wire["response_format"] = map[string]any{"type": string(format)}
	case ChatJSONSchemaResponseFormat:
		schema := map[string]any{"name": format.Name, "schema": normalizeJSONValue(format.Schema)}
		if format.Description != nil {
			schema["description"] = *format.Description
		}
		if format.Strict != nil {
			schema["strict"] = *format.Strict
		}
		wire["response_format"] = map[string]any{"type": "json_schema", "json_schema": schema}
	case ChatLegacyJSONResponseFormat:
		f := map[string]any{"type": "json"}
		if format.Schema != nil {
			f["schema"] = normalizeJSONValue(format.Schema)
		}
		if format.Name != nil {
			f["name"] = *format.Name
		}
		if format.Description != nil {
			f["description"] = *format.Description
		}
		wire["response_format"] = f
	}
	if r.Models != nil {
		wire["models"] = r.Models
	}
	if r.ProviderOptions != nil {
		gateway := map[string]any{}
		g := r.ProviderOptions.Gateway
		if g.Order != nil {
			gateway["order"] = g.Order
		}
		if g.Models != nil {
			gateway["models"] = g.Models
		}
		if g.Sort != nil {
			gateway["sort"] = *g.Sort
		}
		if g.ProviderTimeouts != nil {
			gateway["providerTimeouts"] = map[string]any{"byok": g.ProviderTimeouts.BYOK}
		}
		wire["providerOptions"] = map[string]any{"gateway": gateway}
	}
	if r.Provider != nil {
		wire["provider"] = map[string]any{"sort": r.Provider.Sort}
	}
	return json.Marshal(wire)
}

func encodeChatServerTool(tool ChatServerTool) (string, map[string]any) {
	switch tool := tool.(type) {
	case ChatExaSearchTool:
		return "exa_search", encodeChatExaSearchConfig(tool.Config)
	case *ChatExaSearchTool:
		return "exa_search", encodeChatExaSearchConfig(tool.Config)
	case ChatParallelSearchTool:
		return "parallel_search", encodeChatParallelSearchConfig(tool.Config)
	case *ChatParallelSearchTool:
		return "parallel_search", encodeChatParallelSearchConfig(tool.Config)
	case ChatPerplexitySearchTool:
		return "perplexity_search", encodeChatPerplexitySearchConfig(tool.Config)
	case *ChatPerplexitySearchTool:
		return "perplexity_search", encodeChatPerplexitySearchConfig(tool.Config)
	case ChatTakoSearchTool:
		return "tako_search", encodeChatTakoSearchConfig(tool.Config)
	case *ChatTakoSearchTool:
		return "tako_search", encodeChatTakoSearchConfig(tool.Config)
	default:
		return "", nil
	}
}

func encodeChatExaSearchConfig(c ChatExaSearchConfig) map[string]any {
	out := map[string]any{"query": c.Query}
	putStringEnum(out, "type", c.Type)
	putPointer(out, "num_results", c.NumResults)
	putStringEnum(out, "category", c.Category)
	putPointer(out, "user_location", c.UserLocation)
	putSlicePointer(out, "include_domains", c.IncludeDomains)
	putSlicePointer(out, "exclude_domains", c.ExcludeDomains)
	putPointer(out, "start_published_date", c.StartPublishedDate)
	putPointer(out, "end_published_date", c.EndPublishedDate)
	if c.Contents != nil {
		contents := map[string]any{}
		switch value := c.Contents.Text.(type) {
		case ChatExaTextEnabled:
			contents["text"] = bool(value)
		case *ChatExaTextEnabled:
			contents["text"] = bool(*value)
		case ChatExaTextOptions:
			contents["text"] = encodeChatExaTextOptions(value)
		case *ChatExaTextOptions:
			contents["text"] = encodeChatExaTextOptions(*value)
		}
		switch value := c.Contents.Highlights.(type) {
		case ChatExaHighlightsEnabled:
			contents["highlights"] = bool(value)
		case *ChatExaHighlightsEnabled:
			contents["highlights"] = bool(*value)
		case ChatExaHighlightsOptions:
			contents["highlights"] = encodeChatExaHighlightsOptions(value)
		case *ChatExaHighlightsOptions:
			contents["highlights"] = encodeChatExaHighlightsOptions(*value)
		}
		putPointer(contents, "max_age_hours", c.Contents.MaxAgeHours)
		putPointer(contents, "livecrawl_timeout", c.Contents.LivecrawlTimeout)
		putPointer(contents, "subpages", c.Contents.Subpages)
		switch value := c.Contents.SubpageTarget.(type) {
		case ChatExaSubpageTargetString:
			contents["subpage_target"] = string(value)
		case *ChatExaSubpageTargetString:
			contents["subpage_target"] = string(*value)
		case ChatExaSubpageTargetStrings:
			contents["subpage_target"] = []string(value)
		case *ChatExaSubpageTargetStrings:
			if *value == nil {
				contents["subpage_target"] = []string{}
			} else {
				contents["subpage_target"] = []string(*value)
			}
		}
		if c.Contents.Extras != nil {
			extras := map[string]any{}
			putPointer(extras, "links", c.Contents.Extras.Links)
			putPointer(extras, "image_links", c.Contents.Extras.ImageLinks)
			contents["extras"] = extras
		}
		out["contents"] = contents
	}
	return out
}

func encodeChatExaTextOptions(value ChatExaTextOptions) map[string]any {
	options := map[string]any{}
	putPointer(options, "max_characters", value.MaxCharacters)
	putPointer(options, "include_html_tags", value.IncludeHTMLTags)
	putStringEnum(options, "verbosity", value.Verbosity)
	putSlicePointer(options, "include_sections", value.IncludeSections)
	putSlicePointer(options, "exclude_sections", value.ExcludeSections)
	return options
}

func encodeChatExaHighlightsOptions(value ChatExaHighlightsOptions) map[string]any {
	options := map[string]any{}
	putPointer(options, "query", value.Query)
	putPointer(options, "max_characters", value.MaxCharacters)
	return options
}

func encodeChatParallelSearchConfig(c ChatParallelSearchConfig) map[string]any {
	out := map[string]any{"objective": c.Objective}
	putSlicePointer(out, "search_queries", c.SearchQueries)
	putStringEnum(out, "mode", c.Mode)
	putPointer(out, "max_results", c.MaxResults)
	if c.SourcePolicy != nil {
		value := map[string]any{}
		putSlicePointer(value, "include_domains", c.SourcePolicy.IncludeDomains)
		putSlicePointer(value, "exclude_domains", c.SourcePolicy.ExcludeDomains)
		putPointer(value, "after_date", c.SourcePolicy.AfterDate)
		out["source_policy"] = value
	}
	if c.Excerpts != nil {
		value := map[string]any{}
		putPointer(value, "max_chars_per_result", c.Excerpts.MaxCharsPerResult)
		putPointer(value, "max_chars_total", c.Excerpts.MaxCharsTotal)
		out["excerpts"] = value
	}
	if c.FetchPolicy != nil {
		value := map[string]any{}
		putPointer(value, "max_age_seconds", c.FetchPolicy.MaxAgeSeconds)
		out["fetch_policy"] = value
	}
	return out
}

func encodeChatPerplexitySearchConfig(c ChatPerplexitySearchConfig) map[string]any {
	out := map[string]any{}
	switch query := c.Query.(type) {
	case ChatPerplexityQueryString:
		out["query"] = string(query)
	case *ChatPerplexityQueryString:
		out["query"] = string(*query)
	case ChatPerplexityQueryStrings:
		out["query"] = []string(query)
	case *ChatPerplexityQueryStrings:
		out["query"] = []string(*query)
	}
	putPointer(out, "max_results", c.MaxResults)
	putPointer(out, "max_tokens_per_page", c.MaxTokensPerPage)
	putPointer(out, "max_tokens", c.MaxTokens)
	putPointer(out, "country", c.Country)
	putSlicePointer(out, "search_domain_filter", c.SearchDomainFilter)
	putSlicePointer(out, "search_language_filter", c.SearchLanguageFilter)
	putPointer(out, "search_after_date", c.SearchAfterDate)
	putPointer(out, "search_before_date", c.SearchBeforeDate)
	putPointer(out, "last_updated_after_filter", c.LastUpdatedAfterFilter)
	putPointer(out, "last_updated_before_filter", c.LastUpdatedBeforeFilter)
	putStringEnum(out, "search_recency_filter", c.SearchRecencyFilter)
	return out
}

func encodeChatTakoSearchConfig(c ChatTakoSearchConfig) map[string]any {
	out := map[string]any{"query": c.Query}
	putStringEnum(out, "effort", c.Effort)
	if c.Sources != nil {
		sources := map[string]any{}
		if c.Sources.Data != nil {
			data := map[string]any{}
			putPointer(data, "count", c.Sources.Data.Count)
			putPointer(data, "include_contents", c.Sources.Data.IncludeContents)
			putStringEnum(data, "mode", c.Sources.Data.Mode)
			putStringEnum(data, "content_format", c.Sources.Data.ContentFormat)
			putPointer(data, "max_rows", c.Sources.Data.MaxRows)
			putSlicePointer(data, "node_ids", c.Sources.Data.NodeIDs)
			putPointer(data, "strict", c.Sources.Data.Strict)
			sources["data"] = data
		}
		if c.Sources.Web != nil {
			web := map[string]any{}
			putPointer(web, "count", c.Sources.Web.Count)
			putPointer(web, "include_contents", c.Sources.Web.IncludeContents)
			putStringEnum(web, "category", c.Sources.Web.Category)
			putSlicePointer(web, "include_domains", c.Sources.Web.IncludeDomains)
			putSlicePointer(web, "exclude_domains", c.Sources.Web.ExcludeDomains)
			putPointer(web, "snippet_max_chars", c.Sources.Web.SnippetMaxChars)
			putPointer(web, "highlights", c.Sources.Web.Highlights)
			putPointer(web, "article_content_max_chars", c.Sources.Web.ArticleContentMaxChars)
			putPointer(web, "published_after", c.Sources.Web.PublishedAfter)
			putPointer(web, "published_before", c.Sources.Web.PublishedBefore)
			sources["web"] = web
		}
		out["sources"] = sources
	}
	if c.Location != nil {
		out["location"] = map[string]any{"latitude": c.Location.Latitude, "longitude": c.Location.Longitude}
	}
	putPointer(out, "country_code", c.CountryCode)
	putPointer(out, "locale", c.Locale)
	putPointer(out, "timezone", c.Timezone)
	if c.OutputSettings != nil {
		settings := map[string]any{}
		putPointer(settings, "image_dark_mode", c.OutputSettings.ImageDarkMode)
		putPointer(settings, "force_refresh", c.OutputSettings.ForceRefresh)
		out["output_settings"] = settings
	}
	putPointer(out, "include_related", c.IncludeRelated)
	return out
}

func putPointer[T any](object map[string]any, name string, value *T) {
	if value != nil {
		object[name] = *value
	}
}

func putStringEnum[T ~string](object map[string]any, name string, value *T) {
	if value != nil {
		object[name] = string(*value)
	}
}

func putSlicePointer[T any](object map[string]any, name string, value *[]T) {
	if value == nil {
		return
	}
	if *value == nil {
		object[name] = []T{}
		return
	}
	object[name] = *value
}

func decodeChatCompletionResult(body []byte) (*ChatCompletionResult, error) {
	if err := validateResponseJSON(body); err != nil {
		return nil, chatResponseError(body, "$", err.Error(), err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, chatResponseError(body, "$", err.Error(), err)
	}
	result := &ChatCompletionResult{rawJSON: append([]byte(nil), body...)}
	if err := decodeJSONField(object, "id", memberPath("$", "id"), &result.ID, decodeString); err != nil {
		return nil, chatResponseError(body, err.path, err.reason, err.cause)
	}
	if err := decodeJSONField(object, "object", memberPath("$", "object"), &result.Object, decodeString); err != nil {
		return nil, chatResponseError(body, err.path, err.reason, err.cause)
	}
	if result.Object.Present && !result.Object.Null && result.Object.Value != "chat.completion" {
		return nil, chatResponseError(body, memberPath("$", "object"), "must be chat.completion", nil)
	}
	if err := decodeJSONField(object, "created", memberPath("$", "created"), &result.Created, decodeInt64); err != nil {
		return nil, chatResponseError(body, err.path, err.reason, err.cause)
	}
	if err := decodeJSONField(object, "model", memberPath("$", "model"), &result.Model, decodeString); err != nil {
		return nil, chatResponseError(body, err.path, err.reason, err.cause)
	}
	if err := decodeJSONField(object, "choices", memberPath("$", "choices"), &result.Choices, decodeChatChoices); err != nil {
		return nil, chatResponseError(body, err.path, err.reason, err.cause)
	}
	if err := decodeJSONField(object, "usage", memberPath("$", "usage"), &result.Usage, decodeChatUsage); err != nil {
		return nil, chatResponseError(body, err.path, err.reason, err.cause)
	}
	return result, nil
}

type chatDecodeError struct {
	path, reason string
	cause        error
}

func wrongType(path, want string, cause error) *chatDecodeError {
	return &chatDecodeError{path: path, reason: "must be " + want + " or null", cause: cause}
}
func decodeJSONField[T any](object map[string]json.RawMessage, key, path string, field *JSONField[T], decode func(json.RawMessage, string) (T, *chatDecodeError)) *chatDecodeError {
	raw, ok := object[key]
	if !ok {
		return nil
	}
	field.Present = true
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		field.Null = true
		return nil
	}
	value, err := decode(raw, path)
	if err != nil {
		return err
	}
	field.Value = value
	return nil
}
func decodeString(raw json.RawMessage, path string) (string, *chatDecodeError) {
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", wrongType(path, "a string", err)
	}
	return v, nil
}
func decodeInt64(raw json.RawMessage, path string) (int64, *chatDecodeError) {
	var v int64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, wrongType(path, "an integer", err)
	}
	return v, nil
}
func decodeInt(raw json.RawMessage, path string) (int, *chatDecodeError) {
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, wrongType(path, "an integer", err)
	}
	return v, nil
}
func decodeObject(raw json.RawMessage, path string) (map[string]json.RawMessage, *chatDecodeError) {
	var v map[string]json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil || v == nil {
		return nil, wrongType(path, "an object", err)
	}
	return v, nil
}

func decodeChatChoices(raw json.RawMessage, path string) ([]ChatChoice, *chatDecodeError) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || items == nil {
		return nil, wrongType(path, "an array", err)
	}
	out := make([]ChatChoice, len(items))
	for i, item := range items {
		p := indexPath(path, i)
		object, err := decodeObject(item, p)
		if err != nil {
			return nil, err
		}
		if err = decodeJSONField(object, "index", memberPath(p, "index"), &out[i].Index, decodeInt); err != nil {
			return nil, err
		}
		if err = decodeJSONField(object, "message", memberPath(p, "message"), &out[i].Message, decodeChatAssistantMessage); err != nil {
			return nil, err
		}
		if err = decodeJSONField(object, "finish_reason", memberPath(p, "finish_reason"), &out[i].FinishReason, decodeString); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func decodeChatAssistantMessage(raw json.RawMessage, path string) (ChatAssistantMessage, *chatDecodeError) {
	object, err := decodeObject(raw, path)
	if err != nil {
		return ChatAssistantMessage{}, err
	}
	var out ChatAssistantMessage
	if err = decodeJSONField(object, "role", memberPath(path, "role"), &out.Role, decodeString); err != nil {
		return out, err
	}
	if out.Role.Present && !out.Role.Null && out.Role.Value != "assistant" {
		return out, &chatDecodeError{path: memberPath(path, "role"), reason: "must be assistant"}
	}
	if err = decodeJSONField(object, "content", memberPath(path, "content"), &out.Content, decodeString); err != nil {
		return out, err
	}
	if err = decodeJSONField(object, "tool_calls", memberPath(path, "tool_calls"), &out.ToolCalls, decodeChatToolCalls); err != nil {
		return out, err
	}
	return out, nil
}
func decodeChatToolCalls(raw json.RawMessage, path string) ([]ChatToolCall, *chatDecodeError) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || items == nil {
		return nil, wrongType(path, "an array", err)
	}
	out := make([]ChatToolCall, len(items))
	for i, item := range items {
		p := indexPath(path, i)
		object, err := decodeObject(item, p)
		if err != nil {
			return nil, err
		}
		if err = decodeJSONField(object, "id", memberPath(p, "id"), &out[i].ID, decodeString); err != nil {
			return nil, err
		}
		if err = decodeJSONField(object, "type", memberPath(p, "type"), &out[i].Type, decodeString); err != nil {
			return nil, err
		}
		if out[i].Type.Present && !out[i].Type.Null && out[i].Type.Value != "function" {
			return nil, &chatDecodeError{path: memberPath(p, "type"), reason: "must be function"}
		}
		if err = decodeJSONField(object, "function", memberPath(p, "function"), &out[i].Function, decodeChatFunctionCall); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func decodeChatFunctionCall(raw json.RawMessage, path string) (ChatFunctionCall, *chatDecodeError) {
	object, err := decodeObject(raw, path)
	if err != nil {
		return ChatFunctionCall{}, err
	}
	var out ChatFunctionCall
	if err = decodeJSONField(object, "name", memberPath(path, "name"), &out.Name, decodeString); err != nil {
		return out, err
	}
	if err = decodeJSONField(object, "arguments", memberPath(path, "arguments"), &out.Arguments, decodeString); err != nil {
		return out, err
	}
	return out, nil
}
func decodeChatUsage(raw json.RawMessage, path string) (ChatUsage, *chatDecodeError) {
	object, err := decodeObject(raw, path)
	if err != nil {
		return ChatUsage{}, err
	}
	var out ChatUsage
	if err = decodeJSONField(object, "prompt_tokens", memberPath(path, "prompt_tokens"), &out.PromptTokens, decodeInt64); err != nil {
		return out, err
	}
	if err = decodeJSONField(object, "completion_tokens", memberPath(path, "completion_tokens"), &out.CompletionTokens, decodeInt64); err != nil {
		return out, err
	}
	if err = decodeJSONField(object, "total_tokens", memberPath(path, "total_tokens"), &out.TotalTokens, decodeInt64); err != nil {
		return out, err
	}
	return out, nil
}
func chatResponseError(body []byte, path, reason string, cause error) *ResponseValidationError {
	return &ResponseValidationError{cause: cause, statusCode: http.StatusOK, path: path, reason: reason, rawResponseBody: append([]byte(nil), body...)}
}
