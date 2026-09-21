package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type ImageSize string
type ImageAspectRatio string

type ImageRequest struct {
	Prompt          *string
	Count           int
	Size            *ImageSize
	AspectRatio     *ImageAspectRatio
	Seed            *int64
	Files           []ImageInput
	Mask            *ImageInput
	ProviderOptions []ProviderOption
}

type ImageInput interface{ imageInput() }

type ImageURL struct {
	URL             string
	ProviderOptions []ProviderOption
}
type ImageBase64 struct {
	MediaType       string
	Data            string
	ProviderOptions []ProviderOption
}
type ImageBytes struct {
	MediaType       string
	Data            []byte
	ProviderOptions []ProviderOption
}

func (ImageURL) imageInput()    {}
func (ImageBase64) imageInput() {}
func (ImageBytes) imageInput()  {}

type ImageResult struct {
	Images           [][]byte
	Retryable        *bool
	Warnings         []ProviderWarning
	ProviderMetadata map[string]json.RawMessage
	Response         ResponseMetadata
	Usage            *ImageUsage
}

type ImageUsage struct {
	InputTokens  *float64
	OutputTokens *float64
	TotalTokens  *float64
}

// GenerateImage validates and submits exactly one Image Model V4 request. It never retries.
func (client *Client) GenerateImage(ctx context.Context, modelID string, request ImageRequest) (*ImageResult, error) {
	request = cloneImageRequest(request)
	raw, err := client.executeProviderRequest(ctx, providerRouteImage, modelID, func() ([]byte, error) {
		return prepareImageRequest(modelID, request)
	}, maxImageSuccessBodyBytes)
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
	return decodeImageResult(modelID, raw)
}
