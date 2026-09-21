package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func encodeChatCompletionRequest(r ChatCompletionRequest) ([]byte, error) {
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
	if r.Tools != nil {
		tools := make([]any, len(r.Tools))
		for i, tool := range r.Tools {
			function := map[string]any{"name": tool.Name, "parameters": normalizeJSONValue(tool.Parameters)}
			if tool.Description != nil {
				function["description"] = *tool.Description
			}
			tools[i] = map[string]any{"type": "function", "function": function}
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
