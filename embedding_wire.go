package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"sort"
)

type embeddingRequestWire struct {
	Values          []string       `json:"values"`
	ProviderOptions map[string]any `json:"providerOptions,omitempty"`
}

type embeddingFailure struct {
	path, reason string
	cause        error
}

func decodeEmbeddingResult(modelID string, valueCount int, raw rawProviderResponse) (*EmbeddingResult, error) {
	body := append([]byte(nil), raw.body...)
	invalid := func(f *embeddingFailure) (*EmbeddingResult, error) {
		return nil, &ResponseValidationError{cause: f.cause, statusCode: http.StatusOK, path: f.path, reason: f.reason, rawResponseBody: body}
	}
	if f := validateEmbeddingJSON(body); f != nil {
		return invalid(f)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return invalid(&embeddingFailure{"$", "malformed JSON", err})
	}
	for _, key := range sortedRawKeys(top) {
		if key != "embeddings" && key != "usage" && key != "warnings" && key != "providerMetadata" {
			return invalid(&embeddingFailure{memberPath("$", key), "unknown field", nil})
		}
	}
	rawEmb, ok := top["embeddings"]
	if !ok {
		return invalid(&embeddingFailure{memberPath("$", "embeddings"), "required", nil})
	}
	if bytes.Equal(bytes.TrimSpace(rawEmb), []byte("null")) {
		return invalid(&embeddingFailure{memberPath("$", "embeddings"), "must be an array", nil})
	}
	var vectors []json.RawMessage
	if err := json.Unmarshal(rawEmb, &vectors); err != nil {
		return invalid(&embeddingFailure{memberPath("$", "embeddings"), "must be an array", nil})
	}
	if len(vectors) != valueCount {
		return invalid(&embeddingFailure{memberPath("$", "embeddings"), fmt.Sprintf("must contain exactly %d vectors", valueCount), nil})
	}
	embeddings := make([][]float64, len(vectors))
	total := 0
	for i, rv := range vectors {
		var vector []float64
		if bytes.Equal(bytes.TrimSpace(rv), []byte("null")) || json.Unmarshal(rv, &vector) != nil {
			return invalid(&embeddingFailure{indexPath(memberPath("$", "embeddings"), i), "must be an array", nil})
		}
		if len(vector) < 1 || len(vector) > 65536 {
			return invalid(&embeddingFailure{indexPath(memberPath("$", "embeddings"), i), "must contain 1..65536 numbers", nil})
		}
		total += len(vector)
		if total > 4194304 {
			return invalid(&embeddingFailure{memberPath("$", "embeddings"), "aggregate elements exceed 4194304", nil})
		}
		for j, n := range vector {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return invalid(&embeddingFailure{indexPath(indexPath(memberPath("$", "embeddings"), i), j), "must be finite", nil})
			}
		}
		embeddings[i] = append([]float64(nil), vector...)
	}
	usage, f := decodeEmbeddingUsage(top["usage"])
	if f != nil {
		return invalid(f)
	}
	warnings, f := decodeProviderWarnings(top["warnings"])
	if f != nil {
		return invalid(f)
	}
	metadata, f := decodeEmbeddingMetadata(top["providerMetadata"])
	if f != nil {
		return invalid(f)
	}
	prefix := body
	if len(prefix) > maxDiagnosticBodyBytes {
		prefix = prefix[:maxDiagnosticBodyBytes]
	}
	return &EmbeddingResult{Embeddings: embeddings, Usage: usage, Warnings: warnings, ProviderMetadata: metadata, Response: ResponseMetadata{ModelID: modelID, Headers: nonNilHeaderClone(raw.headers), Body: nonNilByteClone(prefix)}}, nil
}

