package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// TranscriptionRequest describes one provider-protocol transcription request.
type TranscriptionRequest struct {
	Audio           TranscriptionAudio
	ProviderOptions []ProviderOption
}

// TranscriptionAudio is the closed set of supported transcription audio inputs.
type TranscriptionAudio interface{ transcriptionAudio() }

// TranscriptionBase64 carries an opaque caller-provided audio string.
type TranscriptionBase64 struct {
	MediaType string
	Data      string
}

// TranscriptionBytes carries raw audio bytes, encoded once as standard base64 on the wire.
type TranscriptionBytes struct {
	MediaType string
	Data      []byte
}

func (TranscriptionBase64) transcriptionAudio() {}
func (TranscriptionBytes) transcriptionAudio()  {}

// TranscriptionResult contains one transcription and response details.
type TranscriptionResult struct {
	Text             string
	Segments         []TranscriptionSegment
	Language         *string
	DurationSeconds  *float64
	Warnings         []ProviderWarning
	ProviderMetadata map[string]json.RawMessage
	Response         ResponseMetadata
}

// TranscriptionSegment is a timestamped transcription segment.
type TranscriptionSegment struct {
	Text         string
	StartSeconds float64
	EndSeconds   float64
}

// Transcribe validates and submits exactly one Transcription Model V4 request. It never retries.
func (client *Client) Transcribe(ctx context.Context, modelID string, request TranscriptionRequest) (*TranscriptionResult, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "must not be nil")
	}
	if _, err := preflightTranscriptionRequest(modelID, request); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, &TransportError{operation: "send request", cause: err}
	}
	request = cloneTranscriptionRequest(request)
	raw, err := client.executeProviderRequest(ctx, providerRouteTranscription, modelID, func() ([]byte, error) {
		return prepareTranscriptionRequest(modelID, request)
	}, maxTranscriptionSuccessBodyBytes)
	if err != nil {
		return nil, err
	}
	if raw.statusCode != http.StatusOK {
		now := time.Now()
		if client.config.retryHooks.now != nil {
			now = client.config.retryHooks.now()
		}
		return nil, composeResponseError(raw, now)
	}
	return decodeTranscriptionResult(ctx, modelID, raw)
}
