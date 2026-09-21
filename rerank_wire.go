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
	"strconv"
)

type rerankFailure struct {
	path, reason string
	cause        error
}

func decodeRerankResult(modelID string, request *preparedRerankRequest, raw rawProviderResponse) (*RerankResult, error) {
	body := append([]byte(nil), raw.body...)
	invalid := func(f *rerankFailure) (*RerankResult, error) {
		return nil, &ResponseValidationError{cause: f.cause, statusCode: http.StatusOK, path: f.path, reason: f.reason, rawResponseBody: body}
	}
	if f := validateRerankJSON(body); f != nil {
		return invalid(f)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return invalid(&rerankFailure{"$", "malformed JSON", err})
	}
	for _, k := range sortedRawKeys(top) {
		if k != "ranking" && k != "warnings" && k != "providerMetadata" {
			return invalid(&rerankFailure{memberPath("$", k), "unknown field", nil})
		}
	}
	rawRanking, ok := top["ranking"]
	if !ok {
		return invalid(&rerankFailure{memberPath("$", "ranking"), "required", nil})
	}
	if bytes.Equal(bytes.TrimSpace(rawRanking), []byte("null")) {
		return invalid(&rerankFailure{memberPath("$", "ranking"), "must be an array", nil})
	}
	var ranking []json.RawMessage
	if json.Unmarshal(rawRanking, &ranking) != nil {
		return invalid(&rerankFailure{memberPath("$", "ranking"), "must be an array", nil})
	}
	count := len(request.texts)
	if request.objects != nil {
		count = len(request.objects)
	}
	if len(ranking) > count {
		return invalid(&rerankFailure{memberPath("$", "ranking"), fmt.Sprintf("must contain at most %d items", count), nil})
	}
	if request.topN != nil && len(ranking) > *request.topN {
		return invalid(&rerankFailure{memberPath("$", "ranking"), fmt.Sprintf("must contain at most topN (%d) items", *request.topN), nil})
	}
	results := make([]RerankItem, len(ranking))
	seen := make(map[int]bool, len(ranking))
	for i, rawItem := range ranking {
		base := indexPath(memberPath("$", "ranking"), i)
		if bytes.Equal(bytes.TrimSpace(rawItem), []byte("null")) {
			return invalid(&rerankFailure{base, "must be an object", nil})
		}
		var item map[string]json.RawMessage
		if json.Unmarshal(rawItem, &item) != nil {
			return invalid(&rerankFailure{base, "must be an object", nil})
		}
		for _, k := range sortedRawKeys(item) {
			if k != "index" && k != "relevanceScore" {
				return invalid(&rerankFailure{memberPath(base, k), "unknown field", nil})
			}
		}
		index, f := decodeRerankIndex(item["index"], memberPath(base, "index"))
		if f != nil {
			return invalid(f)
		}
		if index < 0 || index >= count {
			return invalid(&rerankFailure{memberPath(base, "index"), fmt.Sprintf("must be in [0,%d)", count), nil})
		}
		if seen[index] {
			return invalid(&rerankFailure{memberPath(base, "index"), "must be unique", nil})
		}
		seen[index] = true
		score, f := decodeRerankScore(item["relevanceScore"], memberPath(base, "relevanceScore"))
		if f != nil {
			return invalid(f)
		}
		var document RerankDocument
		if request.texts != nil {
			document = RerankText{Text: request.texts[index]}
		} else {
			document = RerankJSON{Value: append(json.RawMessage(nil), request.objects[index]...)}
		}
		results[i] = RerankItem{OriginalIndex: index, Score: score, Document: document}
	}
	warnings, ef := decodeProviderWarnings(top["warnings"])
	if ef != nil {
		return invalid(&rerankFailure{ef.path, ef.reason, ef.cause})
	}
	metadata, ef := decodeEmbeddingMetadata(top["providerMetadata"])
	if ef != nil {
		return invalid(&rerankFailure{ef.path, ef.reason, ef.cause})
	}
	prefix := body
	if len(prefix) > maxDiagnosticBodyBytes {
		prefix = prefix[:maxDiagnosticBodyBytes]
	}
	return &RerankResult{Results: results, Warnings: warnings, ProviderMetadata: metadata, Response: ResponseMetadata{ModelID: modelID, Headers: nonNilHeaderClone(raw.headers), Body: nonNilByteClone(prefix)}}, nil
}

