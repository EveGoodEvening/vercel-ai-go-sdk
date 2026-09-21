package gateway

import (
	"reflect"
	"sort"
)

func validateChatCompletionRequest(r ChatCompletionRequest) *ValidationError {
	if err := stringBound(r.Model, memberPath("$", "model")); err != nil {
		return err
	}
	if r.Model == "" {
		return validationError(memberPath("$", "model"), "must be nonempty")
	}
	if r.Messages == nil || len(r.Messages) == 0 {
		return validationError(memberPath("$", "messages"), "must contain at least one item")
	}
	if len(r.Messages) > maxResponseMembers {
		return validationError(memberPath("$", "messages"), "must contain at most 10000 items")
	}
	for i, message := range r.Messages {
		path := indexPath(memberPath("$", "messages"), i)
		if err := validateChatRole(message.Role, memberPath(path, "role")); err != nil {
			return err
		}
		if nilValue(message.Content) {
			return validationError(memberPath(path, "content"), "required")
		}
		switch content := message.Content.(type) {
		case ChatTextContent:
			if err := stringBound(string(content), memberPath(path, "content")); err != nil {
				return err
			}
		case ChatPartsContent:
			if content == nil {
				return validationError(memberPath(path, "content"), "required")
			}
			if len(content) > maxResponseMembers {
				return validationError(memberPath(path, "content"), "must contain at most 10000 items")
			}
			for j, part := range content {
				partPath := indexPath(memberPath(path, "content"), j)
				if nilValue(part) {
					return validationError(partPath, "must be a non-nil content part")
				}
				switch part := part.(type) {
				case ChatTextPart:
					if err := stringBound(part.Text, memberPath(partPath, "text")); err != nil {
						return err
					}
				case ChatImageURLPart:
					if part.URL == "" {
						return validationError(memberPath(memberPath(partPath, "image_url"), "url"), "must be nonempty")
					}
					if err := stringBound(part.URL, memberPath(memberPath(partPath, "image_url"), "url")); err != nil {
						return err
					}
					if err := stringPointer(part.Detail, memberPath(memberPath(partPath, "image_url"), "detail")); err != nil {
						return err
					}
					if part.Detail != nil && *part.Detail != "auto" && *part.Detail != "low" && *part.Detail != "high" {
						return validationError(memberPath(memberPath(partPath, "image_url"), "detail"), "must be auto, low, or high")
					}
				case ChatFilePart:
					filePath := memberPath(partPath, "file")
					for name, value := range map[string]string{"data": part.Data, "media_type": part.MediaType, "filename": part.Filename} {
						if value == "" {
							return validationError(memberPath(filePath, name), "must be nonempty")
						}
						if err := stringBound(value, memberPath(filePath, name)); err != nil {
							return err
						}
					}
				default:
					return validationError(partPath, "content part type is unsupported")
				}
			}
		default:
			return validationError(memberPath(path, "content"), "content type is unsupported")
		}
	}
	if err := boundedFloat(r.Temperature, 0, 2, memberPath("$", "temperature")); err != nil {
		return err
	}
	if err := boundedFloat(r.TopP, 0, 1, memberPath("$", "top_p")); err != nil {
		return err
	}
	if err := boundedFloat(r.FrequencyPenalty, -2, 2, memberPath("$", "frequency_penalty")); err != nil {
		return err
	}
	if err := boundedFloat(r.PresencePenalty, -2, 2, memberPath("$", "presence_penalty")); err != nil {
		return err
	}
	if r.Stop != nil {
		switch stop := r.Stop.(type) {
		case ChatStopString:
			if err := stringBound(string(stop), memberPath("$", "stop")); err != nil {
				return err
			}
		case ChatStopStrings:
			if stop == nil {
				return validationError(memberPath("$", "stop"), "required")
			}
			if len(stop) > maxResponseMembers {
				return validationError(memberPath("$", "stop"), "must contain at most 10000 items")
			}
			for i, value := range stop {
				if err := stringBound(value, indexPath(memberPath("$", "stop"), i)); err != nil {
					return err
				}
			}
		default:
			return validationError(memberPath("$", "stop"), "stop type is unsupported")
		}
	}
	if r.SafetyIdentifier != nil {
		if *r.SafetyIdentifier == "" {
			return validationError(memberPath("$", "safety_identifier"), "must be nonempty")
		}
		if err := stringBound(*r.SafetyIdentifier, memberPath("$", "safety_identifier")); err != nil {
			return err
		}
	}
	if len(r.Tools) > maxResponseMembers {
		return validationError(memberPath("$", "tools"), "must contain at most 10000 items")
	}
	for i, tool := range r.Tools {
		path := memberPath(indexPath(memberPath("$", "tools"), i), "function")
		if tool.Name == "" {
			return validationError(memberPath(path, "name"), "must be nonempty")
		}
		if err := stringBound(tool.Name, memberPath(path, "name")); err != nil {
			return err
		}
		if err := stringPointer(tool.Description, memberPath(path, "description")); err != nil {
			return err
		}
		parametersPath := memberPath(path, "parameters")
		if err := validateChatJSONObject(tool.Parameters, parametersPath); err != nil {
			return err
		}
	}
	if r.ToolChoice != nil {
		switch choice := r.ToolChoice.(type) {
		case ChatToolChoiceMode:
			if choice != ChatToolChoiceAuto && choice != ChatToolChoiceNone && choice != ChatToolChoiceRequired {
				return validationError(memberPath("$", "tool_choice"), "must be auto, none, or required")
			}
		case ChatSpecificToolChoice:
			if choice.Name == "" {
				return validationError(memberPath(memberPath(memberPath("$", "tool_choice"), "function"), "name"), "must be nonempty")
			}
			if err := stringBound(choice.Name, memberPath(memberPath(memberPath("$", "tool_choice"), "function"), "name")); err != nil {
				return err
			}
		default:
			return validationError(memberPath("$", "tool_choice"), "tool choice type is unsupported")
		}
	}
	if err := validateChatResponseFormat(r.ResponseFormat); err != nil {
		return err
	}
	if err := validateStringList(r.Models, memberPath("$", "models"), true); err != nil {
		return err
	}
	if r.ProviderOptions != nil {
		path := memberPath(memberPath("$", "providerOptions"), "gateway")
		g := r.ProviderOptions.Gateway
		if err := validateStringList(g.Order, memberPath(path, "order"), true); err != nil {
			return err
		}
		if err := validateStringList(g.Models, memberPath(path, "models"), true); err != nil {
			return err
		}
		if err := validateChatSort(g.Sort, memberPath(path, "sort")); err != nil {
			return err
		}
		if g.ProviderTimeouts != nil {
			byokPath := memberPath(memberPath(path, "providerTimeouts"), "byok")
			if g.ProviderTimeouts.BYOK == nil {
				return validationError(byokPath, "must be a non-null object")
			}
			keys := make([]string, 0, len(g.ProviderTimeouts.BYOK))
			for key := range g.ProviderTimeouts.BYOK {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) > maxResponseMembers {
				return validationError(byokPath, "must contain at most 10000 members")
			}
			for _, key := range keys {
				p := memberPath(byokPath, key)
				if key == "" {
					return validationError(p, "must be nonempty")
				}
				if err := stringBound(key, p); err != nil {
					return err
				}
				if timeout := g.ProviderTimeouts.BYOK[key]; timeout < 1000 || timeout > 789000 {
					return validationError(p, "must be between 1000 and 789000 inclusive")
				}
			}
		}
	}
	if r.Provider != nil {
		if r.Provider.Sort == "" {
			return validationError(memberPath(memberPath("$", "provider"), "sort"), "must be nonempty")
		}
		value := r.Provider.Sort
		if err := validateChatSort(&value, memberPath(memberPath("$", "provider"), "sort")); err != nil {
			return err
		}
		if r.ProviderOptions != nil && r.ProviderOptions.Gateway.Sort != nil && *r.ProviderOptions.Gateway.Sort != r.Provider.Sort {
			return validationError(memberPath(memberPath("$", "provider"), "sort"), "must match providerOptions.gateway.sort")
		}
	}
	return nil
}

