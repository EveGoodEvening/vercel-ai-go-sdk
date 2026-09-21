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
			if choice != ChatToolChoiceAuto && choice != ChatToolChoiceNone {
				return validationError(memberPath("$", "tool_choice"), "must be auto or none")
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
