package gateway

import (
	"math"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxResponseJSONDepth  = 64
	maxResponseValueBytes = 1 << 20
	maxResponseMembers    = 10000
)

func validateResponsesRequest(r ResponsesRequest) *ValidationError {
	if err := stringBound(r.Model, memberPath("$", "model")); err != nil {
		return err
	}
	if r.Model == "" || !validModelID(r.Model) {
		return validationError(memberPath("$", "model"), "must be a nonempty provider/model string")
	}
	if nilValue(r.Input) {
		return validationError(memberPath("$", "input"), "required")
	}
	if err := validateResponseInput(r.Input, memberPath("$", "input")); err != nil {
		return err
	}
	if r.MaxOutputTokens != nil && *r.MaxOutputTokens < 0 {
		return validationError(memberPath("$", "max_output_tokens"), "must be non-negative")
	}
	if err := boundedFloat(r.Temperature, 0, 2, memberPath("$", "temperature")); err != nil {
		return err
	}
	if err := boundedFloat(r.TopP, 0, 1, memberPath("$", "top_p")); err != nil {
		return err
	}
	if err := finiteFloat(r.PresencePenalty, memberPath("$", "presence_penalty")); err != nil {
		return err
	}
	if err := finiteFloat(r.FrequencyPenalty, memberPath("$", "frequency_penalty")); err != nil {
		return err
	}
	if err := stringPointer(r.Instructions, memberPath("$", "instructions")); err != nil {
		return err
	}
	if len(r.Tools) > maxResponseMembers {
		return validationError(memberPath("$", "tools"), "must contain at most 10000 items")
	}
	for i, t := range r.Tools {
		p := indexPath(memberPath("$", "tools"), i)
		if t.Name == "" {
			return validationError(memberPath(p, "name"), "must be nonempty")
		}
		if err := stringBound(t.Name, memberPath(p, "name")); err != nil {
			return err
		}
		if err := stringPointer(t.Description, memberPath(p, "description")); err != nil {
			return err
		}
		if t.Parameters == nil {
			return validationError(memberPath(p, "parameters"), "required")
		}
		if err := validateBoundedJSON(t.Parameters, memberPath(p, "parameters")); err != nil {
			return err
		}
	}
	if r.ToolChoice != nil {
		switch c := r.ToolChoice.(type) {
		case ResponseToolChoiceMode:
			if err := stringBound(string(c), memberPath("$", "tool_choice")); err != nil {
				return err
			}
			if c != "auto" && c != "required" && c != "none" {
				return validationError(memberPath("$", "tool_choice"), "must be auto, required, or none")
			}
		case ResponseSpecificToolChoice:
			if c.Name == "" {
				return validationError(memberPath(memberPath("$", "tool_choice"), "name"), "must be nonempty")
			}
			if err := stringBound(c.Name, memberPath(memberPath("$", "tool_choice"), "name")); err != nil {
				return err
			}
		default:
			return validationError(memberPath("$", "tool_choice"), "tool choice type is unsupported")
		}
	}
	if len(r.AllowedTools) > maxResponseMembers {
		return validationError(memberPath("$", "allowed_tools"), "must contain at most 10000 items")
	}
	for i, n := range r.AllowedTools {
		if n == "" {
			return validationError(indexPath(memberPath("$", "allowed_tools"), i), "must be nonempty")
		}
		if err := stringBound(n, indexPath(memberPath("$", "allowed_tools"), i)); err != nil {
			return err
		}
	}
	if r.Reasoning != nil {
		allowed := map[string]bool{"none": true, "minimal": true, "low": true, "medium": true, "high": true, "xhigh": true, "max": true}
		if err := stringBound(r.Reasoning.Effort, memberPath(memberPath("$", "reasoning"), "effort")); err != nil {
			return err
		}
		if !allowed[r.Reasoning.Effort] {
			return validationError(memberPath(memberPath("$", "reasoning"), "effort"), "unsupported value")
		}
		if err := stringPointer(r.Reasoning.Summary, memberPath(memberPath("$", "reasoning"), "summary")); err != nil {
			return err
		}
		if r.Reasoning.Summary != nil && *r.Reasoning.Summary != "detailed" && *r.Reasoning.Summary != "auto" && *r.Reasoning.Summary != "concise" {
			return validationError(memberPath(memberPath("$", "reasoning"), "summary"), "unsupported value")
		}
	}
	if r.Text != nil {
		if nilValue(r.Text.Format) {
			return validationError(memberPath(memberPath("$", "text"), "format"), "required")
		}
		switch f := r.Text.Format.(type) {
		case ResponseTextFormatType:
			if err := stringBound(string(f), memberPath(memberPath("$", "text"), "format")); err != nil {
				return err
			}
			if f != "text" && f != "json_object" {
				return validationError(memberPath(memberPath("$", "text"), "format"), "unsupported format")
			}
		case ResponseJSONSchemaFormat:
			p := memberPath(memberPath("$", "text"), "format")
			if f.Name == "" {
				return validationError(memberPath(p, "name"), "must be nonempty")
			}
			if err := stringBound(f.Name, memberPath(p, "name")); err != nil {
				return err
			}
			if err := stringPointer(f.Description, memberPath(p, "description")); err != nil {
				return err
			}
			if f.Schema == nil {
				return validationError(memberPath(p, "schema"), "required")
			}
			if err := validateBoundedJSON(f.Schema, memberPath(p, "schema")); err != nil {
				return err
			}
		default:
			return validationError(memberPath(memberPath("$", "text"), "format"), "format type is unsupported")
		}
	}
	if err := stringPointer(r.Truncation, memberPath("$", "truncation")); err != nil {
		return err
	}
	if r.Truncation != nil && *r.Truncation != "auto" && *r.Truncation != "disabled" {
		return validationError(memberPath("$", "truncation"), "must be auto or disabled")
	}
	for _, v := range []struct {
		p *string
		n string
	}{{r.PreviousResponseID, "previous_response_id"}, {r.PromptCacheKey, "prompt_cache_key"}} {
		if err := stringPointer(v.p, memberPath("$", v.n)); err != nil {
			return err
		}
	}
	if r.PromptCacheKey != nil && utf8.RuneCountInString(*r.PromptCacheKey) > 64 {
		return validationError(memberPath("$", "prompt_cache_key"), "must contain at most 64 characters")
	}
	if len(r.Metadata) > 16 {
		return validationError(memberPath("$", "metadata"), "must contain at most 16 members")
	}
	keys := make([]string, 0, len(r.Metadata))
	for k := range r.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := memberPath(memberPath("$", "metadata"), k)
		if err := stringBound(k, p); err != nil {
			return err
		}
		if err := stringBound(r.Metadata[k], p); err != nil {
			return err
		}
		if utf8.RuneCountInString(k) > 64 {
			return validationError(p, "key must contain at most 64 characters")
		}
		if utf8.RuneCountInString(r.Metadata[k]) > 512 {
			return validationError(p, "value must contain at most 512 characters")
		}
	}
	if err := stringPointer(r.Caching, memberPath("$", "caching")); err != nil {
		return err
	}
	if r.Caching != nil && *r.Caching != "auto" {
		return validationError(memberPath("$", "caching"), "must be auto")
	}
	if r.CacheAnchorItems != nil && *r.CacheAnchorItems < 0 {
		return validationError(memberPath("$", "cache_anchor_items"), "must be non-negative")
	}
	if err := stringPointer(r.CacheTTL, memberPath("$", "cache_ttl")); err != nil {
		return err
	}
	if r.CacheTTL != nil {
		if *r.CacheTTL != "5m" && *r.CacheTTL != "1h" {
			return validationError(memberPath("$", "cache_ttl"), "must be 5m or 1h")
		}
		if r.Caching == nil || *r.Caching != "auto" {
			return validationError(memberPath("$", "cache_ttl"), "requires caching auto")
		}
	}
	return nil
}

