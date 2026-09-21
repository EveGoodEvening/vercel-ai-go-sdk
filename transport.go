package gateway

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/EveGoodEvening/vercel-ai-go-sdk/internal/httpx"
)

type rawEvaluationResponse struct {
	statusCode    int
	headers       http.Header
	body          []byte
	bodyTruncated bool
	bodyErr       error
}

type closeOnceReadCloser struct {
	body io.ReadCloser
	once sync.Once
	err  error
}

func (body *closeOnceReadCloser) Read(buffer []byte) (int, error) {
	return body.body.Read(buffer)
}

func (body *closeOnceReadCloser) Close() error {
	body.once.Do(func() {
		body.err = body.body.Close()
	})
	return body.err
}

func readAndCloseResponse(ctx context.Context, body io.ReadCloser) httpx.BodyCapture {
	if body == nil {
		return httpx.ReadAndClose(nil)
	}

	guarded := &closeOnceReadCloser{body: body}
	var canceled atomic.Bool
	stopCancel := context.AfterFunc(ctx, func() {
		canceled.Store(true)
		_ = guarded.Close()
	})
	capture := httpx.ReadAndClose(guarded)
	stopped := stopCancel()
	if !stopped && canceled.Load() {
		capture.Err = errors.Join(capture.Err, ctx.Err())
	}
	return capture
}

func (client *Client) executeEvaluationRequest(ctx context.Context, modelID string, payload []byte) (rawEvaluationResponse, error) {
	if err := ctx.Err(); err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}

	authorization, authMethod, err := client.config.credential.authorization(ctx)
	if err != nil {
		return rawEvaluationResponse{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.baseURL+"/evaluation-model", bytes.NewReader(payload))
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "create request", cause: err}
	}
	request.Header = client.config.buildHeaders(authorization, authMethod, modelID)

	httpClient := &http.Client{
		Transport: client.config.httpClient.Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Jar:     client.config.httpClient.Jar,
		Timeout: client.config.httpClient.Timeout,
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return rawEvaluationResponse{}, &TransportError{operation: "send request", cause: err}
	}

	outcome := rawEvaluationResponse{
		statusCode: response.StatusCode,
		headers:    response.Header.Clone(),
	}
	capture := readAndCloseResponse(ctx, response.Body)
	outcome.body = capture.Body
	outcome.bodyTruncated = capture.Truncated
	outcome.bodyErr = capture.Err
	return outcome, nil
}
