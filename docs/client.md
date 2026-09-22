# Client configuration

Shared behavior for [generation](generation.md), [provider evaluation](evaluation.md), and the staged provider-protocol `Client.GenerateImage`, `Client.GenerateSpeech`, `Client.Transcribe`, `Client.Embed`, and `Client.Rerank` surfaces described below. `NewClient` applies options in order, stops at the first error, and makes no network request.

## Authentication

Credentials are selected once at construction, in this order:

1. Explicit `WithAPIKey`.
2. Nonblank `AI_GATEWAY_API_KEY`.
3. The last explicit `WithOIDCToken` or `WithOIDCTokenSource`.
4. Nonblank `VERCEL_OIDC_TOKEN`.
5. Otherwise, `*ConfigurationError` with option `credentials`.

An environment API key therefore beats explicit OIDC. A rejected credential is never replaced by another credential. Blank environment values count as absent; explicit fixed credentials reject blank values. Nonblank values are preserved, not trimmed.

For long-lived OIDC clients, use a refresh-capable `TokenSource` rather than a fixed token. Its `Token(context.Context) (string, error)` method is called once per HTTP attempt with the operation's context. An error or blank token becomes a `TransportError` with operation `resolve OIDC token`. Nil and typed-nil sources are rejected.

## Endpoints and HTTP options

| Option | Default base | Used by |
| --- | --- | --- |
| `WithPublicBaseURL` | `https://ai-gateway.vercel.sh/v1` | Responses and Chat, including server-tools methods |
| `WithBaseURL` | `https://ai-gateway.vercel.sh/v4/ai` | `Evaluate`, `GenerateImage`, `GenerateSpeech`, `Transcribe`, `Embed`, and `Rerank` |

Pass the base, not the full endpoint: methods append `/responses`, `/chat/completions`, `/evaluation-model`, `/image-model`, `/speech-model`, `/transcription-model`, `/embedding-model`, or `/reranking-model`. Both options require an absolute HTTP(S) URL with a host, no surrounding whitespace, and no userinfo, query, or fragment. Existing paths and escapes are retained; trailing path slashes are removed. `WithPublicBaseURL` also rejects an empty literal `#` delimiter. No endpoint inference or cross-surface conversion occurs.

Other options:

- `WithHTTPClient`: accepts a non-nil `*http.Client`; retains the pointer and uses its transport, cookie jar, and timeout. It does not mutate the supplied client. **Redirects are never followed**, regardless of its `CheckRedirect`. Subsequent mutation follows `net/http` concurrency rules.
- `WithTeam`: sets `X-Vercel-Ai-Gateway-Team`; requires a nonblank value and has no environment fallback.
- `WithHeaders`: clones the supplied headers at construction and per request; nil is allowed. Authorization, content type, team, and provider-protocol headers are SDK-owned and rejected case-insensitively even when their values are empty. See the protected list in [client.go](../client.go).
- `WithRetryPolicy`: copies and validates the policy described below.

All requests use bearer authentication and JSON. Streams request `text/event-stream`. Provider evaluation, image generation, speech synthesis, transcription, embeddings, and reranking send their matching `/v4` protocol headers and the caller-supplied model ID; no fixed model catalog for these provider protocols is built into the client.

## Provider-protocol image generation

`Client.GenerateImage(ctx, modelID, request)` is a staged, experimental Model V4 provider-protocol surface. It sends exactly one `POST` to `{WithBaseURL}/image-model`, which is `/v4/ai/image-model` at the default base, with `Ai-Image-Model-Specification-Version: 4`. `modelID` is dynamic but must be a nonempty `provider/model` string. This is not a public v1 image API, a JavaScript-parity claim, or evidence that every model or provider supports image generation.