func validateChatCompletionWithServerTools(r ChatCompletionRequest, serverTools []ChatServerTool) *ValidationError {
	if len(r.Tools) > maxResponseMembers-len(serverTools) {
		return validationError(memberPath("$", "tools"), "must contain at most 10000 items")
	}
	if err := validateChatCompletionRequest(r); err != nil {
		return err
	}
	present := make(map[string]bool, 4)
	for i, tool := range serverTools {
		path := indexPath(memberPath("$", "tools"), len(r.Tools)+i)
		if nilValue(tool) {
			return validationError(path, "must be a non-nil server tool")
		}
		identifier, err := validateChatServerTool(tool, path)
		if err != nil {
			return err
		}
		present[identifier] = true
	}
	for i, tool := range r.Tools {
		if present[tool.Name] {
			return validationError(memberPath(memberPath(indexPath(memberPath("$", "tools"), i), "function"), "name"), "must not collide with a present server tool")
		}
	}
	if choice, ok := r.ToolChoice.(ChatSpecificToolChoice); ok {
		for identifier := range present {
			if choice.Name == identifier || choice.Name == "vercel:"+identifier {
				return validationError(memberPath(memberPath(memberPath("$", "tool_choice"), "function"), "name"), "must not select a present server tool")
			}
		}
	}
	return nil
}