func decodeRerankIndex(raw json.RawMessage, path string) (int, *rerankFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, &rerankFailure{path, "required", nil}
	}
	var n json.Number
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return 0, &rerankFailure{path, "must be an integer", nil}
	}
	var ok bool
	n, ok = v.(json.Number)
	if !ok {
		return 0, &rerankFailure{path, "must be an integer", nil}
	}
	r, ok := new(big.Rat).SetString(n.String())
	if !ok || !r.IsInt() {
		return 0, &rerankFailure{path, "must be an integer", nil}
	}
	i := r.Num()
	bits := strconv.IntSize
	if !i.IsInt64() {
		return 0, &rerankFailure{path, "must be representable as int", nil}
	}
	i64 := i.Int64()
	if bits == 32 && (i64 < math.MinInt32 || i64 > math.MaxInt32) {
		return 0, &rerankFailure{path, "must be representable as int", nil}
	}
	return int(i64), nil
}
func decodeRerankScore(raw json.RawMessage, path string) (float64, *rerankFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, &rerankFailure{path, "required", nil}
	}
	var n json.Number
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return 0, &rerankFailure{path, "must be a finite number", nil}
	}
	var ok bool
	n, ok = v.(json.Number)
	if !ok {
		return 0, &rerankFailure{path, "must be a finite number", nil}
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, &rerankFailure{path, "must be a finite number", nil}
	}
	return f, nil
}
func validateRerankJSON(body []byte) *rerankFailure {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	tok, err := d.Token()
	if err != nil {
		return &rerankFailure{"$", "malformed JSON", err}
	}
	if tok != json.Delim('{') {
		return &rerankFailure{"$", "must be an object", nil}
	}
	seen := map[string]bool{}
	for d.More() {
		keyToken, err := d.Token()
		if err != nil {
			return &rerankFailure{"$", "malformed JSON", err}
		}
		key, ok := keyToken.(string)
		if !ok {
			return &rerankFailure{"$", "malformed JSON", nil}
		}
		path := memberPath("$", key)
		if seen[key] {
			return &rerankFailure{path, "duplicate field", nil}
		}
		seen[key] = true
		if key == "ranking" {
			start, err := d.Token()
			if err != nil {
				return &rerankFailure{"$", "malformed JSON", err}
			}
			if start != json.Delim('[') {
				return &rerankFailure{path, "must be an array", nil}
			}
			for i := 0; d.More(); i++ {
				if i >= maxCollectionItems {
					return &rerankFailure{path, "array exceeds 4096 members", nil}
				}
				if f := scanEmbeddingValue(d, indexPath(path, i), 2, false, false); f != nil {
					return &rerankFailure{f.path, f.reason, f.cause}
				}
			}
			if _, err = d.Token(); err != nil {
				return &rerankFailure{"$", "malformed JSON", err}
			}
		} else {
			limit, children := key == "warnings" || key == "providerMetadata", key == "providerMetadata"
			if f := scanEmbeddingValue(d, path, 2, limit, children); f != nil {
				return &rerankFailure{f.path, f.reason, f.cause}
			}
		}
	}
	if _, err = d.Token(); err != nil {
		return &rerankFailure{"$", "malformed JSON", err}
	}
	if _, err = d.Token(); err == nil {
		return &rerankFailure{"$", "trailing JSON value", nil}
	} else if !errors.Is(err, io.EOF) {
		return &rerankFailure{"$", "malformed JSON", err}
	}
	return nil
}
