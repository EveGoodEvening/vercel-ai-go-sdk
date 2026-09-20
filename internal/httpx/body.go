package httpx

import (
	"errors"
	"io"
)

const responseBodyLimit = 1 << 20

// ErrResponseBodyTooLarge reports that a response body exceeded the fixed
// capture limit. The returned body is the retained prefix.
var ErrResponseBodyTooLarge = errors.New("response body exceeds capture limit")

// BodyCapture is the status-neutral result of consuming and closing a body.
type BodyCapture struct {
	Body      []byte
	Truncated bool
	Err       error
}

// ReadAndClose captures at most the fixed response-body limit, detects excess
// data with a limit-plus-one read, and always closes body. Read, overflow, and
// close failures remain reachable through Err.
func ReadAndClose(body io.ReadCloser) BodyCapture {
	if body == nil {
		return BodyCapture{Body: []byte{}}
	}

	data, readErr := io.ReadAll(io.LimitReader(body, responseBodyLimit+1))
	closeErr := body.Close()
	truncated := len(data) > responseBodyLimit
	if truncated {
		data = data[:responseBodyLimit]
		readErr = errors.Join(readErr, ErrResponseBodyTooLarge)
	}
	return BodyCapture{
		Body:      data,
		Truncated: truncated,
		Err:       errors.Join(readErr, closeErr),
	}
}
