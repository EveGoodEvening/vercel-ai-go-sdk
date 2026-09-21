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

const (
	providerRouteEmbedding     = "/embedding-model"
	providerRouteReranking     = "/reranking-model"
	providerRouteImage         = "/image-model"
	providerRouteSpeech        = "/speech-model"
	providerRouteTranscription = "/transcription-model"
	providerRouteLanguage      = "/language-model"
)

type rawEvaluationResponse struct {
	statusCode    int
	headers       http.Header
	body          []byte
	bodyTruncated bool
	bodyErr       error
}

type rawProviderResponse = rawEvaluationResponse

type boundedBodyCapture struct {
	body      []byte
	truncated bool
	err       error
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

func readBoundedResponse(ctx context.Context, body io.ReadCloser, limit int64) boundedBodyCapture {
	if body == nil {
		return boundedBodyCapture{body: []byte{}}
	}

	guarded := &closeOnceReadCloser{body: body}
	var canceled atomic.Bool
	stopCancel := context.AfterFunc(ctx, func() {
		canceled.Store(true)
		_ = guarded.Close()
	})
	data, readErr := io.ReadAll(io.LimitReader(guarded, limit+1))
	closeErr := guarded.Close()
	stopped := stopCancel()
	if !stopped && canceled.Load() {
		readErr = errors.Join(readErr, ctx.Err())
	}
	truncated := int64(len(data)) > limit
	if truncated {
		data = data[:limit]
		readErr = errors.Join(readErr, httpx.ErrResponseBodyTooLarge)
	}
	return boundedBodyCapture{body: data, truncated: truncated, err: errors.Join(readErr, closeErr)}
}

func readDiagnosticResponse(ctx context.Context, response *http.Response) boundedBodyCapture {
	capture := readBoundedResponse(ctx, response.Body, maxDiagnosticBodyBytes)
	if response.ContentLength > maxDiagnosticBodyBytes && !capture.truncated {
		capture.truncated = true
		capture.err = errors.Join(capture.err, httpx.ErrResponseBodyTooLarge)
	}
	return capture
}

func readSuccessResponse(ctx context.Context, response *http.Response, limit int64) (boundedBodyCapture, error) {
	if response.ContentLength > limit {
		var closeErr error
		if response.Body != nil {
			closeErr = response.Body.Close()
		}
		return boundedBodyCapture{}, &TransportError{operation: "read response body", cause: errors.Join(httpx.ErrResponseBodyTooLarge, closeErr)}
	}
	capture := readBoundedResponse(ctx, response.Body, limit)
	if capture.err != nil {
		return capture, &TransportError{operation: "read response body", cause: capture.err}
	}
	return capture, nil
}

func providerSpecificationHeader(route string) (string, bool) {
	switch route {
	case providerRouteEmbedding:
		return headerEmbeddingModelSpecificationVersion, true
	case providerRouteReranking:
		return headerRerankingModelSpecificationVersion, true
	case providerRouteImage:
		return headerImageModelSpecificationVersion, true
	case providerRouteSpeech:
		return headerSpeechModelSpecificationVersion, true
	case providerRouteTranscription:
		return headerTranscriptionModelSpecificationVersion, true
	case providerRouteLanguage:
		return headerLanguageModelSpecificationVersion, true
	default:
		return "", false
	}
}

// executeProviderRequest calls prepare before credential resolution. prepare
// owns request validation and encoding and must return caller-independent bytes.
// The request is attempted exactly once and redirects are never followed.
func (client *Client) executeProviderRequest(ctx context.Context, route, modelID string, prepare func() ([]byte, error), successBodyLimit int64) (rawProviderResponse, error) {
	if ctx == nil {
		return rawProviderResponse{}, validationError(`$["context"]`, "must not be nil")
	}
	if prepare == nil {
		return rawProviderResponse{}, validationError("$", "request preparation is unavailable")
	}
	payload, err := prepare()
	if err != nil {
		return rawProviderResponse{}, err
	}
	if len(payload) > maxRequestBodyBytes {
		return rawProviderResponse{}, validationError("$", "encoded request exceeds 16777216 bytes")
	}
	specificationHeader, ok := providerSpecificationHeader(route)
	if !ok {
		return rawProviderResponse{}, validationError(`$["route"]`, "unsupported provider route")
	}
	if err := ctx.Err(); err != nil {
		return rawProviderResponse{}, &TransportError{operation: "send request", cause: err}
	}
	authorization, authMethod, err := client.config.credential.authorization(ctx)
	if err != nil {
		return rawProviderResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.providerEndpoint(route), bytes.NewReader(payload))
	if err != nil {
		return rawProviderResponse{}, &TransportError{operation: "create request", cause: err}
	}
	request.Header = client.config.buildProviderHeaders(authorization, authMethod, modelID, specificationHeader)
	httpClient := &http.Client{
		Transport:     client.config.httpClient.Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Jar:           client.config.httpClient.Jar,
		Timeout:       client.config.httpClient.Timeout,
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return rawProviderResponse{}, &TransportError{operation: "send request", cause: err}
	}
	outcome := rawProviderResponse{statusCode: response.StatusCode, headers: response.Header.Clone()}
	if response.StatusCode != http.StatusOK {
		capture := readDiagnosticResponse(ctx, response)
		outcome.body = capture.body
		outcome.bodyTruncated = capture.truncated
		outcome.bodyErr = capture.err
		return outcome, nil
	}
	capture, err := readSuccessResponse(ctx, response, successBodyLimit)
	if err != nil {
		return rawProviderResponse{}, err
	}
	outcome.body = capture.body
	return outcome, nil
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
