// Package gateway provides an experimental client for two distinct Vercel AI
// Gateway contracts: the public Responses and Chat Completions generation APIs,
// and the AI SDK Evaluation Model V4 provider protocol.
//
// Public generation uses CreateResponse and StreamResponse at
// {publicBaseURL}/responses, and CreateChatCompletion and StreamChatCompletion
// at {publicBaseURL}/chat/completions. The default public base URL is
// https://ai-gateway.vercel.sh/v1. Buffered results retain bounded raw JSON;
// streams are caller-owned incremental readers with Next, Event, Err, and Close.
// The SDK does not execute returned tool calls.
//
// Evaluate posts to {baseURL}/evaluation-model; its default provider base URL is
// https://ai-gateway.vercel.sh/v4/ai. Evaluation validates shared state and
// keyed boolean, choice, and score questions. It is separate from generation
// and does not implement the public /v1/evaluate API.
//
// Construct a Client with NewClient. Credential precedence is explicit API key,
// AI_GATEWAY_API_KEY, explicit fixed/source OIDC, then VERCEL_OIDC_TOKEN. A
// selected TokenSource is called once per HTTP attempt. Options apply in order.
// WithBaseURL affects only provider evaluation; WithPublicBaseURL affects only
// public generation. WithHeaders clones its input and rejects owned headers;
// WithHTTPClient retains the supplied pointer.
//
// All request methods validate before network I/O and reject a nil context.
// Buffered calls accept the bounded RetryPolicy; retries are off by default and
// only statuses 408, 409, 429, and 500 through 599 are retryable. Streams are
// never retried after headers. Cancellation applies to token resolution,
// requests, body reads, retry waits, and stream reads.
//
// Errors are ConfigurationError, ValidationError, TransportError,
// ResponseError, or ResponseValidationError and support errors.As. Raw JSON,
// diagnostic bodies, prompts, tool data, provider options, headers, and
// identifiers may be sensitive and must be sanitized before logging or storage.
// Typed request declarations for the Exa, Parallel, Perplexity, and Tako Chat
// Gateway server-search tools are available only through the opt-in server-tools
// methods. Outputs retain the existing typed and raw Chat boundaries, with no
// automatic tool execution. Public /v1/evaluate, Responses built-in search,
// typed Chat search outputs or metadata, Gateway-native x_search, and unlisted
// modalities remain unsupported. Hermetic fixtures are not live-service
// evidence; the separately gated public-generation and provider-evaluation live
// runs remain NOT RUN.
package gateway
