# Vercel AI Go SDK

Experimental Go client for two distinct Vercel AI Gateway contracts:

- public generation through `POST /v1/responses` and `POST /v1/chat/completions`, with buffered and streaming calls; and
- the AI SDK Evaluation Model V4 provider protocol at `POST /v4/ai/evaluation-model`.

The public generation surface is not the separate public `POST /v1/evaluate` API. The provider evaluation surface is not an OpenAI-compatible generation endpoint. Search and all other categories remain unsupported unless listed below.

The module requires Go 1.26. The maintained families are Go 1.26 and 1.27; CI is pinned to Go 1.26.8 and Go 1.27.1. The experimental contract baseline is `ai@7.0.107`, `@ai-sdk/gateway@4.0.87`, `@ai-sdk/provider@4.0.17`, and `@ai-sdk/provider-utils@5.0.45`.

## Install

```sh
go get github.com/EveGoodEvening/vercel-ai-go-sdk
```

No public release exists. Release is blocked until the owner selects and commits a license, renames the hosted repository and updates `origin` to match the module path, completes the authorized live contracts, and reviews publication provenance. See [release readiness](docs/releasing.md).

## Public generation

```go
client, err := gateway.NewClient() // resolves credentials from the environment
if err != nil {
    log.Fatal(err)
}

result, err := client.CreateResponse(ctx, gateway.ResponsesRequest{
    Model: "openai/gpt-5-nano",
    Input: gateway.ResponseTextInput("Explain Go contexts briefly."),
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(result.RawJSON()))
```

The package also exposes `StreamResponse`, `CreateChatCompletion`, and `StreamChatCompletion`. See the [complete generation guide](docs/generation.md) and [generation example](examples/generate/main.go).

## Provider evaluation

```go
result, err := client.Evaluate(ctx, "typesafe-ai/jev-latest", gateway.EvaluationRequest{
    State: map[string]any{"answer": "Paris"},
    Questions: map[string]gateway.Question{
        "correct": gateway.BooleanQuestion{Instructions: "Is the answer factually correct?"},
    },
})
```

See the [evaluation guide](docs/evaluation.md) and [evaluation example](examples/evaluate/main.go). This evaluates shared state against keyed questions; it does not generate text and is unrelated to public `/v1/evaluate`.

## Authentication and client options

Credential precedence is fixed: explicit `WithAPIKey`, nonblank `AI_GATEWAY_API_KEY`, the last explicit `WithOIDCToken` or `WithOIDCTokenSource`, then nonblank `VERCEL_OIDC_TOKEN`. Otherwise `NewClient` returns `*ConfigurationError`. A selected credential is never replaced after rejection. A token source is called once per HTTP attempt with the operation context.

`WithBaseURL` changes only the provider-protocol base (default `https://ai-gateway.vercel.sh/v4/ai`). `WithPublicBaseURL` changes only the public generation base (default `https://ai-gateway.vercel.sh/v1`). Both require whitespace-exact absolute HTTP(S) URLs with host and no userinfo, query, or fragment; existing paths are retained and trailing path slashes removed. This split prevents a test or proxy URL for one contract from silently rerouting the other.

`WithHTTPClient` retains the supplied non-nil pointer. `WithTeam` sets the optional team header. `WithHeaders` clones caller headers and rejects protocol-owned names case-insensitively. `WithRetryPolicy` configures status-based retries for buffered calls; retries are disabled by default. Options apply in order and construction stops at the first error.

## Retries, cancellation, errors, and privacy

`RetryPolicy.MaxAttempts` includes the initial attempt: 0 and 1 mean one attempt; 2–10 opt in. Only 408, 409, 429, and 500–599 are retryable. Valid `Retry-After` is honored up to `MaxDelay`; waits are context-cancellable. Buffered Responses, Chat, and Evaluation calls use this policy. Streaming calls do not retry after response headers are received. Opting in can duplicate billable generation or evaluation work.

All operations reject a nil context locally. Cancellation covers credential resolution, request execution, buffered body reads, retry waits, and stream reads. Call `Close` on streams, normally with `defer`, to release the response body and unblock a concurrent `Next`.

Errors support `errors.As`: `ConfigurationError`, `ValidationError`, `TransportError`, `ResponseError`, and `ResponseValidationError`. Raw successful generation JSON, stream event JSON, `RawResponseBody`, evaluation `ResponseMetadata.Body`, prompts, uploaded file data, tool arguments/results, provider options, headers, and identifiers may be sensitive. Do not log or persist them without deliberate sanitization.

## Support matrix

| Surface | Status |
| --- | --- |
| Public Gateway Responses, buffered and SSE streaming | Implemented; hermetic fixtures pass; live execution **NOT RUN** |
| Public Gateway Chat Completions, buffered and SSE streaming | Implemented; hermetic fixtures pass; live execution **NOT RUN** |
| AI SDK Gateway Evaluation Model V4 provider protocol | Implemented; hermetic fixtures pass; live execution **NOT RUN** |
| Responses text/item input, function tools, tool choice, reasoning, text formats, metadata and cache controls | Supported as documented in the generation guide |
| Chat text/image-URL/file parts, function tools, response formats and Gateway routing options | Supported as documented in the generation guide |
| API-key and OIDC bearer authentication, team scope, safe custom headers | Supported |
| Context cancellation and explicitly configured bounded retries | Supported; streams are not resumed or replayed |
| Public REST `POST /v1/evaluate` | Not supported; no exported API or live evidence |
| Credits, spend, generation lookup, or model discovery APIs | Not supported |
| Embeddings, image/video generation, reranking, speech, transcription, realtime, or batches | Not supported |
| Agents, orchestration, automatic tool execution, UI helpers, schema framework, or global provider registry | Not supported |
| Gateway search helpers of any category | Not supported; no exported request types or live evidence |
| Gateway-native xAI `x_search` | Unsupported and unconfirmed; see [the decision record](docs/x-search.md) |
| Direct provider clients, including direct xAI | Not provided |
| Full parity with JavaScript `ai`, `@ai-sdk/gateway`, or OpenAI APIs | Not claimed |

## Evidence status and blockers

The ordinary suite is hermetic: it uses loopback fixtures and rejects non-loopback traffic. That proves local encoding, validation, response parsing, streaming, resource limits, cancellation, and retry behavior; it does not prove the hosted Gateway currently accepts the requests.

The sole sanitized live record is [`docs/evaluation-live-evidence.md`](docs/evaluation-live-evidence.md). Both public generation and provider evaluation remain **NOT RUN / PENDING LIVE RUN** because no owner-authorized paid-network execution, protected-environment run, or sanitized live result exists. Public generation requires its exact acknowledgement and isolated Responses/Chat command; provider evaluation requires its separate acknowledgement and isolated evaluation command. Neither contract is evidence for the other. Search and public `/v1/evaluate` have no authorized probe, exported API, or first-party wire evidence in this repository and remain unsupported rather than pending implementation.

## Migration and release policy

Generation is additive: existing evaluation callers continue using `Evaluate` and `WithBaseURL` unchanged. New generation callers choose Responses or Chat and may set `WithPublicBaseURL`; it does not affect evaluation. No compatibility alias or endpoint auto-detection is provided.

Every exported breaking change, including during v0, requires a version increment, changelog entry, and release-note **Migration** section naming every changed API and caller action. Published tags are immutable. See the complete [release policy](docs/releasing.md).
