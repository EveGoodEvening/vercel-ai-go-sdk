# Client configuration

Shared behavior for [generation](generation.md) and [provider evaluation](evaluation.md). `NewClient` applies options in order, stops at the first error, and makes no network request.

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
| `WithBaseURL` | `https://ai-gateway.vercel.sh/v4/ai` | `Evaluate` only |

Pass the base, not the full endpoint: methods append `/responses`, `/chat/completions`, or `/evaluation-model`. Both options require an absolute HTTP(S) URL with a host, no surrounding whitespace, and no userinfo, query, or fragment. Existing paths and escapes are retained; trailing path slashes are removed. `WithPublicBaseURL` also rejects an empty literal `#` delimiter. No endpoint inference or cross-surface conversion occurs.

Other options:

- `WithHTTPClient`: accepts a non-nil `*http.Client`; retains the pointer and uses its transport, cookie jar, and timeout. It does not mutate the supplied client. **Redirects are never followed**, regardless of its `CheckRedirect`. Subsequent mutation follows `net/http` concurrency rules.
- `WithTeam`: sets `X-Vercel-Ai-Gateway-Team`; requires a nonblank value and has no environment fallback.
- `WithHeaders`: clones the supplied headers at construction and per request; nil is allowed. Authorization, content type, team, and evaluation-protocol headers are SDK-owned and rejected case-insensitively even when their values are empty. See the protected list in [client.go](../client.go).
- `WithRetryPolicy`: copies and validates the policy described below.

All requests use bearer authentication and JSON. Streams request `text/event-stream`. Only provider evaluation sends the `/v4` protocol headers.

## Retries

**Default: one attempt.** Evaluation and generation can be billable and are not assumed idempotent.

`WithRetryPolicy` affects buffered `Evaluate`, `CreateResponse`, `CreateResponseWithBuiltInTools`, `CreateChatCompletion`, and `CreateChatCompletionWithServerTools`. These buffered calls can be billable on every attempt. Streaming methods do not use the SDK retry loop and never resume or replay a stream.

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

Buffered bodies are limited to 1 MiB. At status 200, overflow is a `ResponseValidationError`; read/close failures are `TransportError`. At non-200, `ResponseError` preserves the status and wraps any body failure. Malformed stream data terminates iteration with a `TransportError` (`read response stream` or `read chat completion stream`). Cancellation may also return a context error directly; use `errors.Is(err, context.Canceled)` or `context.DeadlineExceeded` as appropriate.

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

Pass a non-nil context to every operation. Nil is a local `ValidationError`; cancellation covers token resolution, HTTP execution, body reads, retry waits, and stream reads. Keep the context alive until stream iteration finishes. A configured `http.Client.Timeout` also applies to body and stream reads.

Always `defer stream.Close()`, iterate with `Next()`, and check `Err()` afterward. `Event()` is valid only after a successful `Next()`. `Close()` is idempotent and can unblock a concurrent `Next()`; an explicit close is not proof that the server completed generation. Streams retain no transcript and start no producer goroutine. Responses and Chat have different termination rules; see [generation](generation.md).

## Privacy

Treat prompts/state, generated output, image URLs, file data, schemas, tool arguments/results, search configuration, provider options/metadata, raw JSON, headers, identifiers, and diagnostic bodies as sensitive. Defensive copies prevent mutation; **they do not sanitize content**. Never log credentials or authorization headers. The examples intentionally print structural summaries rather than model output.

Paid-test authorization and evidence sanitization are separate from normal client configuration; see [live-contract evidence](evaluation-live-evidence.md).