func validateChatServerTool(tool ChatServerTool, path string) (string, *ValidationError) {
	configPath := memberPath(path, "config")
	var identifier string
	var config map[string]any
	switch tool := tool.(type) {
	case ChatExaSearchTool:
		identifier = "exa_search"
		if err := validateChatExaSearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatExaSearchConfig(tool.Config)
	case *ChatExaSearchTool:
		identifier = "exa_search"
		if err := validateChatExaSearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatExaSearchConfig(tool.Config)
	case ChatParallelSearchTool:
		identifier = "parallel_search"
		if err := validateChatParallelSearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatParallelSearchConfig(tool.Config)
	case *ChatParallelSearchTool:
		identifier = "parallel_search"
		if err := validateChatParallelSearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatParallelSearchConfig(tool.Config)
	case ChatPerplexitySearchTool:
		identifier = "perplexity_search"
		if err := validateChatPerplexitySearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatPerplexitySearchConfig(tool.Config)
	case *ChatPerplexitySearchTool:
		identifier = "perplexity_search"
		if err := validateChatPerplexitySearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatPerplexitySearchConfig(tool.Config)
	case ChatTakoSearchTool:
		identifier = "tako_search"
		if err := validateChatTakoSearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatTakoSearchConfig(tool.Config)
	case *ChatTakoSearchTool:
		identifier = "tako_search"
		if err := validateChatTakoSearchConfig(tool.Config, configPath); err != nil {
			return "", err
		}
		config = encodeChatTakoSearchConfig(tool.Config)
	default:
		return "", validationError(path, "server tool type is unsupported")
	}
	if err := validateBoundedJSON(config, configPath); err != nil {
		return "", err
	}
	return identifier, nil
}

