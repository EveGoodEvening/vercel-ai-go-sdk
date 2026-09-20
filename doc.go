// Package gateway provides an experimental evaluation-only client for Vercel
// AI Gateway's AI SDK Evaluation Model V4 provider protocol. It posts to
// {baseURL}/evaluation-model; the default base URL is
// https://ai-gateway.vercel.sh/v4/ai. It does not implement the separate public
// /v1/evaluate API, OpenAI-compatible APIs, generation, streaming, search, or
// parity with the JavaScript AI SDK.
//
// Construct a Client with NewClient. Credential precedence is explicit API key,
// AI_GATEWAY_API_KEY, explicit fixed/source OIDC, then VERCEL_OIDC_TOKEN. A
// selected TokenSource is called once per HTTP attempt. Options are applied in
// order and stop at the first ConfigurationError. WithHeaders clones its input
// and rejects protocol-owned headers; WithHTTPClient retains the supplied
// pointer; WithBaseURL accepts only an absolute non-opaque HTTP(S) URL without
// userinfo, query, or fragment and removes only trailing path slashes.
//
// Evaluate validates all input before network I/O. A nil context is a
// ValidationError at $["context"]. The request supports JSON-compatible shared
// state, keyed BooleanQuestion, ChoiceQuestion, and ScoreQuestion values, and
// optional provider objects. Only HTTP status 200 is successful; response
// Content-Type is not enforced. Successful JSON is strict: duplicate and
// unknown fixed fields, trailing values, non-finite numbers, and contract
// violations produce ResponseValidationError.
//
// Retries are disabled unless RetryPolicy.MaxAttempts is 2 through 10. Only
// statuses 408, 409, 429, and 500 through 599 are retryable; error type/code do
// not affect classification. Evaluation may be billable and non-idempotent, so
// enabling retries accepts duplicate-work risk.
//
// Errors are classified as ConfigurationError, ValidationError, TransportError,
// ResponseError, or ResponseValidationError and support errors.As. Accessors
// are nil-safe. RawResponseBody and ResponseMetadata.Body may contain echoed
// state or provider options and must be sanitized before logging or storage.
package gateway
