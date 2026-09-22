# Vercel AI Go SDK

Experimental Go client for Vercel AI Gateway text generation, evaluation, and staged provider-protocol image generation, speech synthesis, transcription, embeddings, and reranking. Requires **Go 1.26+**; the package name is `gateway`.

| API | Methods | Result support |
| --- | --- | --- |
| Public `/v1/responses` | `CreateResponse`, `StreamResponse`; opt-in `CreateResponseWithBuiltInTools`, `StreamResponseWithBuiltInTools` | Raw buffered JSON; typed text deltas and raw streaming events |
| Public `/v1/chat/completions` | `CreateChatCompletion`, `StreamChatCompletion` | Typed buffered fields; streamed text deltas; raw JSON access |
| Provider `/v4/ai/evaluation-model` | `Evaluate` | Validated boolean, choice, and score answers |
| Provider `/v4/ai/image-model` | `GenerateImage` | Strictly validated decoded image bytes, retryability, usage, warnings, provider metadata, and bounded response metadata |
| Provider `/v4/ai/speech-model` | `GenerateSpeech` | Strictly validated opaque audio string, warnings, provider metadata, and bounded response metadata |
| Provider `/v4/ai/transcription-model` | `Transcribe` | Strictly validated transcript text, segments, optional language and duration, warnings, provider metadata, and bounded response metadata |
| Provider `/v4/ai/embedding-model` | `Embed` | Strictly validated vectors, usage, warnings, provider metadata, and bounded response metadata |
| Provider `/v4/ai/reranking-model` | `Rerank` | Strictly validated provider-ordered rankings with reconstructed input documents, warnings, provider metadata, and bounded response metadata |

**Unreleased:** hermetic fixtures cover these APIs. The full credentialed hosted generation and evaluation contract suites have **not run**, and no hosted image, speech, transcription, embedding, or reranking success is claimed; the only hosted generation evidence is narrow sanitized structural corroboration for the documented Responses search declarations on their exact routes, which does not establish general generation compatibility or configurable-option semantics. See [search evidence](docs/x-search.md#sanitized-structural-evidence), [live evidence](docs/evaluation-live-evidence.md), and [release readiness](docs/releasing.md). This is not a full port of the JavaScript AI SDK or a general OpenAI client.

## Quick start

```sh
go get github.com/EveGoodEvening/vercel-ai-go-sdk
```

Configure credentials explicitly with `gateway.WithAPIKey(...)`, or set `AI_GATEWAY_API_KEY` (or `VERCEL_OIDC_TOKEN`) in the environment. The example below uses environment discovery. Running it sends a real request and may incur charges.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    gateway "github.com/EveGoodEvening/vercel-ai-go-sdk"
)