func validateChatExaSearchConfig(c ChatExaSearchConfig, path string) *ValidationError {
	if c.Query == "" {
		return validationError(memberPath(path, "query"), "must be nonempty")
	}
	if c.Type != nil && !oneOf(string(*c.Type), "auto", "fast", "instant") {
		return validationError(memberPath(path, "type"), "must be auto, fast, or instant")
	}
	if c.Category != nil && !oneOf(string(*c.Category), "company", "people", "research paper", "news", "personal site", "financial report") {
		return validationError(memberPath(path, "category"), "unsupported category")
	}
	if c.Contents == nil {
		return nil
	}
	contentsPath := memberPath(path, "contents")
	if nilValue(c.Contents.Text) {
		if c.Contents.Text != nil {
			return validationError(memberPath(contentsPath, "text"), "must be a non-nil text option")
		}
	} else {
		switch value := c.Contents.Text.(type) {
		case ChatExaTextEnabled, *ChatExaTextEnabled:
		case ChatExaTextOptions:
			if err := validateChatExaTextOptions(value, contentsPath); err != nil {
				return err
			}
		case *ChatExaTextOptions:
			if err := validateChatExaTextOptions(*value, contentsPath); err != nil {
				return err
			}
		default:
			return validationError(memberPath(contentsPath, "text"), "text type is unsupported")
		}
	}
	if nilValue(c.Contents.Highlights) {
		if c.Contents.Highlights != nil {
			return validationError(memberPath(contentsPath, "highlights"), "must be a non-nil highlights option")
		}
	} else {
		switch c.Contents.Highlights.(type) {
		case ChatExaHighlightsEnabled, *ChatExaHighlightsEnabled, ChatExaHighlightsOptions, *ChatExaHighlightsOptions:
		default:
			return validationError(memberPath(contentsPath, "highlights"), "highlights type is unsupported")
		}
	}
	switch value := c.Contents.SubpageTarget.(type) {
	case nil:
	case ChatExaSubpageTargetString, ChatExaSubpageTargetStrings:
	case *ChatExaSubpageTargetString:
		if value == nil {
			return validationError(memberPath(contentsPath, "subpage_target"), "must be a non-nil subpage target")
		}
	case *ChatExaSubpageTargetStrings:
		if value == nil {
			return validationError(memberPath(contentsPath, "subpage_target"), "must be a non-nil subpage target")
		}
	default:
		return validationError(memberPath(contentsPath, "subpage_target"), "subpage target type is unsupported")
	}
	return nil
}

func validateChatExaTextOptions(value ChatExaTextOptions, contentsPath string) *ValidationError {
	if value.Verbosity != nil && !oneOf(string(*value.Verbosity), "compact", "standard", "full") {
		return validationError(memberPath(memberPath(contentsPath, "text"), "verbosity"), "must be compact, standard, or full")
	}
	for name, sections := range map[string]*[]ChatExaSection{"include_sections": value.IncludeSections, "exclude_sections": value.ExcludeSections} {
		if sections != nil {
			for i, section := range *sections {
				if !oneOf(string(section), "header", "navigation", "banner", "body", "sidebar", "footer", "metadata") {
					return validationError(indexPath(memberPath(memberPath(contentsPath, "text"), name), i), "unsupported section")
				}
			}
		}
	}
	return nil
}

func validateChatParallelSearchConfig(c ChatParallelSearchConfig, path string) *ValidationError {
	if c.Objective == "" {
		return validationError(memberPath(path, "objective"), "must be nonempty")
	}
	if c.Mode != nil && !oneOf(string(*c.Mode), "one-shot", "agentic") {
		return validationError(memberPath(path, "mode"), "must be one-shot or agentic")
	}
	return nil
}

func validateChatPerplexitySearchConfig(c ChatPerplexitySearchConfig, path string) *ValidationError {
	queryPath := memberPath(path, "query")
	if nilValue(c.Query) {
		return validationError(queryPath, "required")
	}
	switch query := c.Query.(type) {
	case ChatPerplexityQueryString:
		if query == "" {
			return validationError(queryPath, "must be nonempty")
		}
	case *ChatPerplexityQueryString:
		if *query == "" {
			return validationError(queryPath, "must be nonempty")
		}
	case ChatPerplexityQueryStrings:
		if len(query) == 0 {
			return validationError(queryPath, "must contain at least one item")
		}
	case *ChatPerplexityQueryStrings:
		if len(*query) == 0 {
			return validationError(queryPath, "must contain at least one item")
		}
	default:
		return validationError(queryPath, "query type is unsupported")
	}
	if c.SearchRecencyFilter != nil && !oneOf(string(*c.SearchRecencyFilter), "day", "week", "month", "year") {
		return validationError(memberPath(path, "search_recency_filter"), "must be day, week, month, or year")
	}
	return nil
}