func decodeEmbeddingUsage(raw json.RawMessage) (*EmbeddingUsage, *embeddingFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, &embeddingFailure{memberPath("$", "usage"), "must be an object", nil}
	}
	for _, k := range sortedRawKeys(obj) {
		if k != "tokens" {
			return nil, &embeddingFailure{memberPath(memberPath("$", "usage"), k), "unknown field", nil}
		}
	}
	v, ok := obj["tokens"]
	path := memberPath(memberPath("$", "usage"), "tokens")
	if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return nil, &embeddingFailure{path, "required", nil}
	}
	decoder := json.NewDecoder(bytes.NewReader(v))
	decoder.UseNumber()
	var token any
	if decoder.Decode(&token) != nil {
		return nil, &embeddingFailure{path, "must be an integer", nil}
	}
	number, ok := token.(json.Number)
	if !ok {
		return nil, &embeddingFailure{path, "must be an integer", nil}
	}
	rational, ok := new(big.Rat).SetString(number.String())
	if !ok || !rational.IsInt() {
		return nil, &embeddingFailure{path, "must be an integer", nil}
	}
	integer := rational.Num()
	if !integer.IsInt64() {
		return nil, &embeddingFailure{path, "must be a signed 64-bit integer", nil}
	}
	value := integer.Int64()
	return &EmbeddingUsage{Tokens: &value}, nil
}
func decodeProviderWarnings(raw json.RawMessage) ([]ProviderWarning, *embeddingFailure) {
	if len(raw) == 0 {
		return make([]ProviderWarning, 0), nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &embeddingFailure{memberPath("$", "warnings"), "must be an array", nil}
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil, &embeddingFailure{memberPath("$", "warnings"), "must be an array", nil}
	}
	out := make([]ProviderWarning, len(items))
	for i, obj := range items {
		base := indexPath(memberPath("$", "warnings"), i)
		var typ string
		if v, ok := obj["type"]; !ok || json.Unmarshal(v, &typ) != nil {
			return nil, &embeddingFailure{memberPath(base, "type"), "required string", nil}
		}
		allowed := map[string]bool{"type": true}
		w := ProviderWarning{Type: ProviderWarningType(typ)}
		req := func(key string, dst *string) *embeddingFailure {
			v, ok := obj[key]
			if !ok || json.Unmarshal(v, dst) != nil {
				return &embeddingFailure{memberPath(base, key), "required string", nil}
			}
			allowed[key] = true
			return nil
		}
		switch w.Type {
		case ProviderWarningUnsupported, ProviderWarningCompatibility:
			if f := req("feature", &w.Feature); f != nil {
				return nil, f
			}
			allowed["details"] = true
			if v, ok := obj["details"]; ok {
				var s string
				if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &s) != nil {
					return nil, &embeddingFailure{memberPath(base, "details"), "must be a string", nil}
				}
				w.Details = &s
			}
		case ProviderWarningDeprecated:
			if f := req("setting", &w.Setting); f != nil {
				return nil, f
			}
			if f := req("message", &w.Message); f != nil {
				return nil, f
			}
		case ProviderWarningOther:
			if f := req("message", &w.Message); f != nil {
				return nil, f
			}
		default:
			return nil, &embeddingFailure{memberPath(base, "type"), "unsupported warning type", nil}
		}
		for _, k := range sortedRawKeys(obj) {
			if !allowed[k] {
				return nil, &embeddingFailure{memberPath(base, k), "unknown field", nil}
			}
		}
		out[i] = w
	}
	return out, nil
}

func decodeEmbeddingMetadata(raw json.RawMessage) (map[string]json.RawMessage, *embeddingFailure) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &embeddingFailure{memberPath("$", "providerMetadata"), "must be an object", nil}
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, &embeddingFailure{memberPath("$", "providerMetadata"), "must be an object", nil}
	}
	out := make(map[string]json.RawMessage, len(obj))
	for k, v := range obj {
		var extension map[string]json.RawMessage
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &extension) != nil {
			return nil, &embeddingFailure{memberPath(memberPath("$", "providerMetadata"), k), "must be an object", nil}
		}
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out, nil
}

func sortedRawKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func validateEmbeddingJSON(body []byte) *embeddingFailure {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if f := scanEmbeddingValue(d, "$", 1, false, false); f != nil {
		return f
	}
	if _, err := d.Token(); err == nil {
		return &embeddingFailure{"$", "trailing JSON value", nil}
	} else if !errors.Is(err, io.EOF) {
		return &embeddingFailure{"$", "malformed JSON", err}
	}
	return nil
}
func scanEmbeddingValue(d *json.Decoder, path string, depth int, limitCollection, limitChildren bool) *embeddingFailure {
	if depth > maxJSONDepth {
		return &embeddingFailure{path, "maximum depth is 64", nil}
	}
	tok, err := d.Token()
	if err != nil {
		return &embeddingFailure{"$", "malformed JSON", err}
	}
	if text, ok := tok.(string); ok && len(text) > maxStringBytes {
		return &embeddingFailure{path, "string exceeds 1 MiB", nil}
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		count := 0
		for d.More() {
			limit := 10000
			reason := "object exceeds 10000 members"
			if limitCollection {
				limit = maxCollectionItems
				reason = "object exceeds 4096 members"
			}
			if count >= limit {
				return &embeddingFailure{path, reason, nil}
			}
			kt, e := d.Token()
			if e != nil {
				return &embeddingFailure{"$", "malformed JSON", e}
			}
			k, ok := kt.(string)
			if !ok {
				return &embeddingFailure{"$", "malformed JSON", nil}
			}
			if len(k) > maxStringBytes {
				return &embeddingFailure{path, "object key exceeds 1 MiB", nil}
			}
			p := memberPath(path, k)
			if seen[k] {
				return &embeddingFailure{p, "duplicate field", nil}
			}
			seen[k] = true
			count++
			childLimitCollection, childLimitChildren := limitChildren, limitChildren
			if path == "$" {
				switch k {
				case "embeddings", "warnings":
					childLimitCollection = true
					childLimitChildren = false
				case "providerMetadata":
					childLimitCollection = true
					childLimitChildren = true
				}
			}
			if f := scanEmbeddingValue(d, p, depth+1, childLimitCollection, childLimitChildren); f != nil {
				return f
			}
		}
		if _, e := d.Token(); e != nil {
			return &embeddingFailure{"$", "malformed JSON", e}
		}
	case '[':
		for i := 0; d.More(); i++ {
			if limitCollection && i >= maxCollectionItems {
				return &embeddingFailure{path, "array exceeds 4096 members", nil}
			}
			if f := scanEmbeddingValue(d, indexPath(path, i), depth+1, limitChildren, limitChildren); f != nil {
				return f
			}
		}
		if _, e := d.Token(); e != nil {
			return &embeddingFailure{"$", "malformed JSON", e}
		}
	default:
		return &embeddingFailure{"$", "malformed JSON", nil}
	}
	return nil
}
