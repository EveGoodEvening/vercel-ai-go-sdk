// Package gateway provides an experimental client for distinct Vercel AI
// Gateway contracts: the public Responses and Chat Completions generation APIs,
// and the AI SDK Model V4 provider protocols for evaluation, embeddings, and
// reranking.
// Public generation uses CreateResponse and StreamResponse at
// {publicBaseURL}/responses, and CreateChatCompletion and StreamChatCompletion
// at {publicBaseURL}/chat/completions. The default public base URL is
// https://ai-gateway.vercel.sh/v1. Buffered results retain bounded raw JSON;
// streams are caller-owned incremental readers with Next, Event, Err, and Close.
// The SDK does not execute returned tool calls.
//
// Evaluate posts to {baseURL}/evaluation-model, Embed posts to
// {baseURL}/embedding-model, and Rerank posts to {baseURL}/reranking-model; the
// default provider base URL is https://ai-gateway.vercel.sh/v4/ai. Evaluation
// validates shared state and keyed boolean, choice, and score questions. Embed
// accepts a dynamic provider/model ID and 1..4096 strings as one request.
// Rerank accepts a dynamic provider/model ID, query, optional topN, and one
// homogeneous envelope of 1..4096 text or JSON-object documents. These methods
// are separate from generation and do not implement corresponding public v1
// evaluation, embedding, or reranking APIs.
//
// Construct a Client with NewClient. Credential precedence is explicit API key,
// AI_GATEWAY_API_KEY, explicit fixed/source OIDC, then VERCEL_OIDC_TOKEN. A
// selected TokenSource is called once per HTTP attempt. Options apply in order.
// WithBaseURL affects provider evaluation, embeddings, and reranking;
// WithPublicBaseURL affects only public generation. WithHeaders clones its
// input and rejects owned headers; WithHTTPClient retains the supplied pointer.
//
// All request methods validate before network I/O and reject a nil context.
// Buffered generation and evaluation calls accept the bounded RetryPolicy;
// retries are off by default and only statuses 408, 409, 429, and 500 through
// 599 are retryable. Embed and Rerank always make exactly one request and
// perform no automatic batching or retry. Cancellation applies to token
// resolution, requests, body reads, retry waits, and stream reads.
//
// Embed limits each input string to 1 MiB and the encoded request to 16 MiB.
// A successful embedding body is validated up to 32 MiB; ResponseMetadata.Body
// retains at most its first 1 MiB. The response must contain exactly one finite,
// nonempty vector per input. Usage is absent/null or an exact int64 token count;
// absent warnings normalize to an empty slice, while explicit null is invalid;
// warnings are a closed discriminator union and provider metadata values must be
// non-null JSON objects. ProviderOption is sealed and currently has no concrete
// implementation, so nil or empty options are the only usable forms.
//
// Rerank limits the query and each text or raw JSON-object document to 1 MiB,
// allows 1..4096 documents, and limits the encoded request to 16 MiB. topN,
// when present, must be in 1..len(documents). Successful bodies are strictly
// validated up to 32 MiB and ResponseMetadata.Body retains at most 1 MiB.
// Results retain provider order, require unique in-range original indices, and
// reconstruct each input document. Scores may be any finite float64; no 0..1,
// nonnegative, or sorting guarantee is made. Warnings and provider metadata
// use the shared strict shapes. ProviderOption has no concrete exported
// implementation.
//
// Errors are ConfigurationError, ValidationError, TransportError,
// ResponseError, or ResponseValidationError and support errors.As. Raw JSON,
// diagnostic bodies, prompts, embedding inputs and vectors, reranking queries,
// documents and scores, tool data, provider options and metadata, headers, and
// identifiers may be sensitive and must be sanitized before logging or storage.
// Typed request declarations for the Exa, Parallel, Perplexity, and Tako Chat
// Gateway server-search tools are available only through the opt-in server-tools
// methods. Responses opt-in built-in-tools methods support fixed low-context
// {"type":"web_search","search_context_size":"low"}, fieldless
// {"type":"x_search"}, and ResponseXSearchOptionsTool's allowed_x_handles,
// excluded_x_handles, from_date, to_date, enable_image_understanding, and
// enable_video_understanding fields for spacexai/grok-4.6. Cleared shapes are
// a single field, the neutral shape with both empty handle lists and both flags
// explicitly false, and either maximal shape with one non-empty handle list,
// both dates, and both flags true. Simultaneous non-empty handle lists are
// rejected locally; arbitrary subsets remain unresolved. Buffered search
// results are available only through ResponseResult.RawJSON(), and streaming
// search events remain RawResponseEvent values. Other web-search forms or
// options, wider model compatibility, semantic or limit claims, date semantics,
// wrong-kind behavior, direct-xAI support, public /v1/evaluate, public
// /v1/embeddings, and public v1 reranking remain unsupported. Image/video
// generation, speech, transcription, realtime, batches, and management APIs are
// also absent. These probes do not satisfy the separately gated public-
// generation or provider-evaluation live runs, which remain NOT RUN; no hosted
// embedding or reranking success is claimed.

package gateway