func validateChatTakoSearchConfig(c ChatTakoSearchConfig, path string) *ValidationError {
	if c.Query == "" {
		return validationError(memberPath(path, "query"), "must be nonempty")
	}
	if c.Effort != nil && !oneOf(string(*c.Effort), "deep", "fast", "instant") {
		return validationError(memberPath(path, "effort"), "must be deep, fast, or instant")
	}
	if c.Sources != nil && c.Sources.Data != nil {
		data := c.Sources.Data
		dataPath := memberPath(memberPath(path, "sources"), "data")
		if data.Mode != nil && !oneOf(string(*data.Mode), "inline", "url") {
			return validationError(memberPath(dataPath, "mode"), "must be inline or url")
		}
		if data.ContentFormat != nil && !oneOf(string(*data.ContentFormat), "card_json", "csv", "json_compact", "json_records") {
			return validationError(memberPath(dataPath, "content_format"), "unsupported content format")
		}
		if data.Strict != nil && *data.Strict && (data.NodeIDs == nil || len(*data.NodeIDs) == 0) {
			return validationError(memberPath(dataPath, "node_ids"), "must be present and nonempty when strict is true")
		}
	}
	if c.Sources != nil && c.Sources.Web != nil && c.Sources.Web.Category != nil && !oneOf(string(*c.Sources.Web.Category), "finance", "news", "sports") {
		return validationError(memberPath(memberPath(memberPath(path, "sources"), "web"), "category"), "must be finance, news, or sports")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateChatRole(role, path string) *ValidationError {
	if err := stringBound(role, path); err != nil {
		return err
	}
	switch role {
	case "system", "developer", "user", "assistant":
		return nil
	default:
		return validationError(path, "unsupported role")
	}
}
func validateStringList(values []string, path string, nonempty bool) *ValidationError {
	if len(values) > maxResponseMembers {
		return validationError(path, "must contain at most 10000 items")
	}
	for i, value := range values {
		p := indexPath(path, i)
		if nonempty && value == "" {
			return validationError(p, "must be nonempty")
		}
		if err := stringBound(value, p); err != nil {
			return err
		}
	}
	return nil
}
func validateChatSort(value *string, path string) *ValidationError {
	if value == nil {
		return nil
	}
	if err := stringBound(*value, path); err != nil {
		return err
	}
	if *value != "cost" && *value != "ttft" && *value != "tps" {
		return validationError(path, "must be cost, ttft, or tps")
	}
	return nil
}
func validateChatResponseFormat(format ChatResponseFormat) *ValidationError {
	if format == nil {
		return nil
	}
	path := memberPath("$", "response_format")
	switch format := format.(type) {
	case ChatResponseFormatType:
		if format != ChatResponseFormatText && format != ChatResponseFormatJSON {
			return validationError(memberPath(path, "type"), "unsupported format")
		}
	case ChatJSONSchemaResponseFormat:
		jp := memberPath(path, "json_schema")
		if format.Name == "" {
			return validationError(memberPath(jp, "name"), "must be nonempty")
		}
		if err := stringBound(format.Name, memberPath(jp, "name")); err != nil {
			return err
		}
		if err := stringPointer(format.Description, memberPath(jp, "description")); err != nil {
			return err
		}
		schemaPath := memberPath(jp, "schema")
		if err := validateChatJSONObject(format.Schema, schemaPath); err != nil {
			return err
		}
	case ChatLegacyJSONResponseFormat:
		if format.Schema != nil {
			if err := validateBoundedJSON(format.Schema, memberPath(path, "schema")); err != nil {
				return err
			}
		}
		if err := stringPointer(format.Name, memberPath(path, "name")); err != nil {
			return err
		}
		if err := stringPointer(format.Description, memberPath(path, "description")); err != nil {
			return err
		}
	default:
		return validationError(path, "response format type is unsupported")
	}
	return nil
}

func validateChatJSONObject(value any, path string) *ValidationError {
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return validationError(path, "must be a non-null object")
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Map || v.IsNil() || v.Type().Key().Kind() != reflect.String {
		return validationError(path, "must be a non-null object")
	}
	return validateBoundedJSON(value, path)
}
