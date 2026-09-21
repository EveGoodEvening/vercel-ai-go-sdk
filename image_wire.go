package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

type imageFailure struct {
	path, reason string
	cause        error
}

func decodeImageResult(modelID string, raw rawProviderResponse) (*ImageResult, error) {
	body := raw.body
	prefix := body
	if len(prefix) > maxDiagnosticBodyBytes {
		prefix = prefix[:maxDiagnosticBodyBytes]
	}
	retained := nonNilByteClone(prefix)
	invalid := func(f *imageFailure) (*ImageResult, error) {
		return nil, &ResponseValidationError{cause: f.cause, statusCode: http.StatusOK, path: f.path, reason: f.reason, rawResponseBody: append([]byte(nil), retained...)}
	}
	if f := scanImageJSON(body); f != nil {
		return invalid(f)
	}
	var top map[string]json.RawMessage
	if e := json.Unmarshal(body, &top); e != nil {
		return invalid(&imageFailure{"$", "malformed JSON", e})
	}
	allowed := map[string]bool{"images": true, "isRetryable": true, "warnings": true, "providerMetadata": true, "usage": true}
	for _, k := range sortedRawKeys(top) {
		if !allowed[k] {
			return invalid(&imageFailure{memberPath("$", k), "unknown field", nil})
		}
	}
	rawImages, ok := top["images"]
	if !ok {
		return invalid(&imageFailure{memberPath("$", "images"), "required", nil})
	}
	if bytes.Equal(bytes.TrimSpace(rawImages), []byte("null")) {
		return invalid(&imageFailure{memberPath("$", "images"), "must be an array", nil})
	}
	var encoded []string
	if json.Unmarshal(rawImages, &encoded) != nil {
		return invalid(&imageFailure{memberPath("$", "images"), "must be an array of strings", nil})
	}
	if len(encoded) > 16 {
		return invalid(&imageFailure{memberPath("$", "images"), "must contain at most 16 images", nil})
	}
	images := make([][]byte, len(encoded))
	total := 0
	for i, s := range encoded {
		p := indexPath(memberPath("$", "images"), i)
		maxEnc := base64.StdEncoding.EncodedLen(maxImageDecodedBytes)
		if len(s) > maxEnc {
			return invalid(&imageFailure{p, "decoded image exceeds 16777216 bytes", nil})
		}
		if !strictPaddedBase64(s) {
			return invalid(&imageFailure{p, "must be strict standard padded base64", nil})
		}
		var decoded bytes.Buffer
		decoded.Grow(base64.StdEncoding.DecodedLen(len(s)))
		dec := base64.NewDecoder(base64.StdEncoding.Strict(), strings.NewReader(s))
		written, e := io.Copy(&decoded, io.LimitReader(dec, int64(maxImageDecodedBytes)+1))
		if e != nil {
			return invalid(&imageFailure{p, "must be strict standard padded base64", e})
		}
		if written > maxImageDecodedBytes {
			return invalid(&imageFailure{p, "decoded image exceeds 16777216 bytes", nil})
		}
		if total > maxImageAggregateDecodedBytes-int(written) {
			return invalid(&imageFailure{p, "aggregate decoded images exceed 67108864 bytes", nil})
		}
		total += int(written)
		images[i] = decoded.Bytes()
	}
	var retryable *bool
	if v, ok := top["isRetryable"]; ok {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return invalid(&imageFailure{memberPath("$", "isRetryable"), "must be a boolean", nil})
		}
		var x bool
		if json.Unmarshal(v, &x) != nil {
			return invalid(&imageFailure{memberPath("$", "isRetryable"), "must be a boolean", nil})
		}
		retryable = &x
	}
	if warningFailure := validateImageWarnings(top["warnings"]); warningFailure != nil {
		return invalid(warningFailure)
	}
	warnings, warningFailure := decodeProviderWarnings(top["warnings"])
	if warningFailure != nil {
		return invalid(&imageFailure{warningFailure.path, warningFailure.reason, warningFailure.cause})
	}
	metadata, metadataFailure := decodeImageMetadata(top["providerMetadata"])
	if metadataFailure != nil {
		return invalid(metadataFailure)
	}
	usage, usageFailure := decodeImageUsage(top["usage"])
	if usageFailure != nil {
		return invalid(usageFailure)
	}
	return &ImageResult{Images: images, Retryable: retryable, Warnings: warnings, ProviderMetadata: metadata, Response: ResponseMetadata{ModelID: modelID, Headers: nonNilHeaderClone(raw.headers), Body: retained}, Usage: usage}, nil
}
func strictPaddedBase64(value string) bool {
	if len(value)%4 != 0 {
		return false
	}
	padding := 0
	if len(value) > 0 && value[len(value)-1] == '=' {
		padding++
		if len(value) > 1 && value[len(value)-2] == '=' {
			padding++
		}
	}
	for i := 0; i < len(value)-padding; i++ {
		c := value[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/') {
			return false
		}
	}
	for i := len(value) - padding; i < len(value); i++ {
		if value[i] != '=' {
			return false
		}
	}
	return padding <= 2
}
func decodeImageUsage(raw json.RawMessage) (*ImageUsage, *imageFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, &imageFailure{memberPath("$", "usage"), "must be an object", nil}
	}
	allowed := map[string]bool{"inputTokens": true, "outputTokens": true, "totalTokens": true}
	for _, k := range sortedRawKeys(obj) {
		if !allowed[k] {
			return nil, &imageFailure{memberPath(memberPath("$", "usage"), k), "unknown field", nil}
		}
	}
	out := &ImageUsage{}
	fields := []struct {
		name string
		dst  **float64
	}{{"inputTokens", &out.InputTokens}, {"outputTokens", &out.OutputTokens}, {"totalTokens", &out.TotalTokens}}
	for _, f := range fields {
		v, ok := obj[f.name]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			continue
		}
		var n float64
		if json.Unmarshal(v, &n) != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return nil, &imageFailure{memberPath(memberPath("$", "usage"), f.name), "must be a finite number", nil}
		}
		*f.dst = &n
	}
	return out, nil
}
func validateImageWarnings(raw json.RawMessage) *imageFailure {
	if len(raw) == 0 {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return &imageFailure{memberPath("$", "warnings"), "must be an array", nil}
	}
	var items []map[string]json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return &imageFailure{memberPath("$", "warnings"), "must be an array", nil}
	}
	for i, item := range items {
		base := indexPath(memberPath("$", "warnings"), i)
		typRaw, ok := item["type"]
		var typ string
		if !ok || bytes.Equal(bytes.TrimSpace(typRaw), []byte("null")) || json.Unmarshal(typRaw, &typ) != nil {
			return &imageFailure{memberPath(base, "type"), "required string", nil}
		}
		required := []string{}
		switch ProviderWarningType(typ) {
		case ProviderWarningUnsupported, ProviderWarningCompatibility:
			required = []string{"feature"}
		case ProviderWarningDeprecated:
			required = []string{"setting", "message"}
		case ProviderWarningOther:
			required = []string{"message"}
		}
		for _, key := range required {
			v, ok := item[key]
			var s string
			if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &s) != nil {
				return &imageFailure{memberPath(base, key), "required string", nil}
			}
		}
		if v, ok := item["details"]; ok && bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return &imageFailure{memberPath(base, "details"), "must be a string", nil}
		}
	}
	return nil
}
func decodeImageMetadata(raw json.RawMessage) (map[string]json.RawMessage, *imageFailure) {
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &imageFailure{memberPath("$", "providerMetadata"), "must be an object", nil}
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, &imageFailure{memberPath("$", "providerMetadata"), "must be an object", nil}
	}
	out := make(map[string]json.RawMessage, len(obj))
	for k, v := range obj {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return nil, &imageFailure{memberPath(memberPath("$", "providerMetadata"), k), "must be an object", nil}
		}
		var x map[string]json.RawMessage
		if json.Unmarshal(v, &x) != nil {
			return nil, &imageFailure{memberPath(memberPath("$", "providerMetadata"), k), "must be an object", nil}
		}
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out, nil
}

