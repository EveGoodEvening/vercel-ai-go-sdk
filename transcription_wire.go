package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"
)

type transcriptionFailure struct {
	path, reason string
	cause        error
}

func decodeTranscriptionResult(ctx context.Context, modelID string, raw rawProviderResponse) (*TranscriptionResult, error) {
	canceled := func() (*TranscriptionResult, error) {
		return nil, &TransportError{operation: "read response body", cause: ctx.Err()}
	}
	if ctx.Err() != nil {
		return canceled()
	}
	prefix := raw.body
	if len(prefix) > maxDiagnosticBodyBytes {
		prefix = prefix[:maxDiagnosticBodyBytes]
	}
	retained := nonNilByteClone(prefix)
	invalid := func(f *transcriptionFailure) (*TranscriptionResult, error) {
		return nil, &ResponseValidationError{cause: f.cause, statusCode: http.StatusOK, path: f.path, reason: f.reason, bodyTruncated: raw.bodyTruncated, rawResponseBody: append([]byte(nil), retained...)}
	}
	if raw.bodyTruncated {
		return invalid(&transcriptionFailure{"$", "response body exceeds 16777216 bytes", raw.bodyErr})
	}
	if raw.bodyErr != nil {
		return nil, &TransportError{operation: "read response body", cause: raw.bodyErr}
	}
	if ctx.Err() != nil {
		return canceled()
	}
	if !utf8.Valid(raw.body) {
		return invalid(&transcriptionFailure{"$", "malformed JSON", nil})
	}
	if f := scanSpeechJSON(ctx, raw.body); f != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(&transcriptionFailure{f.path, f.reason, f.cause})
	}
	var top map[string]json.RawMessage
	if err := decodeSpeechJSON(ctx, raw.body, &top); err != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(&transcriptionFailure{"$", "malformed JSON", err})
	}
	for _, key := range sortedRawKeys(top) {
		switch key {
		case "text", "segments", "language", "durationInSeconds", "warnings", "providerMetadata":
		default:
			return invalid(&transcriptionFailure{memberPath("$", key), "unknown field", nil})
		}
	}
	text, f := transcriptionRequiredString(ctx, top["text"], memberPath("$", "text"), maxStringBytes)
	if f != nil {
		return invalid(f)
	}
	segments, f := decodeTranscriptionSegments(ctx, top["segments"])
	if f != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(f)
	}
	language, f := transcriptionNullableString(ctx, top["language"], memberPath("$", "language"), 255)
	if f != nil {
		return invalid(f)
	}
	duration, f := transcriptionNullableFloat(ctx, top["durationInSeconds"], memberPath("$", "durationInSeconds"))
	if f != nil {
		return invalid(f)
	}
	warnings, wf := decodeTranscriptionWarnings(ctx, top["warnings"])
	if wf != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(wf)
	}
	metadata, mf := decodeTranscriptionMetadata(ctx, top["providerMetadata"])
	if mf != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(mf)
	}
	if ctx.Err() != nil {
		return canceled()
	}
	return &TranscriptionResult{Text: text, Segments: segments, Language: language, DurationSeconds: duration, Warnings: warnings, ProviderMetadata: metadata, Response: ResponseMetadata{ModelID: modelID, Headers: nonNilHeaderClone(raw.headers), Body: retained}}, nil
}

func transcriptionRequiredString(ctx context.Context, raw json.RawMessage, path string, limit int) (string, *transcriptionFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", &transcriptionFailure{path, "required string", nil}
	}
	var value string
	if err := decodeSpeechJSON(ctx, raw, &value); err != nil {
		return "", &transcriptionFailure{path, "required string", err}
	}
	if !utf8.ValidString(value) {
		return "", &transcriptionFailure{path, "must be valid UTF-8", nil}
	}
	if len(value) > limit {
		return "", &transcriptionFailure{path, transcriptionStringLimitReason(limit), nil}
	}
	return value, nil
}

func transcriptionNullableString(ctx context.Context, raw json.RawMessage, path string, limit int) (*string, *transcriptionFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	v, f := transcriptionRequiredString(ctx, raw, path, limit)
	if f != nil {
		return nil, f
	}
	return &v, nil
}

func transcriptionNullableFloat(ctx context.Context, raw json.RawMessage, path string) (*float64, *transcriptionFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var v float64
	if err := decodeSpeechJSON(ctx, raw, &v); err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, &transcriptionFailure{path, "required finite number", err}
	}
	return &v, nil
}