func validateResponsesBuiltInToolsRequest(r ResponsesBuiltInToolsRequest) *ValidationError {
	if err := validateResponsesRequest(r.Request); err != nil {
		return err
	}
	if len(r.Request.Tools)+len(r.Tools) > maxResponseMembers {
		return validationError(memberPath("$", "tools"), "must contain at most 10000 items")
	}
	for i, tool := range r.Tools {
		path := indexPath(memberPath("$", "tools"), len(r.Request.Tools)+i)
		if nilValue(tool) {
			return validationError(path, "must be a non-nil built-in tool")
		}
		switch tool.(type) {
		case ResponseWebSearchTool, *ResponseWebSearchTool:
		default:
			return validationError(path, "built-in tool type is unsupported")
		}
	}
	return nil
}

func validModelID(s string) bool { i := strings.IndexByte(s, '/'); return i > 0 && i < len(s)-1 }
func nilValue(v any) bool {
	if v == nil {
		return true
	}
	x := reflect.ValueOf(v)
	return (x.Kind() == reflect.Pointer || x.Kind() == reflect.Slice || x.Kind() == reflect.Map || x.Kind() == reflect.Interface) && x.IsNil()
}
func validateResponseInput(input ResponseInput, path string) *ValidationError {
	switch v := input.(type) {
	case ResponseTextInput:
		if v == "" {
			return validationError(path, "must be nonempty")
		}
		return stringBound(string(v), path)
	case ResponseItemsInput:
		if v == nil {
			return validationError(path, "required")
		}
		if len(v) > maxResponseMembers {
			return validationError(path, "must contain at most 10000 items")
		}
		for i, item := range v {
			p := indexPath(path, i)
			if nilValue(item) {
				return validationError(p, "must be a non-nil input item")
			}
			switch x := item.(type) {
			case ResponseMessage:
				if err := stringBound(x.Role, memberPath(p, "role")); err != nil {
					return err
				}
				if x.Role != "user" && x.Role != "assistant" && x.Role != "system" && x.Role != "developer" {
					return validationError(memberPath(p, "role"), "unsupported role")
				}
				if err := stringBound(x.Content, memberPath(p, "content")); err != nil {
					return err
				}
			case ResponseFunctionCall:
				if x.CallID == "" || x.Name == "" || x.Arguments == "" {
					return validationError(p, "call_id, name, and arguments must be nonempty")
				}
				for n, s := range map[string]string{"id": x.ID, "call_id": x.CallID, "name": x.Name, "arguments": x.Arguments} {
					if err := stringBound(s, memberPath(p, n)); err != nil {
						return err
					}
				}
			case ResponseFunctionCallOutput:
				if x.CallID == "" {
					return validationError(memberPath(p, "call_id"), "must be nonempty")
				}
				if err := stringBound(x.CallID, memberPath(p, "call_id")); err != nil {
					return err
				}
				if err := stringBound(x.Output, memberPath(p, "output")); err != nil {
					return err
				}
			default:
				return validationError(p, "input item type is unsupported")
			}
		}
		return nil
	default:
		return validationError(path, "input type is unsupported")
	}
}
func boundedFloat(v *float64, min, max float64, path string) *ValidationError {
	if err := finiteFloat(v, path); err != nil {
		return err
	}
	if v != nil && (*v < min || *v > max) {
		return validationError(path, "out of range")
	}
	return nil
}
func finiteFloat(v *float64, path string) *ValidationError {
	if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
		return validationError(path, "number must be finite")
	}
	return nil
}
func stringPointer(v *string, path string) *ValidationError {
	if v == nil {
		return nil
	}
	return stringBound(*v, path)
}
func stringBound(v, path string) *ValidationError {
	if len(v) > maxResponseValueBytes {
		return validationError(path, "must contain at most 1 MiB")
	}
	return nil
}
func validateBoundedJSON(v any, path string) *ValidationError {
	return (&boundedJSONValidator{active: map[jsonVisit]bool{}}).walk(reflect.ValueOf(v), path, 0, 0)
}

