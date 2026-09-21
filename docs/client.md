# Client configuration

Shared behavior for [generation](generation.md), [provider evaluation](evaluation.md), and the staged provider-protocol `Client.Embed` surface described below. `NewClient` applies options in order, stops at the first error, and makes no network request.

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
| `WithBaseURL` | `https://ai-gateway.vercel.sh/v4/ai` | `Evaluate` and `Embed` |

Pass the base, not the full endpoint: methods append `/responses`, `/chat/completions`, `/evaluation-model`, or `/embedding-model`. Both options require an absolute HTTP(S) URL with a host, no surrounding whitespace, and no userinfo, query, or fragment. Existing paths and escapes are retained; trailing path slashes are removed. `WithPublicBaseURL` also rejects an empty literal `#` delimiter. No endpoint inference or cross-surface conversion occurs.

Other options:

- `WithHTTPClient`: accepts a non-nil `*http.Client`; retains the pointer and uses its transport, cookie jar, and timeout. It does not mutate the supplied client. **Redirects are never followed**, regardless of its `CheckRedirect`. Subsequent mutation follows `net/http` concurrency rules.
- `WithTeam`: sets `X-Vercel-Ai-Gateway-Team`; requires a nonblank value and has no environment fallback.
- `WithHeaders`: clones the supplied headers at construction and per request; nil is allowed. Authorization, content type, team, and provider-protocol headers are SDK-owned and rejected case-insensitively even when their values are empty. See the protected list in [client.go](../client.go).
- `WithRetryPolicy`: copies and validates the policy described below.

All requests use bearer authentication and JSON. Streams request `text/event-stream`. Provider evaluation and embeddings send their matching `/v4` protocol headers and the caller-supplied model ID; no fixed embedding-model catalog is built into the client.

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

## Retries

**Default: one attempt.** Evaluation and generation can be billable and are not assumed idempotent. `Embed` is always one attempt regardless of this option.

`WithRetryPolicy` affects buffered `Evaluate`, `CreateResponse`, `CreateResponseWithBuiltInTools`, `CreateChatCompletion`, and `CreateChatCompletionWithServerTools`. These buffered calls can be billable on every attempt. `Embed` does not use this retry loop. Streaming methods do not use the SDK retry loop and never resume or replay a stream.

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

Most existing buffered generation and evaluation success bodies are limited to 1 MiB. Embedding success is instead read and validated up to 32 MiB, while `EmbeddingResult.Response.Body` retains at most its first 1 MiB. At status 200, overflow or invalid data is a `ResponseValidationError` where applicable; embedding read-limit overflow and read/close failures are `TransportError`. At non-200, `ResponseError` preserves the status and up to 1 MiB of diagnostic body and wraps any body failure. Malformed stream data terminates iteration with a `TransportError` (`read response stream` or `read chat completion stream`). Cancellation may also return a context error directly or wrapped; use `errors.Is(err, context.Canceled)` or `context.DeadlineExceeded` as appropriate.

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

Pass a non-nil context to every operation. Nil is a local `ValidationError`; cancellation covers token resolution, HTTP execution, body reads, retry waits, and stream reads. `Embed` has no retry wait but its sole attempt and bounded response read use the operation context. Keep a streaming context alive until iteration finishes. A configured `http.Client.Timeout` also applies to body and stream reads.

Always `defer stream.Close()`, iterate with `Next()`, and check `Err()` afterward. `Event()` is valid only after a successful `Next()`. `Close()` is idempotent and can unblock a concurrent `Next()`; an explicit close is not proof that the server completed generation. Streams retain no transcript and start no producer goroutine. Responses and Chat have different termination rules; see [generation](generation.md).

## Privacy

Treat prompts/state, embedding inputs and vectors, generated output, image URLs, file data, schemas, tool arguments/results, search configuration, provider options/metadata, raw JSON, response metadata bodies, headers, identifiers, and diagnostic bodies as sensitive. Defensive copies prevent mutation; **they do not sanitize content**. Never log credentials or authorization headers. The examples intentionally print structural summaries rather than model output or embedding values.

Paid-test authorization and evidence sanitization are separate from normal client configuration; see [live-contract evidence](evaluation-live-evidence.md).