func scanImageJSON(body []byte) *imageFailure {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if f := scanImageValue(d, "$", 1, false); f != nil {
		return f
	}
	if _, e := d.Token(); e == nil {
		return &imageFailure{"$", "trailing JSON value", nil}
	} else if !errors.Is(e, io.EOF) {
		return &imageFailure{"$", "malformed JSON", e}
	}
	return nil
}
func scanImageValue(d *json.Decoder, path string, depth int, imageString bool) *imageFailure {
	if depth > maxJSONDepth {
		return &imageFailure{path, "maximum depth is 64", nil}
	}
	tok, err := d.Token()
	if err != nil {
		return &imageFailure{path, "malformed JSON", err}
	}
	switch value := tok.(type) {
	case string:
		if !imageString && len(value) > maxStringBytes {
			return &imageFailure{path, "must not exceed 1048576 bytes", nil}
		}
	case json.Delim:
		switch value {
		case '{':
			seen := map[string]bool{}
			count := 0
			for d.More() {
				keyToken, err := d.Token()
				if err != nil {
					return &imageFailure{path, "malformed JSON", err}
				}
				key, ok := keyToken.(string)
				if !ok {
					return &imageFailure{path, "malformed JSON", nil}
				}
				keyPath := memberPath(path, key)
				if seen[key] {
					return &imageFailure{keyPath, "duplicate field", nil}
				}
				seen[key] = true
				count++
				if count > maxCollectionItems {
					return &imageFailure{path, "must contain at most 4096 members", nil}
				}
				if failure := scanImageValue(d, keyPath, depth+1, false); failure != nil {
					return failure
				}
			}
			if _, err = d.Token(); err != nil {
				return &imageFailure{path, "malformed JSON", err}
			}
		case '[':
			count := 0
			for d.More() {
				itemPath := indexPath(path, count)
				exempt := path == memberPath("$", "images")
				if failure := scanImageValue(d, itemPath, depth+1, exempt); failure != nil {
					return failure
				}
				count++
				if count > maxCollectionItems {
					return &imageFailure{path, "must contain at most 4096 items", nil}
				}
			}
			if _, err = d.Token(); err != nil {
				return &imageFailure{path, "malformed JSON", err}
			}
		default:
			return &imageFailure{path, fmt.Sprintf("unexpected delimiter %q", value), nil}
		}
	}
	return nil
}