func decodeTranscriptionSegments(ctx context.Context, raw json.RawMessage) ([]TranscriptionSegment, *transcriptionFailure) {
	path := memberPath("$", "segments")
	if len(raw) == 0 {
		return make([]TranscriptionSegment, 0), nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &transcriptionFailure{path, "required array", nil}
	}
	var items []json.RawMessage
	if err := decodeSpeechJSON(ctx, raw, &items); err != nil || items == nil {
		return nil, &transcriptionFailure{path, "required array", err}
	}
	if len(items) > maxCollectionItems {
		return nil, &transcriptionFailure{path, "must contain at most 4096 items", nil}
	}
	out := make([]TranscriptionSegment, len(items))
	for i, item := range items {
		if ctx.Err() != nil {
			return nil, &transcriptionFailure{indexPath(path, i), "canceled", ctx.Err()}
		}
		var obj map[string]json.RawMessage
		if err := decodeSpeechJSON(ctx, item, &obj); err != nil || obj == nil {
			return nil, &transcriptionFailure{indexPath(path, i), "required object", err}
		}
		base := indexPath(path, i)
		for _, key := range sortedRawKeys(obj) {
			if key != "text" && key != "startSecond" && key != "endSecond" {
				return nil, &transcriptionFailure{memberPath(base, key), "unknown field", nil}
			}
		}
		text, f := transcriptionRequiredString(ctx, obj["text"], memberPath(base, "text"), maxStringBytes)
		if f != nil {
			return nil, f
		}
		start, f := transcriptionRequiredFloat(ctx, obj["startSecond"], memberPath(base, "startSecond"))
		if f != nil {
			return nil, f
		}
		end, f := transcriptionRequiredFloat(ctx, obj["endSecond"], memberPath(base, "endSecond"))
		if f != nil {
			return nil, f
		}
		out[i] = TranscriptionSegment{Text: text, StartSeconds: start, EndSeconds: end}
	}
	return out, nil
}

func transcriptionRequiredFloat(ctx context.Context, raw json.RawMessage, path string) (float64, *transcriptionFailure) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, &transcriptionFailure{path, "required finite number", nil}
	}
	var v float64
	if err := decodeSpeechJSON(ctx, raw, &v); err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, &transcriptionFailure{path, "required finite number", err}
	}
	return v, nil
}

func transcriptionStringLimitReason(limit int) string {
	if limit == 255 {
		return "must not exceed 255 bytes"
	}
	return "must not exceed 1048576 bytes"
}

func decodeTranscriptionWarnings(ctx context.Context, raw json.RawMessage) ([]ProviderWarning, *transcriptionFailure) {
	path := memberPath("$", "warnings")
	if len(raw) == 0 {
		return make([]ProviderWarning, 0), nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &transcriptionFailure{path, "must be an array", nil}
	}
	var items []json.RawMessage
	if err := decodeSpeechJSON(ctx, raw, &items); err != nil || items == nil {
		return nil, &transcriptionFailure{path, "must be an array", err}
	}
	if len(items) > maxCollectionItems {
		return nil, &transcriptionFailure{path, "must contain at most 4096 items", nil}
	}
	out := make([]ProviderWarning, 0, len(items))
	for i, item := range items {
		if ctx.Err() != nil {
			return nil, &transcriptionFailure{indexPath(path, i), "canceled", ctx.Err()}
		}
		wrapped := make([]byte, 0, len(item)+2)
		wrapped = append(wrapped, '[')
		wrapped = append(wrapped, item...)
		wrapped = append(wrapped, ']')
		one, failure := decodeProviderWarnings(wrapped)
		if failure != nil {
			failurePath := failure.path
			prefix := indexPath(path, 0)
			if strings.HasPrefix(failurePath, prefix) {
				failurePath = indexPath(path, i) + strings.TrimPrefix(failurePath, prefix)
			}
			return nil, &transcriptionFailure{failurePath, failure.reason, failure.cause}
		}
		out = append(out, one[0])
	}
	return out, nil
}

func decodeTranscriptionMetadata(ctx context.Context, raw json.RawMessage) (map[string]json.RawMessage, *transcriptionFailure) {
	path := memberPath("$", "providerMetadata")
	if len(raw) == 0 {
		return nil, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, &transcriptionFailure{path, "must be an object", nil}
	}
	var providers map[string]json.RawMessage
	if err := decodeSpeechJSON(ctx, raw, &providers); err != nil || providers == nil {
		return nil, &transcriptionFailure{path, "must be an object", err}
	}
	out := make(map[string]json.RawMessage, len(providers))
	for _, name := range sortedRawKeys(providers) {
		if ctx.Err() != nil {
			return nil, &transcriptionFailure{memberPath(path, name), "canceled", ctx.Err()}
		}
		var object map[string]json.RawMessage
		if err := decodeSpeechJSON(ctx, providers[name], &object); err != nil || object == nil {
			return nil, &transcriptionFailure{memberPath(path, name), "must be an object", err}
		}
		out[name] = append(json.RawMessage(nil), providers[name]...)
	}
	return out, nil
}
