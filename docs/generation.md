# Public generation guide

This package implements Vercel AI Gateway's public `POST /v1/responses` and `POST /v1/chat/completions` endpoints, each in buffered and streaming form. These are generation APIs. They are distinct from the AI SDK Evaluation Model V4 provider protocol documented in [evaluation.md](evaluation.md), and they do not add the separate public `POST /v1/evaluate` API.

Hermetic loopback fixtures cover all four generation paths. The authorized paid live execution remains **NOT RUN / PENDING LIVE RUN** in [the sole live evidence record](evaluation-live-evidence.md). No hosted-service success is implied.

## Client, authentication, and endpoints

Create one `Client` with `NewClient`. Credential precedence is:

1. explicit `WithAPIKey`;
2. nonblank `AI_GATEWAY_API_KEY`;
3. the last explicit `WithOIDCToken` or `WithOIDCTokenSource`;
4. nonblank `VERCEL_OIDC_TOKEN`;
5. otherwise `ConfigurationError("credentials", "no credential configured")`.

There is no fallback after the selected credential is rejected. A selected token source is invoked once per HTTP attempt with the call's context; a source error or blank token is a `TransportError` whose operation is `resolve OIDC token`.

Generation defaults to `https://ai-gateway.vercel.sh/v1`. `WithPublicBaseURL` changes only that base, after removing trailing path slashes, and generation appends `/responses` or `/chat/completions`. It requires a whitespace-exact absolute non-opaque HTTP(S) URL with host and no userinfo, query, fragment, or literal fragment delimiter. Existing paths and escaped path data are retained. `WithBaseURL` is intentionally separate and changes only the provider evaluation endpoint.

Generation sends `Authorization: Bearer <credential>` and `Content-Type: application/json`; streaming also sends `Accept: text/event-stream`. `WithTeam` adds `X-Vercel-Ai-Gateway-Team`. `WithHeaders` clones allowed caller headers. It rejects the shared owned header set, including authorization, content type, team, and evaluation protocol headers, even though generation itself does not emit the evaluation-only headers. Redirects are returned rather than followed.

## Responses API

### Buffered call

```go
result, err := client.CreateResponse(ctx, gateway.ResponsesRequest{
    Model: "openai/gpt-5-nano",
    Input: gateway.ResponseTextInput("Explain Go contexts briefly."),
})
if err != nil {
    return err
}
fmt.Printf("response_bytes=%d\n", len(result.RawJSON()))
```

`CreateResponse` posts with `stream:false`. A status-200 body must be a single non-null JSON object with valid UTF-8, no duplicate object keys, bounded nesting/member/string sizes, and no trailing JSON value. `ResponseResult.RawJSON` returns a fresh copy; the package does not project the provider-specific response object into a larger typed result.

### Request fields

`ResponsesRequest` supports:

- required `Model`, which must have a nonempty `provider/model` shape;
- required `Input`, either nonempty `ResponseTextInput` or `ResponseItemsInput`;
- `MaxOutputTokens`, `Temperature` (0–2), `TopP` (0–1), finite presence/frequency penalties, and `Instructions`;
- function `Tools`, `ToolChoice` (`auto`, `required`, `none`, or `ResponseSpecificToolChoice`), `ParallelToolCalls`, and `AllowedTools`;
- `Reasoning` effort (`none`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`) and optional summary (`detailed`, `auto`, `concise`);
- text format `text`, `json_object`, or `ResponseJSONSchemaFormat`;
- truncation `auto` or `disabled`, previous response ID, `Store`, and up to 16 metadata entries;
- Gateway cache controls: `Caching` may be `auto`, `CacheTTL` may be `5m` or `1h` only when caching is auto, plus nonnegative `CacheAnchorItems` and a prompt-cache key of at most 64 characters.

`ResponseItemsInput` accepts `ResponseMessage` with role `user`, `assistant`, `system`, or `developer`; `ResponseFunctionCall`; and `ResponseFunctionCallOutput`. Tool parameters and JSON schemas must be JSON-compatible, finite, acyclic, string-keyed, and within the documented resource bounds. The SDK serializes tools but never invokes them.

### Streaming call

```go
stream, err := client.StreamResponse(ctx, request)
if err != nil {
    return err
}
defer stream.Close()

textDeltaEvents := 0
rawEvents := 0
for stream.Next() {
    switch stream.Event().(type) {
    case gateway.ResponseOutputTextDeltaEvent:
        textDeltaEvents++
    case gateway.RawResponseEvent:
        rawEvents++
    }
}
if err := stream.Err(); err != nil {
    return err
}
fmt.Printf("text_delta_events=%d raw_events=%d\n", textDeltaEvents, rawEvents)
```

`StreamResponse` sends the same request with `stream:true`. `ResponseOutputTextDeltaEvent` exposes the JSON type, SSE event name and ID, and text delta. Other valid JSON event objects are retained as `RawResponseEvent`. Clean, correctly framed EOF terminates successfully. Invalid framing/UTF-8/JSON, a missing event type, resource-limit violations, cancellation, or body read/close failure terminates with `TransportError` operation `read response stream`.

The stream starts no producer goroutine and retains no transcript. `Event` is valid only after `Next` returns true. `Err` is stable after termination. `Close` is idempotent, releases the body exactly once, and may unblock a concurrent `Next`. At most 10,000 events are accepted.

## Chat Completions API

### Buffered call

```go
result, err := client.CreateChatCompletion(ctx, gateway.ChatCompletionRequest{
    Model: "openai/gpt-5-nano",
    Messages: []gateway.ChatMessage{
        {Role: "user", Content: gateway.ChatTextContent("Explain Go contexts briefly.")},
    },
})
if err != nil {
    return err
}
contentChoices := 0
for _, choice := range result.Choices.Value {
    if choice.Message.Present && choice.Message.Value.Content.Present {
        contentChoices++
    }
}
fmt.Printf("choices=%d content_choices=%d\n", len(result.Choices.Value), contentChoices)
```

`CreateChatCompletion` posts with `stream:false`. `ChatCompletionResult` provides absence/null/value-preserving `JSONField` values for ID, object, creation time, model, choices, assistant content/tool calls, finish reason, and usage. `RawJSON` returns a fresh copy of the complete bounded response. Fixed typed fields are validated, while unmodeled response fields remain available in the raw JSON.

### Request fields

`ChatCompletionRequest` supports:

- required nonempty `Model` and at least one `Message`;
- roles `system`, `developer`, `user`, and `assistant`;
- `ChatTextContent` or `ChatPartsContent` containing text, image URL, or inline file parts; image detail is `auto`, `low`, or `high`;
- temperature 0–2, top-p 0–1, frequency/presence penalties -2–2, optional `MaxTokens`, one stop string or a string list, and nonempty safety identifier;
- function tools with non-null object parameters; tool choice `auto`, `none`, or a named function;
- response format `text`, `json`, `ChatJSONSchemaResponseFormat`, or `ChatLegacyJSONResponseFormat`;
- fallback `Models`;
- `ProviderOptions.Gateway` routing order/models/sort and BYOK provider timeouts; sort is `cost`, `ttft`, or `tps`, and BYOK timeout values are 1000–789000 milliseconds;
- legacy `Provider.Sort`, which must match `ProviderOptions.Gateway.Sort` when both are set.

Strings, arrays, objects, schemas, and responses are bounded: individual strings are at most 1 MiB, JSON depth at most 64, and collection/member/event counts at most 10,000. Validation paths use canonical `$["field"]` notation and fail before credential-source or network work.

### Streaming call

```go
stream, err := client.StreamChatCompletion(ctx, request)
if err != nil {
    return err
}
defer stream.Close()

chunks := 0
choices := 0
for stream.Next() {
    chunks++
    choices += len(stream.Event().Choices)
}
if err := stream.Err(); err != nil {
    return err
}
fmt.Printf("chunks=%d choices=%d\n", chunks, choices)
```

`StreamChatCompletion` sends `stream:true`. Each event is a validated `chat.completion.chunk`; `ChatCompletionChunk` exposes ordered choices and their text delta, while `RawJSON` retains the complete chunk. The exact `data: [DONE]` marker is required. EOF before `[DONE]`, malformed framing/JSON, the wrong object discriminator, resource-limit violations, cancellation, or body read/close failure becomes `TransportError` operation `read chat completion stream`.

As with Responses, there is no producer goroutine or transcript. `Event` is valid only after successful `Next`; `Err` is stable; `Close` is idempotent and may unblock `Next`; and the stream accepts at most 10,000 events.

## Status handling, retries, and errors

Only HTTP 200 is success for all four generation methods. Before a stream is returned, a non-200 body is consumed and returned as `ResponseError`. Buffered status-200 body failures are `TransportError` or `ResponseValidationError`; malformed streamed events are stream-terminal `TransportError` values.

Buffered `CreateResponse` and `CreateChatCompletion` use `RetryPolicy`. `MaxAttempts` includes the initial request: 0 and 1 mean one attempt; 2–10 opt in. Enabled zero defaults are 100 ms initial delay, 2 s maximum delay, and multiplier 2; jitter zero disables jitter. Only 408, 409, 429, and 500–599 are retryable. A valid `Retry-After` replaces the calculated delay up to `MaxDelay`. Body-read failures are not retried. Streaming calls do not retry after headers and never resume or replay a partial stream. Retrying can duplicate billable generation work.

All five exported error types support `errors.As`:

- `ConfigurationError`: invalid construction option or absent credential;
- `ValidationError`: local request failure with canonical path and stable reason;
- `TransportError`: credential, encoding, request, response-body, or stream-read failure;
- `ResponseError`: non-200 status with parsed error/ID/retry metadata and bounded diagnostic body;
- `ResponseValidationError`: invalid status-200 buffered response.

`ResponseError` exposes status, message/type/code/param, generation/request/response IDs, retry metadata, truncation, raw diagnostic bytes, and its cause. Error strings deliberately omit raw bodies and wrapped-error text.

## Cancellation and resource ownership

Passing a nil context returns `ValidationError` at `$["context"]` before token or network work. An already-canceled context fails before sending. Cancellation propagates through token resolution, HTTP execution, bounded response reads, retry waits, and blocking stream reads. For streaming calls, keep the context alive until iteration finishes and always close the stream. The configured `http.Client.Timeout` also applies; caller-side mutation of a supplied client or transport follows `net/http` concurrency rules.

## Privacy and logging

Treat all of the following as sensitive: prompts and instructions; message text; image URLs; inline file data and filenames; tool definitions, arguments, and results; schemas; metadata and cache keys; model/provider routing; successful raw JSON; raw stream events/chunks; response headers; IDs; usage; and `ResponseError.RawResponseBody`. Raw accessors return defensive copies, not sanitized data. Prefer typed fields and explicitly selected structural metadata in logs. Never record credentials or authorization headers.

## Migration from evaluation-only use

Generation is additive. Existing evaluation code keeps using `Evaluate` and `WithBaseURL`; those names and the provider endpoint do not change. New callers select one of the four generation methods and optionally configure `WithPublicBaseURL`. Setting only `WithBaseURL` does not redirect generation, and setting only `WithPublicBaseURL` does not redirect evaluation. There is no endpoint inference, compatibility alias, or automatic conversion between Responses and Chat request types.

## Evidence boundary and unsupported categories

The hermetic fixture gate exercises buffered and streaming Responses and Chat calls entirely over loopback and rejects non-loopback destinations. The hosted live contract is still blocked because no owner-authorized paid-network execution, protected-environment run, or sanitized result exists. The exact public-generation live command and acknowledgement are recorded in [evaluation-live-evidence.md](evaluation-live-evidence.md); the evaluation command and acknowledgement are separate, and neither result can stand in for the other.

The package exports no public `/v1/evaluate`, credits, spend, generation lookup, model discovery, embeddings, image/video generation, reranking, speech, transcription, realtime, batch, agent, automatic-tool, UI, or search API. In particular, generic tool serialization does not imply provider-executed search support. Gateway search categories, including native xAI `x_search`, lack an exported contract and authorized live evidence here and remain unsupported; see [x-search.md](x-search.md).
