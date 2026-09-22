package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxSpeechAudioBytes = maxSpeechDecodedBytes

type speechFailure struct {
	path, reason string
	cause        error
}

func decodeSpeechResult(ctx context.Context, modelID string, raw rawProviderResponse) (*SpeechResult, error) {
	canceled := func() (*SpeechResult, error) {
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
	invalid := func(f *speechFailure) (*SpeechResult, error) {
		return nil, &ResponseValidationError{cause: f.cause, statusCode: http.StatusOK, path: f.path, reason: f.reason, bodyTruncated: raw.bodyTruncated, rawResponseBody: append([]byte(nil), retained...)}
	}
	if raw.bodyTruncated {
		return invalid(&speechFailure{"$", "response body exceeds 100663296 bytes", raw.bodyErr})
	}
	if raw.bodyErr != nil {
		return nil, &TransportError{operation: "read response body", cause: raw.bodyErr}
	}
	if ctx.Err() != nil {
		return canceled()
	}
	if f := scanSpeechJSON(ctx, raw.body); f != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(f)
	}
	if ctx.Err() != nil {
		return canceled()
	}
	var top map[string]json.RawMessage
	if err := decodeSpeechJSON(ctx, raw.body, &top); err != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(&speechFailure{"$", "malformed JSON", err})
	}
	for _, key := range sortedRawKeys(top) {
		if key != "audio" && key != "warnings" && key != "providerMetadata" {
			return invalid(&speechFailure{memberPath("$", key), "unknown field", nil})
		}
	}
	if ctx.Err() != nil {
		return canceled()
	}
	rawAudio, ok := top["audio"]
	if !ok {
		return invalid(&speechFailure{memberPath("$", "audio"), "required string", nil})
	}
	var audio string
	if bytes.Equal(bytes.TrimSpace(rawAudio), []byte("null")) {
		return invalid(&speechFailure{memberPath("$", "audio"), "required string", nil})
	}
	if err := decodeSpeechJSON(ctx, rawAudio, &audio); err != nil {
		if ctx.Err() != nil {
			return canceled()
		}
		return invalid(&speechFailure{memberPath("$", "audio"), "required string", nil})
	}
	if len(audio) > maxSpeechAudioBytes {
		return invalid(&speechFailure{memberPath("$", "audio"), "must not exceed 67108864 bytes", nil})
	}
	if ctx.Err() != nil {
		return canceled()
	}
	warnings, wf := decodeProviderWarnings(top["warnings"])
	if wf != nil {
		return invalid(&speechFailure{wf.path, wf.reason, wf.cause})
	}
	if ctx.Err() != nil {
		return canceled()
	}
	metadata, mf := decodeEmbeddingMetadata(top["providerMetadata"])
	if mf != nil {
		return invalid(&speechFailure{mf.path, mf.reason, mf.cause})
	}
	if ctx.Err() != nil {
		return canceled()
	}
	return &SpeechResult{Audio: audio, Warnings: warnings, ProviderMetadata: metadata, Response: ResponseMetadata{ModelID: modelID, Headers: nonNilHeaderClone(raw.headers), Body: retained}}, nil
}

func scanSpeechJSON(ctx context.Context, body []byte) *speechFailure {
	d := json.NewDecoder(&speechContextReader{ctx: ctx, r: bytes.NewReader(body)})
	d.UseNumber()
	if f := scanSpeechValue(ctx, d, "$", 1); f != nil {
		return f
	}
	if _, err := d.Token(); err == nil {
		return &speechFailure{"$", "trailing JSON value", nil}
	} else if !errors.Is(err, io.EOF) {
		return &speechFailure{"$", "malformed JSON", err}
	}
	return nil
}

func scanSpeechValue(ctx context.Context, d *json.Decoder, path string, depth int) *speechFailure {
	if err := ctx.Err(); err != nil {
		return &speechFailure{path, "canceled", err}
	}
	if depth > maxJSONDepth {
		return &speechFailure{path, "maximum depth is 64", nil}
	}
	tok, err := d.Token()
	if err != nil {
		return &speechFailure{path, "malformed JSON", err}
	}
	switch value := tok.(type) {
	case string:
		limit := maxStringBytes
		if path == memberPath("$", "audio") {
			limit = maxSpeechAudioBytes
		}
		if len(value) > limit {
			if limit == maxSpeechAudioBytes {
				return &speechFailure{path, "must not exceed 67108864 bytes", nil}
			}
			return &speechFailure{path, "must not exceed 1048576 bytes", nil}
		}
	case json.Delim:
		switch value {
		case '{':
			seen := make(map[string]bool)
			count := 0
			for d.More() {
				kt, e := d.Token()
				if e != nil {
					return &speechFailure{path, "malformed JSON", e}
				}
				key, ok := kt.(string)
				if !ok {
					return &speechFailure{path, "malformed JSON", nil}
				}
				if len(key) > maxStringBytes {
					return &speechFailure{path, "object key exceeds 1 MiB", nil}
				}
				kp := memberPath(path, key)
				if seen[key] {
					return &speechFailure{kp, "duplicate field", nil}
				}
				seen[key] = true
				count++
				if count > maxCollectionItems {
					return &speechFailure{path, "must contain at most 4096 members", nil}
				}
				if f := scanSpeechValue(ctx, d, kp, depth+1); f != nil {
					return f
				}
			}
			if _, e := d.Token(); e != nil {
				return &speechFailure{path, "malformed JSON", e}
			}
		case '[':
			count := 0
			for d.More() {
				if count >= maxCollectionItems {
					return &speechFailure{path, "must contain at most 4096 items", nil}
				}
				if f := scanSpeechValue(ctx, d, indexPath(path, count), depth+1); f != nil {
					return f
				}
				count++
			}
			if _, e := d.Token(); e != nil {
				return &speechFailure{path, "malformed JSON", e}
			}
		default:
			return &speechFailure{path, "malformed JSON", nil}
		}
	}
	return nil
}

type speechContextReader struct {
	ctx context.Context
	r   *bytes.Reader
}

func (r *speechContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	// Bound decoder work between cancellation checks even when it asks for a large buffer.
	if len(p) > 32<<10 {
		p = p[:32<<10]
	}
	return r.r.Read(p)
}

func decodeSpeechJSON(ctx context.Context, body []byte, dst any) error {
	d := json.NewDecoder(&speechContextReader{ctx: ctx, r: bytes.NewReader(body)})
	return d.Decode(dst)
}
