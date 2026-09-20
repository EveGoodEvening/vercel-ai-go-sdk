# Vercel AI Gateway Go SDK

Experimental Go client for Vercel AI Gateway's **AI SDK Evaluation Model V4 provider protocol**. It calls `POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model`; it is not a client for the separate public REST `POST /v1/evaluate` API and is not an OpenAI-compatible wrapper.

The module declares Go 1.26 as its minimum (`go 1.26` in `go.mod`). The intended support window is the maintained Go 1.26 and 1.27 families. Go 1.26.8 and Go 1.27.1 are the planned validation targets; until Chunk 11 adds the pinned CI matrix, validation of those toolchains is manual.

Evaluation is experimental and compatibility-sensitive. The contract is pinned to `ai@7.0.107`, `@ai-sdk/gateway@4.0.87`, `@ai-sdk/provider@4.0.17`, and `@ai-sdk/provider-utils@5.0.45`. This project remains at v0 and does not claim parity with JavaScript `ai` or `@ai-sdk/gateway`.

## Install

```sh
go get github.com/EveGoodEvening/vercel-ai-gateway-go-sdk
```

A public release remains blocked until the repository owner selects and commits a license.

## Evaluation

```go
client, err := gateway.NewClient() // resolves credentials from the environment
if err != nil {
    log.Fatal(err)
}

result, err := client.Evaluate(ctx, "typesafe-ai/jev-latest", gateway.EvaluationRequest{
    State: map[string]any{"answer": "Paris"},
    Questions: map[string]gateway.Question{
        "correct": gateway.BooleanQuestion{
            Instructions: "Is the answer factually correct?",
        },
    },
})
```

See the [complete evaluation guide](docs/evaluation.md) and [runnable example](examples/evaluate/main.go). The authorized paid live-contract run is still pending; the [live evidence record](docs/evaluation-live-evidence.md) accurately records `NOT RUN` and must not be read as a pass.

## Authentication and options

Credential precedence is strict:

1. explicit `WithAPIKey`;
2. `AI_GATEWAY_API_KEY`;
3. explicit `WithOIDCToken` or `WithOIDCTokenSource` (last explicit OIDC form wins);
4. `VERCEL_OIDC_TOKEN`;
5. otherwise `NewClient` returns `*ConfigurationError` with `Option()=="credentials"`.

A nonblank environment API key therefore beats explicit OIDC. There is no fallback after a selected credential is rejected. Blank environment credentials are absent; accepted nonblank credential strings are preserved exactly. A selected token source is called once per HTTP attempt with the `Evaluate` context; source errors or blank results are `*TransportError` values with operation `resolve OIDC token`.

Options are applied in order and stop at the first error:

- `WithBaseURL` requires a nonblank, whitespace-exact, absolute non-opaque HTTP(S) URL with host and without userinfo, query, or fragment. Existing paths are allowed. Only trailing path slashes are removed before `/evaluation-model` is appended; the remaining URL is preserved.
- `WithHTTPClient` rejects nil, stores the supplied pointer, and neither clones nor mutates it. Caller-side concurrent use and mutation follow `http.Client` rules.
- `WithAPIKey`, `WithOIDCToken`, and `WithTeam` reject empty/all-Unicode-whitespace input and otherwise preserve it exactly. There is no environment team fallback.
- `WithOIDCTokenSource` rejects nil and typed-nil sources.
- `WithHeaders(nil)` is valid. Non-nil headers are cloned when configured and again per request; later caller mutation cannot affect the client. Values, order, duplicates, nil/empty slices, and empty strings are preserved.
- Caller headers may not contain, case-insensitively, `Authorization`, `Content-Type`, `Ai-Gateway-Protocol-Version`, `Ai-Gateway-Auth-Method`, `Ai-Evaluation-Model-Specification-Version`, `Ai-Model-Id`, or `X-Vercel-Ai-Gateway-Team`, even with no values.
- `WithRetryPolicy` copies and validates the value; details are below.

`Evaluate(nil, ...)` fails locally with `*ValidationError` at `$["context"]`, reason `required`, before token-source or network work.

## Wire protocol

