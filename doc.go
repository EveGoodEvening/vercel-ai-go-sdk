// Package gateway provides an experimental client for distinct Vercel AI
// Gateway contracts: the public Responses and Chat Completions generation APIs,
// and the AI SDK Model V4 provider protocols for evaluation, image generation,
// speech synthesis, transcription, embeddings, and reranking.
// Public generation uses CreateResponse and StreamResponse at
// {publicBaseURL}/responses, and CreateChatCompletion and StreamChatCompletion
// at {publicBaseURL}/chat/completions. The default public base URL is
// https://ai-gateway.vercel.sh/v1. Buffered results retain bounded raw JSON;
// streams are caller-owned incremental readers with Next, Event, Err, and Close.
// The SDK does not execute returned tool calls.
//
// Evaluate posts to {baseURL}/evaluation-model, GenerateImage posts to
// {baseURL}/image-model, GenerateSpeech posts to {baseURL}/speech-model,
// Transcribe posts to {baseURL}/transcription-model, Embed posts to
// {baseURL}/embedding-model, and Rerank posts to {baseURL}/reranking-model; the
// default provider base URL is https://ai-gateway.vercel.sh/v4/ai. Evaluation
// validates shared state and keyed boolean, choice, and score questions.
// GenerateImage accepts a dynamic provider/model ID, 1..16 requested outputs,
// and URL, opaque base64, or byte image inputs. GenerateSpeech accepts a
// dynamic provider/model ID and exact presence-preserving speech fields.
// Transcribe accepts a dynamic provider/model ID and one opaque-string or byte
// audio input. Embed accepts a dynamic provider/model ID and 1..4096 strings as
// one request. Rerank accepts a dynamic provider/model ID, query, optional topN,
// and one homogeneous envelope of 1..4096 text or JSON-object documents. These
// methods are separate from public generation and do not implement corresponding
// public v1 evaluation, image, speech, transcription, embedding, or reranking APIs.
//
// Construct a Client with NewClient. Credential precedence is explicit API key,
// AI_GATEWAY_API_KEY, explicit fixed/source OIDC, then VERCEL_OIDC_TOKEN. A
// selected TokenSource is called once per HTTP attempt. Options apply in order.
// WithBaseURL affects provider evaluation, image generation, speech synthesis,
// transcription, embeddings, and reranking; WithPublicBaseURL affects only public generation.
// WithHeaders clones input and rejects owned headers; WithHTTPClient retains
// the supplied pointer.
//
// All request methods validate before network I/O and reject a nil context.
// Buffered generation and evaluation calls accept the bounded RetryPolicy;
// retries are off by default and only statuses 408, 409, 429, and 500 through
// 599 are retryable. GenerateImage, GenerateSpeech, Transcribe, Embed, and
// Rerank always make exactly one request and perform no automatic batching or
// retry. Cancellation applies to token resolution, requests, body reads, retry
// waits, provider response decoding, and stream reads.
//
// GenerateImage limits Count to 1..16 and the encoded request to 16 MiB. Prompt
// presence is preserved; present-empty size/aspect ratio and present-zero seed
// are omitted; nil versus empty files is preserved. ImageURL values are sent as
// URLs without fetching, ImageBase64 is opaque request data, and ImageBytes is
// base64-encoded locally; the SDK performs no upload. Successful bodies are
// strictly validated up to 96 MiB and may contain 0..16 images independently of
// Count. Each output must be strict standard padded base64 and decode to at most
// 16 MiB, with at most 64 MiB decoded in aggregate; ResponseMetadata.Body keeps
// at most 1 MiB. Retryability, usage, warnings, and metadata preserve optional
// presence. ProviderOption is sealed with no concrete exported implementation,
// and ImageResult intentionally has no media-type field.
//
// GenerateSpeech always emits Text, including empty, while Voice, Instructions,
// Language, and OutputFormat omit empty strings but preserve whitespace. Speed
// is omitted only when nil and otherwise accepts any finite value, including
// zero or negative values. Text and instructions are limited to 1 MiB; voice,
// language, and output format to 255 bytes; and the encoded request to 16 MiB.
// Successful bodies are strictly validated up to 96 MiB. Audio is an opaque
// string of at most 64 MiB: it is not base64-decoded and has no media-type field.
// Warnings and provider metadata use the shared strict presence rules, and
// ResponseMetadata.Body retains at most a 1 MiB prefix. ProviderOption remains
// sealed with no concrete exported implementation.
//
// Transcribe accepts TranscriptionBase64 as opaque valid UTF-8 of at most 1 MiB
// without decoding or normalization, or TranscriptionBytes of at most 8 MiB,
// standard-base64 encoded exactly once. MediaType is opaque valid UTF-8 of at
// most 255 bytes; the encoded request is limited to 16 MiB. Successful bodies
// are strictly validated up to 16 MiB. Text is required; absent segments become
// a non-nil empty slice, and at most 4096 segments with required text and finite
// timestamps are accepted. Language and duration preserve absent/null as nil;
// present duration and timestamps may be negative, fractional, equal, or
// reversed. Warnings and provider metadata use the shared strict presence rules,
// and ResponseMetadata.Body retains at most a 1 MiB prefix. The method is
// buffered and has no URL-audio variant, streaming, public-v1, or parity claim.
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
// diagnostic bodies, prompts, image inputs and decoded outputs, speech text,
// voices, instructions, languages, output formats, speeds and opaque audio,
// transcription audio, transcript text, segments, language and duration,
// embedding inputs and vectors, reranking queries, documents and scores, tool
// data, provider options and metadata, headers, and identifiers may be sensitive
// and must be sanitized before logging or storage.
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
// /v1/embeddings, public v1 image generation, public v1 speech, public v1
// transcription, public v1 reranking, and streaming transcription remain
// unsupported. Video generation, realtime, batches, and management APIs are
// also absent. No JavaScript parity, universal provider/model compatibility,
// concrete provider option semantics, or hosted image, speech, transcription,
// embedding, or reranking success is claimed. Search probes do
// not satisfy the separately gated public-generation or provider-evaluation
// live runs, which remain NOT RUN.

package gateway