type boundedJSONValidator struct{ active map[jsonVisit]bool }

func (b *boundedJSONValidator) walk(v reflect.Value, path string, depth, indirections int) *ValidationError {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		if indirections >= maxResponseJSONDepth {
			return validationError(path, "maximum depth is 64")
		}
		return b.walk(v.Elem(), path, depth, indirections+1)
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		visit := jsonVisit{typ: v.Type(), ptr: unsafePointer(uintptr(v.UnsafePointer()))}
		if b.active[visit] {
			return validationError(path, "cycle detected")
		}
		if indirections >= maxResponseJSONDepth {
			return validationError(path, "maximum depth is 64")
		}
		b.active[visit] = true
		defer delete(b.active, visit)
		return b.walk(v.Elem(), path, depth, indirections+1)
	}
	switch v.Kind() {
	case reflect.Bool:
	case reflect.String:
		return stringBound(v.String(), path)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
			return validationError(path, "number must be finite")
		}
	case reflect.Map:
		depth++
		if depth > maxResponseJSONDepth {
			return validationError(path, "maximum depth is 64")
		}
		if v.Type().Key().Kind() != reflect.String {
			return validationError(path, "map keys must be strings")
		}
		if v.Len() > maxResponseMembers {
			return validationError(path, "must contain at most 10000 members")
		}
		visit := jsonVisit{typ: v.Type(), ptr: unsafePointer(uintptr(v.UnsafePointer()))}
		if b.active[visit] {
			return validationError(path, "cycle detected")
		}
		b.active[visit] = true
		defer delete(b.active, visit)
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, k := range keys {
			if err := stringBound(k.String(), memberPath(path, k.String())); err != nil {
				return err
			}
			if err := b.walk(v.MapIndex(k), memberPath(path, k.String()), depth, 0); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		depth++
		if depth > maxResponseJSONDepth {
			return validationError(path, "maximum depth is 64")
		}
		if v.Len() > maxResponseMembers {
			return validationError(path, "must contain at most 10000 members")
		}
		if v.Kind() == reflect.Slice && !v.IsNil() {
			visit := jsonVisit{typ: v.Type(), ptr: unsafePointer(uintptr(v.UnsafePointer()))}
			if b.active[visit] {
				return validationError(path, "cycle detected")
			}
			b.active[visit] = true
			defer delete(b.active, visit)
		}
		for i := range v.Len() {
			if err := b.walk(v.Index(i), indexPath(path, i), depth, 0); err != nil {
				return err
			}
		}
	default:
		return validationError(path, "must be JSON-compatible")
	}
	return nil
}
