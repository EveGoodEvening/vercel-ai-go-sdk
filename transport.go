package gateway

import (
	"bytes"
	"context"
	"net/http"

	"github.com/EveGoodEvening/vercel-ai-gateway-go-sdk/internal/httpx"
)

type rawEvaluationResponse struct {
	statusCode    int
	headers       http.Header
	body          []byte
	bodyTruncated bool
	bodyErr       error
}

func (client *Client) executeEvaluationRequest(ctx context.Context, modelID string, payload []byte) (rawEvaluationResponse, error) {
	authorization, authMethod, err := client.config.credential.authorization(ctx)
	if err != nil {
		return rawEvaluationResponse{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.baseURL+"/evaluation-model", bytes.NewReader(payload))
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "create request", cause: err}
	}
	request.Header = client.config.buildHeaders(authorization, authMethod, modelID)

	response, err := client.config.httpClient.Do(request)
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}

	outcome := rawEvaluationResponse{
		statusCode: response.StatusCode,
		headers:    response.Header.Clone(),
	}
	capture := httpx.ReadAndClose(response.Body)
	outcome.body = capture.Body
	outcome.bodyTruncated = capture.Truncated
	outcome.bodyErr = capture.Err
	return outcome, nil
}
