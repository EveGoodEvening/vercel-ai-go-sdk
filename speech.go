package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// SpeechRequest describes one provider-protocol speech synthesis request.
type SpeechRequest struct {
	Text            string
	Voice           string
	Instructions    string
	Language        string
	OutputFormat    string
	Speed           *float64
	ProviderOptions []ProviderOption
}

// SpeechResult contains the provider's opaque audio string and response details.
type SpeechResult struct {
	Audio            string
	Warnings         []ProviderWarning
	ProviderMetadata map[string]json.RawMessage
	Response         ResponseMetadata
}

// GenerateSpeech validates and submits exactly one Speech Model V4 request. It never retries.
func (client *Client) GenerateSpeech(ctx context.Context, modelID string, request SpeechRequest) (*SpeechResult, error) {
	if ctx == nil {
		return nil, validationError(memberPath("$", "context"), "must not be nil")
	}
	if _, err := preflightSpeechRequest(modelID, request); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, &TransportError{operation: "send request", cause: err}
	}
	request = cloneSpeechRequest(request)
	raw, err := client.executeProviderRequest(ctx, providerRouteSpeech, modelID, func() ([]byte, error) {
		return prepareSpeechRequest(modelID, request)
	}, maxSpeechSuccessBodyBytes)
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
	return decodeSpeechResult(ctx, modelID, raw)
}