`ImageRequest.Count` is required in **1–16**. Optional request presence is exact rather than inferred: a non-nil `Prompt` is emitted even when empty and is limited to **1 MiB**; `Size` and `AspectRatio` are omitted when nil or present-empty and otherwise passed through as opaque strings of at most 255 bytes; `Seed` is omitted when nil or present-zero and otherwise emitted exactly; nil `Files` omits the key while a non-nil empty slice emits `"files":[]`; and a non-nil `Mask` is emitted. The SDK does not validate size/aspect grammar or claim provider support for particular values.

`Files` and `Mask` use the sealed `ImageInput` variants, accepted as values or non-nil pointers:

- `ImageURL` sends its URL string as given. The SDK does not fetch it, validate its scheme or grammar, or upload its contents.
- `ImageBase64` sends `MediaType` and `Data` as opaque strings. Request data is not decoded, validated as base64, or normalized.
- `ImageBytes` standard-base64 encodes `Data` exactly once and sends it with the opaque `MediaType`.

URL, media-type, and opaque base64 strings may be empty and are limited to **1 MiB** each. The complete encoded JSON request is limited to **16 MiB**. Top-level and per-input `ProviderOptions` use sealed `[]ProviderOption` values: nil and empty omit `providerOptions`, and this release exports no concrete implementation, so no supported nonempty option value is constructible. No provider-specific option, pricing, caching, routing, or fallback behavior is promised.

The HTTP-200 body is read and strictly validated up to **96 MiB**. The required `images` array may contain **0–16** strings independently of requested `Count`. Every string must be strict, padded, standard-alphabet base64; decoded images are returned as independently owned `[][]byte`, limited to **16 MiB each** and **64 MiB in aggregate**. `ImageResult` intentionally has no media-type field: the client neither infers nor promises an output format.

Result presence is preserved. `Retryable` is nil when `isRetryable` is absent and otherwise points to the exact boolean; JSON null is invalid. `Usage` is nil when absent or null, while a present empty object produces non-nil usage with nil fields. Present `inputTokens`, `outputTokens`, and `totalTokens` values must be finite numbers and may be negative, fractional, or zero; each field may be absent or null. Absent warnings normalize to a non-nil empty slice while null is invalid; warnings use the shared closed variants. Provider metadata is nil when absent, preserves an empty object, and requires non-null object values retained as defensive-copy raw JSON. `Response.ModelID` is caller supplied, `Headers` is a non-nil clone, and `Body` retains a defensive copy of at most the first **1 MiB** of the validated success body.

