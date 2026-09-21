package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const maxRerankingSuccessBodyBytes = 32 << 20

// RerankRequest contains a query, one homogeneous document envelope, and optional controls.
type RerankRequest struct {
	Query           string
	Documents       RerankDocuments
	TopN            *int
	ProviderOptions []ProviderOption
}

// RerankDocuments is the sealed homogeneous request-document envelope.
type RerankDocuments interface{ rerankDocuments() }

// RerankTexts contains text documents.
type RerankTexts struct{ Values []string }

func (RerankTexts) rerankDocuments() {}

// RerankObjects contains JSON object documents.
type RerankObjects struct{ Values []json.RawMessage }

func (RerankObjects) rerankDocuments() {}

// RerankDocument is the sealed reconstructed result-document union.
type RerankDocument interface{ rerankDocument() }

// RerankText is a reconstructed text document.
type RerankText struct{ Text string }

func (RerankText) rerankDocument() {}

// RerankJSON is a reconstructed JSON object document. Value is independently owned.
type RerankJSON struct{ Value json.RawMessage }

func (RerankJSON) rerankDocument() {}

// RerankItem is one provider-ordered ranking result.
type RerankItem struct {
	OriginalIndex int
	Score         float64
	Document      RerankDocument
}

// RerankResult is a strictly validated Reranking Model V4 result.
type RerankResult struct {
	Results          []RerankItem
	Warnings         []ProviderWarning
	ProviderMetadata map[string]json.RawMessage
	Response         ResponseMetadata
}

// Rerank validates and submits exactly one Reranking Model V4 request. It does not retry.
func (client *Client) Rerank(ctx context.Context, modelID string, request RerankRequest) (*RerankResult, error) {
	var prepared *preparedRerankRequest
	raw, err := client.executeProviderRequest(ctx, providerRouteReranking, modelID, func() ([]byte, error) {
		var prepareErr error
		prepared, prepareErr = prepareRerankRequest(modelID, request)
		if prepareErr != nil {
			return nil, prepareErr
		}
		return prepared.payload, nil
	}, maxRerankingSuccessBodyBytes)
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
	return decodeRerankResult(modelID, prepared, raw)
}