Every request is `POST {baseURL}/evaluation-model` with:

- `Authorization: Bearer <credential>`
- `Content-Type: application/json`
- `ai-gateway-protocol-version: 0.0.1`
- `ai-gateway-auth-method: api-key` or `oidc`
- `ai-evaluation-model-specification-version: 4`
- `ai-model-id: <modelID>`
- optional `x-vercel-ai-gateway-team`

The JSON body contains only `state`, `questions`, and optional `providerOptions`; it never contains `model`. Only HTTP 200 is success. Every other status, including other 2xx statuses, is a `*ResponseError`. Response `Content-Type` is deliberately ignored because intermediaries may omit or rewrite it.

## Retries

Retries are off by default because evaluation may be billable and non-idempotent. `MaxAttempts` counts the initial request: 0 and 1 mean one attempt; 2–10 opt in. With retries enabled, zero values resolve to `InitialDelay=100ms`, `MaxDelay=2s`, and `Multiplier=2`; `Jitter=0` disables jitter. Explicit bounds are: initial delay 1ms–1m, maximum delay 1ms–5m and not below initial, multiplier 1–10, jitter 0–1. Durations are Go `time.Duration` values.

Only status 408, 409, 429, or 500–599 is retryable. Gateway `error.type` and `error.code` never change that status-only decision. A successfully parsed `Retry-After` controls the wait (capped at `MaxDelay`); waits are context-cancellable. Read/overflow/close failures on non-200 bodies are not retried. Opting in accepts possible duplicate billable work.

## Typed errors

All five types work with `errors.As`; accessors are nil-safe and return their documented zero values:

| Type | Role and structured accessors |
| --- | --- |
| `ConfigurationError` | Construction failure: `Option`, `Reason`. Option/reason values are closed and documented in the evaluation guide. |
| `ValidationError` | Local request failure: canonical `Path`, stable `Reason`. |
| `TransportError` | Token, encoding, request, HTTP, or body-read failure: closed `Operation`, `Unwrap`. |
| `ResponseError` | Non-200 response: status, parsed envelope fields/IDs, retry metadata, bounded diagnostic body, `Unwrap`. |
| `ResponseValidationError` | Malformed or contract-invalid status-200 body: status, canonical wire `Path`, stable `Reason`, IDs, bounded diagnostic body, `Unwrap`. |

`RawResponseBody` and successful `ResponseMetadata.Body` may contain echoed state or provider options. Never log or persist them without sanitization. Prefer the safe structured accessors.

## Support matrix

| Surface | Status |
| --- | --- |
| AI SDK Gateway provider protocol, `POST /v4/ai/evaluation-model` | Supported |
| Evaluation Model V4 boolean, choice, and score questions | Supported |
| Arbitrary nonempty model IDs, including `typesafe-ai/jev-latest` | Supported; no closed model catalog |
| JSON-compatible shared state and provider options | Supported |
| API-key and OIDC bearer authentication | Supported |
| Team scope and non-protected caller headers | Supported |
| Context cancellation and explicitly configured bounded retries | Supported |
| Public REST `POST /v1/evaluate` | Not supported |
| OpenAI-compatible `/v1/chat/completions` or `/v1/responses` | Not supported |
| Language/text generation or streaming | Not supported |
| Embeddings, images, video, reranking, speech, transcription, realtime, batches | Not supported |
| Credits, spend, generation lookup, model discovery | Not supported |
| Agents, `generateText` orchestration, automatic tools, UI helpers, schema framework, global provider registry | Not supported |
| Gateway-executed search helpers | Not supported |
| Gateway-native xAI `x_search` | Unsupported and unconfirmed; see [the decision record](docs/x-search.md) |
| Direct xAI `x_search` client | Not provided; direct xAI facts do not establish Gateway support |
| Full parity with JavaScript `ai` or `@ai-sdk/gateway` | Not claimed |

## Live evidence

The gated contract test and sanitized record are in [`docs/evaluation-live-evidence.md`](docs/evaluation-live-evidence.md). It remains **NOT RUN / PENDING LIVE RUN** because no authorized paid credentialed execution has occurred. Do not claim a live pass until that record is completed from the exact authorized command.