`GenerateImage` ignores `WithRetryPolicy`: it makes one HTTP attempt and performs no retry or batching. Nil or cancelled contexts, token resolution, sending, body reading, and closing follow the shared [cancellation and error rules](#cancellation-and-streams). Non-200 responses use `ResponseError` with at most 1 MiB of diagnostic body; transport/read failures use `TransportError`; local request failures use `ValidationError`; invalid HTTP-200 bodies use `ResponseValidationError`. Cancellation may be returned directly or wrapped and remains detectable with `errors.Is`.

This loopback-only snippet exercises the route and byte-input encoding without hosted traffic or billing:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost || r.URL.Path != "/v4/ai/image-model" {
        http.Error(w, "unexpected route", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"images":["aGk="],"warnings":[]}`)
}))
defer server.Close()

client, err := gateway.NewClient(
    gateway.WithAPIKey("loopback-only"),
    gateway.WithBaseURL(server.URL+"/v4/ai"),
)
if err != nil {
    log.Fatal(err)
}
result, err := client.GenerateImage(context.Background(), "example/dynamic-model", gateway.ImageRequest{
    Count: 1,
    Files: []gateway.ImageInput{
        gateway.ImageBytes{MediaType: "image/png", Data: []byte("local-input")},
    },
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("images=%d first_image_bytes=%d retained_body_bytes=%d\n",
    len(result.Images), len(result.Images[0]), len(result.Response.Body))
```

The snippet needs `context`, `fmt`, `io`, `log`, `net/http`, `net/http/httptest`, and the package import. It prints structural counts only. It proves local client behavior, not hosted success, model availability, universal support, release readiness, pricing, routing, fallback behavior, or output media type.

## Provider-protocol speech synthesis

`Client.GenerateSpeech(ctx, modelID, request)` is a staged, experimental Model V4 provider-protocol surface. It sends exactly one `POST` to `{WithBaseURL}/speech-model`, which is `/v4/ai/speech-model` at the default base, with `Ai-Speech-Model-Specification-Version: 4`. `modelID` is dynamic but must be a nonempty `provider/model` string. This is not a public v1 speech API, a direct-provider client, a JavaScript-parity claim, or evidence that every model or provider supports speech synthesis.

Request presence is exact. `SpeechRequest.Text` is always emitted, including when empty. `Voice`, `Instructions`, `Language`, and `OutputFormat` are omitted when empty; whitespace-only values are emitted unchanged. `Speed` is omitted only when nil: a non-nil finite value is emitted exactly, including zero or a negative value. Text and instructions must be valid UTF-8 and are each limited to **1 MiB**; voice, language, and output format must be valid UTF-8 and are each limited to **255 bytes**. The complete encoded JSON request is limited to **16 MiB**. Validation completes before cancellation, credential resolution, or network work.

`ProviderOptions` uses the shared sealed `[]ProviderOption` contract. Nil and empty slices omit `providerOptions`; this release exports no concrete implementation, so no supported nonempty option value is constructible. No provider-specific option semantics, service limits beyond the local bounds above, pricing, caching, routing, or fallback behavior is promised.

The HTTP-200 body is read and strictly validated up to **96 MiB**. It requires `audio` to be a JSON string of at most **64 MiB**. `SpeechResult.Audio` is that exact decoded JSON string: the SDK treats it as opaque, does **not** validate or decode it as base64, and exposes no media-type field or format inference.

Warnings and provider metadata use the shared strict presence rules: absent warnings normalize to a non-nil empty slice while null is invalid; absent provider metadata remains nil, a present empty object is preserved, and each provider value must be a non-null object retained as defensive-copy raw JSON. `Response.ModelID` is caller supplied, `Headers` is a non-nil clone, and `Body` retains a defensive copy of at most the first **1 MiB** of the validated body. Audio, warnings, metadata, headers, and the retained body do not share mutable storage with transport buffers.

`GenerateSpeech` ignores `WithRetryPolicy`: it makes one HTTP attempt and performs no retry or batching. A call may be billable and is not assumed idempotent. Nil or cancelled contexts, token resolution, sending, body reading and closing, response decoding, and return-time cancellation checks follow the shared [cancellation and error rules](#cancellation-and-streams). Non-200 responses use `ResponseError` with at most 1 MiB of diagnostic body; transport/read failures use `TransportError`; local request failures use `ValidationError`; invalid HTTP-200 bodies use `ResponseValidationError`. Cancellation may be returned directly or wrapped and remains detectable with `errors.Is`.

This loopback-only snippet exercises the exact route and opaque-audio boundary without hosted traffic or billing:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost || r.URL.Path != "/v4/ai/speech-model" {
        http.Error(w, "unexpected route", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"audio":"opaque-local-audio","warnings":[]}`)
}))
defer server.Close()

client, err := gateway.NewClient(
    gateway.WithAPIKey("loopback-only"),
    gateway.WithBaseURL(server.URL+"/v4/ai"),
)
if err != nil {
    log.Fatal(err)
}
speed := 0.0
result, err := client.GenerateSpeech(context.Background(), "example/dynamic-model", gateway.SpeechRequest{
    Text:         "local text",
    Voice:        "local voice",
    OutputFormat: "opaque-format-name",
    Speed:        &speed,
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("audio_bytes=%d warnings=%d retained_body_bytes=%d\n",
    len(result.Audio), len(result.Warnings), len(result.Response.Body))
```

The snippet needs `context`, `fmt`, `io`, `log`, `net/http`, `net/http/httptest`, and the package import. It prints structural counts only—not text, voice, audio, warnings, metadata, or raw bodies. It proves local client behavior only, not hosted success, live-service compatibility, model availability, universal support, release readiness, pricing, routing, fallback behavior, or an audio media type.

## Provider-protocol transcription

`Client.Transcribe(ctx, modelID, request)` is a buffered, staged, experimental Model V4 provider-protocol surface. It sends exactly one `POST` to `{WithBaseURL}/transcription-model`, which is `/v4/ai/transcription-model` at the default base, with `Ai-Transcription-Model-Specification-Version: 4`. `modelID` is dynamic but must be a nonempty `provider/model` string. This is not streaming transcription, a public v1 transcription API, a direct-provider client, a JavaScript-parity claim, or evidence of hosted or universal model/provider support.

`TranscriptionRequest.Audio` is required and accepts the sealed `TranscriptionBase64` and `TranscriptionBytes` variants as values or non-nil pointers; there is no URL or streaming-input variant. Both send `mediaType` as an opaque, valid-UTF-8 string of at most **255 bytes**; it may be empty and its grammar is not validated. `TranscriptionBase64.Data` is opaque valid UTF-8, may be empty, is limited to **1 MiB**, and is passed through without base64 validation, decoding, normalization, or re-encoding. `TranscriptionBytes.Data` may be nil or empty, is limited to **8 MiB**, and is standard-base64 encoded exactly once. The complete encoded JSON request is limited to **16 MiB**.

`ProviderOptions` uses the shared sealed `[]ProviderOption` contract. Nil and empty slices omit `providerOptions`; this release exports no concrete implementation, so no supported nonempty option value is constructible. No provider-specific option semantics, accepted media-type catalog, service limits beyond the local bounds above, pricing, caching, routing, fallback, or format conversion is promised.

The HTTP-200 body is read and strictly validated up to **16 MiB**. `text` is required, may be empty, and is limited to **1 MiB**. Absent `segments` normalizes to a non-nil empty slice; null is invalid; at most **4096** segments are accepted. Every segment requires `text` of at most **1 MiB** and finite `startSecond` and `endSecond` numbers. The SDK preserves negative values and does not impose ordering, non-overlap, or a relation between start and end. `language` is nil when absent or null and otherwise preserves an opaque string of at most **255 bytes**, including empty. `durationInSeconds` is nil when absent or null and otherwise preserves any finite number, including zero or a negative value.

Absent warnings normalize to a non-nil empty slice while null is invalid; warnings use the shared closed variants. Provider metadata is nil when absent, preserves an empty object, and requires non-null object values retained as defensive-copy raw JSON. `Response.ModelID` is caller supplied, `Headers` is a non-nil clone, and `Body` retains a defensive copy of at most the first **1 MiB** of the validated response. Returned text, segments, language, warnings, metadata, headers, and retained body do not alias transport buffers.

`Transcribe` ignores `WithRetryPolicy`: it makes one HTTP attempt and performs no retry, batching, stream, resume, or replay. A call may be billable and is not assumed idempotent. For a non-nil context, deterministic request validation precedes cancellation observation; cancellation is then checked before request copying, credential resolution, or network work. Nil or cancelled contexts, token resolution, sending, bounded body reading and closing, response decoding, and return-time cancellation checks follow the shared [cancellation and error rules](#cancellation-and-streams). Non-200 responses use `ResponseError` with at most 1 MiB of diagnostic body; transport/read failures use `TransportError`; local request failures use `ValidationError`; invalid HTTP-200 bodies use `ResponseValidationError`. Cancellation may be returned directly or wrapped and remains detectable with `errors.Is`.

This loopback-only snippet exercises the exact route, byte-input base64 encoding, and structural result fields without hosted traffic or billing:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost || r.URL.Path != "/v4/ai/transcription-model" {
        http.Error(w, "unexpected route", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"text":"local transcript","segments":[{"text":"local","startSecond":0,"endSecond":1}],"language":"en","durationInSeconds":1,"warnings":[]}`)
}))
defer server.Close()

client, err := gateway.NewClient(
    gateway.WithAPIKey("loopback-only"),
    gateway.WithBaseURL(server.URL+"/v4/ai"),
)
if err != nil {
    log.Fatal(err)
}
result, err := client.Transcribe(context.Background(), "example/dynamic-model", gateway.TranscriptionRequest{
    Audio: gateway.TranscriptionBytes{MediaType: "audio/wav", Data: []byte("local-audio")},
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("text_bytes=%d segments=%d language_present=%t duration_present=%t retained_body_bytes=%d\n",
    len(result.Text), len(result.Segments), result.Language != nil,
    result.DurationSeconds != nil, len(result.Response.Body))
```

The snippet needs `context`, `fmt`, `io`, `log`, `net/http`, `net/http/httptest`, and the package import. It prints structural facts only—not audio, transcript text, segment text/timestamps, language, warnings, metadata, or raw bodies. It proves local buffered client behavior only, not hosted success, live-service compatibility, model availability, universal support, release readiness, pricing, routing, fallback behavior, media-type acceptance, transcription quality, streaming, or public-v1 support.

## Provider-protocol embeddings

`Client.Embed(ctx, modelID, request)` is a staged, experimental Model V4 provider-protocol surface. It sends exactly one `POST` to `{WithBaseURL}/embedding-model`, which is `/v4/ai/embedding-model` at the default base. `modelID` is dynamic but must be a nonempty `provider/model` string. This is not a public `/v1/embeddings` API, a JavaScript-parity claim, or evidence that every model or provider supports embeddings.

`EmbeddingRequest.Values` represents both cases: one string for a single embedding, or multiple strings for a many-value request. One call accepts **1–4096** values and sends them together; the SDK never partitions them into batches. Each value is at most **1 MiB** (1,048,576 bytes), and the complete encoded JSON request is at most **16 MiB** (16,777,216 bytes). Empty strings are allowed. Validation happens before credential resolution and network work.

`ProviderOptions` is a sealed `[]ProviderOption`. Nil and empty slices both omit `providerOptions`; this release intentionally exports no concrete option implementation, so callers cannot construct a supported nonempty slice. This surface makes no provider-specific option, caching, pricing, routing, or fallback promise.

The HTTP-200 body is read and strictly validated up to **32 MiB**. It must contain exactly one vector per requested value, in order. Every vector has 1–65,536 finite numbers, with at most 4,194,304 elements across all vectors; unequal vector dimensions are not repaired or rejected merely for being unequal. Unknown or duplicate contract fields, malformed/trailing JSON, invalid collection bounds, and invalid discriminator shapes produce `ResponseValidationError` rather than best-effort decoding.

Response metadata is presence-preserving and strict:

- `Usage` is nil when `usage` is absent or JSON null. When present, it contains only a required, non-null `tokens` value that must be exactly representable as `int64`; `Tokens` is non-nil.
- An absent `warnings` key normalizes to a non-nil empty slice. JSON null is invalid. Each item must be exactly one closed variant: `unsupported` or `compatibility` with `feature` and optional string `details`; `deprecated` with `setting` and `message`; or `other` with `message`. Unknown types or fields are invalid.
- `ProviderMetadata` is nil when absent, preserves a present empty object, and stores each provider's value as defensive-copy `json.RawMessage`. Each value must be a non-null JSON object; nested arbitrary JSON is preserved subject to fixed depth, string, and collection bounds.
- `Response.ModelID` is the caller-supplied ID, `Headers` is a non-nil clone, and `Body` is a defensive copy of at most the first **1 MiB** of the already validated success body. The body may therefore be shorter than a valid response that was read and validated up to 32 MiB.

`Embed` ignores `WithRetryPolicy`: it makes one HTTP attempt and does not automatically retry or batch. A nil or cancelled context, token resolution, sending, body reading, and closing remain cancellation-aware under the shared [cancellation and error rules](#cancellation-and-streams). Non-200 responses use `ResponseError` with at most 1 MiB of diagnostic body; transport/read failures use `TransportError`; local request failures use `ValidationError`; invalid HTTP-200 bodies use `ResponseValidationError`. Raw metadata, vectors, inputs, identifiers, headers, and diagnostic bodies remain sensitive; follow [Privacy](#privacy).

This loopback-only snippet exercises the exact route without credentials, hosted traffic, or billing:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost || r.URL.Path != "/v4/ai/embedding-model" {
        http.Error(w, "unexpected route", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"embeddings":[[1,0],[0,1]],"warnings":[]}`)
}))
defer server.Close()

client, err := gateway.NewClient(
    gateway.WithAPIKey("loopback-only"),
    gateway.WithBaseURL(server.URL+"/v4/ai"),
)
if err != nil {
    log.Fatal(err)
}
result, err := client.Embed(context.Background(), "example/dynamic-model", gateway.EmbeddingRequest{
    Values: []string{"first", "second"},
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("vectors=%d retained_body_bytes=%d\n", len(result.Embeddings), len(result.Response.Body))
```

The snippet needs `context`, `fmt`, `io`, `log`, `net/http`, `net/http/httptest`, and the package import. It deliberately prints only structural counts. It proves local client behavior only, not hosted success, universal support, release readiness, pricing, or fallback behavior.

## Provider-protocol reranking

`Client.Rerank(ctx, modelID, request)` is a staged, experimental Model V4 provider-protocol surface. It sends exactly one `POST` to `{WithBaseURL}/reranking-model`, which is `/v4/ai/reranking-model` at the default base. `modelID` is dynamic but must be a nonempty `provider/model` string. This is not a public v1 API, a JavaScript-parity claim, or evidence that every model or provider supports reranking.

`RerankRequest` contains a query, optional `TopN`, and exactly one homogeneous document envelope: `RerankTexts{Values: []string{...}}` or `RerankObjects{Values: []json.RawMessage{...}}`. Value and non-nil pointer forms are accepted. One call accepts **1–4096** documents; the SDK never partitions them into batches. The query may be empty. The query and each text or raw object are at most **1 MiB** (1,048,576 bytes), and the complete encoded request is at most **16 MiB** (16,777,216 bytes). Each object must be one non-null, duplicate-free JSON object within the shared depth, string, and collection limits. It is compacted while preserving key order, numeric lexemes, and JSON escape spelling. `TopN`, when present, must be in `1..len(documents)`. Validation finishes before credential resolution and network work.

`ProviderOptions` has the same sealed contract as embeddings: nil and empty slices omit `providerOptions`, and no concrete option implementation is exported. No provider-specific option, caching, pricing, routing, fallback, or hosted-model support is promised.

The HTTP-200 body is read and strictly validated up to **32 MiB**. `ranking` may contain zero through the requested document count, and no more than `TopN` when supplied. Results retain provider order; each item requires a unique exact-integer index in range and a finite `float64` relevance score. There is no 0..1, nonnegative, monotonic, or descending-score guarantee. `OriginalIndex` identifies the source item and `Document` reconstructs its validated text or compacted JSON object as an independently owned `RerankText` or `RerankJSON`. Unknown or duplicate fields, malformed or trailing JSON, repeated or invalid indices, non-finite scores, and invalid bounds produce `ResponseValidationError` rather than repaired output.

Warnings and provider metadata use the same strict, presence-preserving rules described for embeddings: absent warnings normalize to a non-nil empty slice while null is invalid; absent provider metadata is nil while a present empty object is preserved; provider values are non-null JSON objects retained as defensive copies. `Response.ModelID` is the caller-supplied ID, `Headers` is a non-nil clone, and `Body` retains a defensive copy of at most the first **1 MiB** of the validated success body.

`Rerank` ignores `WithRetryPolicy`: it makes one HTTP attempt and does not automatically retry or batch. Nil and cancelled contexts and token resolution, sending, reading, and closing follow the shared [cancellation and error rules](#cancellation-and-streams). Non-200 responses use `ResponseError` with at most 1 MiB of diagnostic body; transport/read failures use `TransportError`; local request failures use `ValidationError`; invalid HTTP-200 bodies use `ResponseValidationError`.

This loopback-only snippet exercises the exact route without hosted traffic or billing:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost || r.URL.Path != "/v4/ai/reranking-model" {
        http.Error(w, "unexpected route", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"ranking":[{"index":1,"relevanceScore":-2},{"index":0,"relevanceScore":3}],"warnings":[]}`)
}))
defer server.Close()

client, err := gateway.NewClient(
    gateway.WithAPIKey("loopback-only"),
    gateway.WithBaseURL(server.URL+"/v4/ai"),
)
if err != nil {
    log.Fatal(err)
}
topN := 2
result, err := client.Rerank(context.Background(), "example/dynamic-model", gateway.RerankRequest{
    Query:     "local query",
    Documents: gateway.RerankTexts{Values: []string{"first", "second"}},
    TopN:      &topN,
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("results=%d first_original_index=%d retained_body_bytes=%d\n",
    len(result.Results), result.Results[0].OriginalIndex, len(result.Response.Body))
```

The snippet needs `context`, `fmt`, `io`, `log`, `net/http`, `net/http/httptest`, and the package import. It prints only structural counts and an input index—not source documents, scores, warnings, metadata, or raw bodies. It proves local client behavior only, not hosted success, model availability, universal support, release readiness, pricing, or fallback behavior.

## Retries

**Default: one attempt.** Evaluation and public generation can be billable and are not assumed idempotent. `GenerateImage`, `GenerateSpeech`, `Transcribe`, `Embed`, and `Rerank` are always one attempt regardless of this option.

`WithRetryPolicy` affects buffered `Evaluate`, `CreateResponse`, `CreateResponseWithBuiltInTools`, `CreateChatCompletion`, and `CreateChatCompletionWithServerTools`. These buffered calls can be billable on every attempt. `GenerateImage`, `GenerateSpeech`, `Transcribe`, `Embed`, and `Rerank` do not use this retry loop. Streaming methods do not use the SDK retry loop and never resume or replay a stream.

| `RetryPolicy` field | Meaning and accepted values |
| --- | --- |
| `MaxAttempts` | Includes the initial request. 0 or 1 means one attempt; 2–10 enables retries. |
| `InitialDelay` | With retries enabled, zero defaults to 100 ms; explicit values: 1 ms–1 minute. |
| `MaxDelay` | With retries enabled, zero defaults to 2 seconds; explicit values: 1 ms–5 minutes, at least the resolved initial delay. |
| `Multiplier` | With retries enabled, zero defaults to 2; explicit values: 1–10. |
| `Jitter` | Fraction 0–1; zero disables jitter. |

Only HTTP **408, 409, 429, and 500–599** are retryable, and only with a successfully read body, attempts remaining, and an active context. Transport, body-read, and response-validation failures are not retried. `ResponseError.Retryable()` reports status eligibility, not whether the client will actually retry.

Delays grow exponentially, with optional symmetric jitter and a `MaxDelay` cap. Valid `Retry-After` seconds or an HTTP date replaces that delay, still capped by `MaxDelay`. Waiting is context-cancellable. See [RetryPolicy and its bounds](../client.go) for the full contract.

## Errors

Only HTTP 200 is success. SDK-specific errors support `errors.As`:

| Type | Meaning | Useful accessors |
| --- | --- | --- |
| `ConfigurationError` | Missing credentials or invalid client option | `Option`, `Reason` |
| `ValidationError` | Invalid request, rejected before token-source or network work | `Path`, `Reason` |
| `TransportError` | Credential resolution, encoding, HTTP, body, or stream failure | `Operation`, `Unwrap` |
| `ResponseError` | Non-200 HTTP status, including redirects and other 2xx statuses | `StatusCode`, `Retryable`, `RetryAfter`, `BodyTruncated` |
| `ResponseValidationError` | Malformed, oversized, or contract-invalid buffered HTTP-200 body | `Path`, `Reason`, `BodyTruncated` |

Most existing buffered public-generation and evaluation success bodies are limited to 1 MiB. Transcription success bodies are read and validated up to 16 MiB; embedding and reranking bodies up to 32 MiB; and image and speech bodies up to 96 MiB. Their `Response.Body` fields retain at most the first 1 MiB. At status 200, overflow or invalid data is a `ResponseValidationError` where applicable; provider-protocol read-limit overflow and read/close failures are `TransportError`. At non-200, `ResponseError` preserves the status and up to 1 MiB of diagnostic body and wraps any body failure. Malformed stream data terminates iteration with a `TransportError` (`read response stream` or `read chat completion stream`). Cancellation may also return a context error directly or wrapped; use `errors.Is`.

Validation paths use `$["field"]` and array indexes. Paths, error envelopes, identifiers, and diagnostic bodies can contain sensitive caller or provider data. Log selected structural fields rather than dumping accessors:

```go
var responseErr *gateway.ResponseError
if errors.As(err, &responseErr) {
    log.Printf("gateway status=%d retryable=%t request_id_present=%t",
        responseErr.StatusCode(), responseErr.Retryable(), responseErr.RequestID() != "")
}
```

Error strings omit raw bodies and wrapped-error text. Accessors are nil-safe; `RawResponseBody` and raw JSON accessors return defensive copies. Full accessor definitions are in [errors.go](../errors.go).

## Cancellation and streams

Pass a non-nil context to every operation. Nil is a local `ValidationError`; cancellation covers token resolution, HTTP execution, body reads and closes, retry waits, provider response decoding, return-time checks, and stream reads. `GenerateImage`, `GenerateSpeech`, `Transcribe`, `Embed`, and `Rerank` have no retry wait, but their sole attempts and bounded response reads use the operation context. For `Transcribe` with a non-nil context, deterministic request validation occurs before cancellation is observed. Keep a streaming context alive until iteration finishes. A configured `http.Client.Timeout` also applies to body and stream reads.

Always `defer stream.Close()`, iterate with `Next()`, and check `Err()` afterward. `Event()` is valid only after a successful `Next()`. `Close()` is idempotent and can unblock a concurrent `Next()`; an explicit close is not proof that the server completed generation. Streams retain no transcript and start no producer goroutine. Responses and Chat have different termination rules; see [generation](generation.md).

## Privacy

Treat prompts/state, image URLs, opaque base64 and byte inputs, masks, decoded image outputs, image usage, speech text, voices, instructions, languages, output formats, speeds, opaque audio strings, transcription audio strings/bytes/media types, transcript text, segments and timestamps, transcription language and duration, embedding inputs and vectors, reranking queries, source and reconstructed documents, scores and rankings, generated output, file data, schemas, tool arguments/results, search configuration, warnings, provider options/metadata, raw JSON, response metadata bodies, headers, identifiers, and diagnostic bodies as sensitive. Defensive copies prevent mutation; **they do not sanitize content**. Never log credentials or authorization headers. The examples intentionally print structural summaries rather than sensitive values.

Paid-test authorization and evidence sanitization are separate from normal client configuration; see [live-contract evidence](evaluation-live-evidence.md).
