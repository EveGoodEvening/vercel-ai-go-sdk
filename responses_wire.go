package gateway

import "encoding/json"

func encodeResponsesRequest(request ResponsesRequest) ([]byte, error) {
	return encodeResponsesRequestWithBuiltInTools(request, nil, false)
}

func encodeResponsesBuiltInToolsRequest(request ResponsesBuiltInToolsRequest, stream bool) ([]byte, error) {
	return encodeResponsesRequestWithBuiltInTools(request.Request, request.Tools, stream)
}

func encodeResponsesRequestWithBuiltInTools(request ResponsesRequest, builtInTools []ResponseBuiltInTool, stream bool) ([]byte, error) {
	wire := make(map[string]any, 24)
	wire["model"] = request.Model
	wire["stream"] = stream
	switch input := request.Input.(type) {
	case ResponseTextInput:
		wire["input"] = string(input)
	case ResponseItemsInput:
		items := make([]any, len(input))
		for i, item := range input {
			switch item := item.(type) {
			case ResponseMessage:
				items[i] = map[string]any{"role": item.Role, "content": item.Content}
			case ResponseFunctionCall:
				v := map[string]any{"type": "function_call", "call_id": item.CallID, "name": item.Name, "arguments": item.Arguments}
				if item.ID != "" {
					v["id"] = item.ID
				}
				items[i] = v
			case ResponseFunctionCallOutput:
				items[i] = map[string]any{"type": "function_call_output", "call_id": item.CallID, "output": item.Output}
			}
		}
		wire["input"] = items
	}
	put := func(key string, value any) { wire[key] = value }
	if request.MaxOutputTokens != nil {
		put("max_output_tokens", *request.MaxOutputTokens)
	}
	if request.Temperature != nil {
		put("temperature", *request.Temperature)
	}
	if request.TopP != nil {
		put("top_p", *request.TopP)
	}
	if request.PresencePenalty != nil {
		put("presence_penalty", *request.PresencePenalty)
	}
	if request.FrequencyPenalty != nil {
		put("frequency_penalty", *request.FrequencyPenalty)
	}
	if request.Instructions != nil {
		put("instructions", *request.Instructions)
	}
	if request.Tools != nil || builtInTools != nil {
		tools := make([]any, 0, len(request.Tools)+len(builtInTools))
		for _, t := range request.Tools {
			v := map[string]any{"type": "function", "name": t.Name, "parameters": normalizeJSONValue(t.Parameters)}
			if t.Description != nil {
				v["description"] = *t.Description
			}
			if t.Strict != nil {
				v["strict"] = *t.Strict
			}
			tools = append(tools, v)
		}
		for _, tool := range builtInTools {
			switch tool.(type) {
			case ResponseWebSearchTool, *ResponseWebSearchTool:
				tools = append(tools, map[string]any{"type": "web_search", "search_context_size": "low"})
			}
		}
		put("tools", tools)
	}
	if request.ToolChoice != nil {
		switch choice := request.ToolChoice.(type) {
		case ResponseToolChoiceMode:
			put("tool_choice", string(choice))
		case ResponseSpecificToolChoice:
			put("tool_choice", map[string]any{"type": "function", "name": choice.Name})
		}
	}
	if request.ParallelToolCalls != nil {
		put("parallel_tool_calls", *request.ParallelToolCalls)
	}
	if request.AllowedTools != nil {
		put("allowed_tools", request.AllowedTools)
	}
	if request.Reasoning != nil {
		v := map[string]any{"effort": request.Reasoning.Effort}
		if request.Reasoning.Summary != nil {
			v["summary"] = *request.Reasoning.Summary
		}
		put("reasoning", v)
	}
	if request.Text != nil {
		v := map[string]any{}
		switch f := request.Text.Format.(type) {
		case ResponseTextFormatType:
			v["format"] = map[string]any{"type": string(f)}
		case ResponseJSONSchemaFormat:
			sf := map[string]any{"type": "json_schema", "name": f.Name, "schema": normalizeJSONValue(f.Schema)}
			if f.Description != nil {
				sf["description"] = *f.Description
			}
			if f.Strict != nil {
				sf["strict"] = *f.Strict
			}
			v["format"] = sf
		}
		put("text", v)
	}
	if request.Truncation != nil {
		put("truncation", *request.Truncation)
	}
	if request.PreviousResponseID != nil {
		put("previous_response_id", *request.PreviousResponseID)
	}
	if request.Store != nil {
		put("store", *request.Store)
	}
	if request.Metadata != nil {
		put("metadata", request.Metadata)
	}
	if request.Caching != nil {
		put("caching", *request.Caching)
	}
	if request.CacheAnchorItems != nil {
		put("cache_anchor_items", *request.CacheAnchorItems)
	}
	if request.CacheTTL != nil {
		put("cache_ttl", *request.CacheTTL)
	}
	if request.PromptCacheKey != nil {
		put("prompt_cache_key", *request.PromptCacheKey)
	}
	return json.Marshal(wire)
}
