package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// EmbeddingRequest contains one or more strings to embed and optional sealed provider options.
type EmbeddingRequest struct {
	Values          []string
	ProviderOptions []ProviderOption
}

// EmbeddingResult is a strictly validated Embedding Model V4 result.
type EmbeddingResult struct {
	Embeddings       [][]float64
	Usage            *EmbeddingUsage
	Warnings         []ProviderWarning
	ProviderMetadata map[string]json.RawMessage
	Response         ResponseMetadata
}

// EmbeddingUsage reports the provider token count. Tokens is non-nil whenever Usage is non-nil.
type EmbeddingUsage struct{ Tokens *int64 }

// ProviderWarningType identifies a closed provider warning variant.
type ProviderWarningType string

const (
	ProviderWarningUnsupported   ProviderWarningType = "unsupported"
	ProviderWarningCompatibility ProviderWarningType = "compatibility"
	ProviderWarningDeprecated    ProviderWarningType = "deprecated"
	ProviderWarningOther         ProviderWarningType = "other"
)

// ProviderWarning is the closed V4 provider warning union.
type ProviderWarning struct {
	Type    ProviderWarningType
	Feature string
	Details *string
	Setting string
	Message string
}

// Embed validates and submits exactly one Embedding Model V4 request. It does not batch or retry.
func (client *Client) Embed(ctx context.Context, modelID string, request EmbeddingRequest) (*EmbeddingResult, error) {
	raw, err := client.executeProviderRequest(ctx, providerRouteEmbedding, modelID, func() ([]byte, error) {
		return prepareEmbeddingRequest(modelID, request)
	}, maxEmbeddingSuccessBodyBytes)
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
	return decodeEmbeddingResult(modelID, len(request.Values), raw)
}