func main() {
    client, err := gateway.NewClient()
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := client.CreateResponse(ctx, gateway.ResponsesRequest{
        Model: "openai/gpt-5-nano",
        Input: gateway.ResponseTextInput("Explain Go contexts briefly."),
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("response completed: raw_json_bytes=%d\n", len(result.RawJSON()))
}
```

`RawJSON()` returns a defensive copy, not sanitized data. The example prints only its size; see the generation guide for Chat fields and streaming text access.

## Guides and examples

- [Generation](docs/generation.md): Responses, Chat, streaming, function tools, exact request-only Responses search, and separate Chat server search.
- [Evaluation](docs/evaluation.md): shared state, keyed questions, typed answers, and response validation.
- [Client configuration](docs/client.md): credentials, endpoints, retries, errors, cancellation, privacy, and the staged provider-protocol image, speech, transcription, embedding, and reranking contracts.
- Runnable examples: [generation](examples/generate/main.go) and [evaluation](examples/evaluate/main.go); the client guide includes loopback-only image, speech, transcription, embedding, and reranking snippets.
- Maintainers: [contributing](CONTRIBUTING.md), [changelog](CHANGELOG.md), and [release policy](docs/releasing.md).

## Important boundaries

- **Separate endpoints:** `WithPublicBaseURL` configures public generation; `WithBaseURL` configures provider evaluation and staged provider-protocol image generation, speech synthesis, transcription, embeddings, and reranking. Neither redirects the other. `Evaluate` does **not** call public `/v1/evaluate`, `Embed` does **not** call public `/v1/embeddings`, and `GenerateImage`, `GenerateSpeech`, `Transcribe`, and `Rerank` do not call public v1 endpoints.
- **Image generation is staged and exact-attempt.** `GenerateImage` appends `/image-model` to `WithBaseURL` and accepts a dynamic `provider/model` ID. `Count` is 1–16. Pointer fields preserve presence, except present-empty `Size` and `AspectRatio` and present-zero `Seed` are omitted; nil versus empty `Files` is preserved. Inputs are `ImageURL`, opaque `ImageBase64`, or `ImageBytes` (encoded by the SDK), accepted as values or non-nil pointers; the SDK does not fetch URLs or upload files. The encoded request is limited to 16 MiB. One attempt is made with no retry or batching. Successful bodies are strictly validated up to 96 MiB and may return 0–16 images independently of `Count`; outputs must be strict standard padded base64 and decode to at most 16 MiB each and 64 MiB total. Response metadata retains at most 1 MiB. Usage, retryability, warnings, and provider metadata preserve their documented optional presence. Provider options are sealed and have no exported concrete values, and `ImageResult` intentionally has no media-type field.
- **Speech synthesis is staged and exact-attempt.** `GenerateSpeech` appends `/speech-model` to `WithBaseURL` and accepts a dynamic `provider/model` ID. `Text` is always emitted, including when empty, and is limited to 1 MiB. `Voice`, `Instructions`, `Language`, and `OutputFormat` omit empty strings but emit whitespace and are limited to 255 bytes, 1 MiB, 255 bytes, and 255 bytes respectively. A non-nil `Speed` preserves zero and negative values but must be finite. The encoded request is limited to 16 MiB. One attempt is made with no retry or batching. Successful bodies are strictly validated up to 96 MiB; `Audio` is an opaque string limited to 64 MiB—not base64-decoded or assigned a media type—and response metadata retains at most 1 MiB. Warnings and provider metadata use the shared strict presence rules. Local validation precedes credentials and network work; cancellation and typed error behavior follow the shared client rules. No public v1 speech API, JavaScript parity, universal model/provider support, concrete provider options, hosted success, or live-service compatibility is claimed.
- **Transcription is buffered, staged, and exact-attempt.** `Transcribe` appends `/transcription-model` to `WithBaseURL` and accepts a dynamic `provider/model` ID. `TranscriptionBase64` passes through an opaque valid-UTF-8 string of at most 1 MiB without decoding or normalization; `TranscriptionBytes` accepts at most 8 MiB and is standard-base64 encoded exactly once. Both carry an opaque media type of at most 255 bytes; empty data and media types are allowed, and there is no URL or streaming-input variant. The encoded request is limited to 16 MiB. One attempt is made with no retry, batching, streaming, resume, or replay. Successful bodies are strictly validated up to 16 MiB and return required text, up to 4096 timestamped segments, optional language and duration, warnings, provider metadata, and at most 1 MiB of retained response body. Finite timestamps and duration are preserved without nonnegative, ordering, or non-overlap guarantees. This is not public-v1 or streaming transcription, and it makes no JavaScript-parity, hosted-success, live-service, universal-support, pricing, fallback, accepted-media-type, or transcription-quality claim.
- **Embedding is staged and exact-attempt.** `Embed` appends `/embedding-model` to `WithBaseURL`, accepts a dynamic `provider/model` ID and 1–4096 strings in one request, and validates the complete request before credential resolution or network work. Each value is limited to 1 MiB and encoded requests to 16 MiB; the SDK makes one attempt with no batching or retry. Successful bodies are strictly validated up to 32 MiB. Whole `usage` absent or null becomes nil; present usage requires a non-null integral `tokens` exactly representable as `int64`. Absent `warnings` becomes a non-nil empty slice while null is invalid; absent provider metadata remains nil, a present empty object is preserved, and each provider value is retained as defensive-copy raw JSON. `ResponseMetadata.Body` retains at most the first 1 MiB.
- **Reranking is staged and exact-attempt.** `Rerank` appends `/reranking-model` to `WithBaseURL`, accepts a dynamic `provider/model` ID, a query, and exactly one homogeneous `RerankTexts` or `RerankObjects` envelope containing 1–4096 documents. Query, text, and raw object inputs are each limited to 1 MiB; objects must be non-null duplicate-free JSON objects; the encoded request is limited to 16 MiB. `TopN`, when present, is in `1..len(documents)`. The SDK makes one attempt with no batching or retry. A successful body is strictly validated up to 32 MiB; provider order is retained, indices are unique and reconstruct the original input document, and scores may be any finite `float64`—they are not promised to be in 0..1 or sorted. Response metadata retains at most the first 1 MiB. No concrete `ProviderOption` is exported.
- **Retries are off by default.** Supported buffered generation and evaluation calls can opt in; retries may duplicate billable work. `GenerateImage`, `GenerateSpeech`, `Transcribe`, `Embed`, and `Rerank` never use the retry policy. Streams are not resumed or replayed. Always close a stream and check its final `Err()`.
- **Search declarations are request-only and server-executed.** Responses opt-in built-in-tools methods accept fixed low-context `ResponseWebSearchTool{}`, fieldless `ResponseXSearchTool{}`, and `ResponseXSearchOptionsTool` with six presence-preserving request fields: allowed/excluded X handles, from/to dates, and image/video understanding booleans. The exact evidenced routes are `openai/gpt-5.4-mini` and `spacexai/grok-4.6`; the catalog remains dynamic. Only simultaneous non-empty allowed/excluded handle lists receive option-specific local validation. Option semantics, wider-model behavior, fallback, typed search outputs, and typed lifecycle events remain unsupported. Chat separately accepts four `vercel:...` server tools through its opt-in methods. The SDK executes none of these tools automatically. See [search support](docs/x-search.md) for exact presence and output boundaries and [client configuration](docs/client.md) for shared authentication, retries, errors, and privacy behavior.

Not implemented:

- Public `/v1/evaluate`; `web_search_preview`, non-low or omitted web-search context, other current/preview forms, and undocumented search options; wider model compatibility, search-specific tool choice/`allowed_tools`, Gateway fallback behavior, configurable `x_search` semantics beyond supported request serialization, and typed search outputs/events.
- The internal `/v4/ai/language-model` protocol, public `/v1/embeddings`, and any public v1 image, speech, transcription, or reranking endpoint are outside this SDK; direct-provider clients are also outside it.
- Video generation, streaming transcription, realtime, batches, and management APIs (credits, spend, generation lookup, model discovery). Provider-protocol image generation, speech synthesis, buffered transcription, embeddings, and reranking are implemented experimentally; no universal model support, concrete provider options, batching, caching, pricing, fallback behavior, hosted success, or output media-type inference is claimed.
- Agents, orchestration, automatic function-tool execution, UI helpers, schema frameworks, and a global provider registry.
