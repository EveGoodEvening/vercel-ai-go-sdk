# Vercel AI Go SDK

Experimental Go client for Vercel AI Gateway text generation and evaluation. Requires **Go 1.26+**; the package name is `gateway`.

| API | Methods | Result support |
| --- | --- | --- |
| Public `/v1/responses` | `CreateResponse`, `StreamResponse`; opt-in `CreateResponseWithBuiltInTools`, `StreamResponseWithBuiltInTools` | Raw buffered JSON; typed text deltas and raw streaming events |
| Public `/v1/chat/completions` | `CreateChatCompletion`, `StreamChatCompletion` | Typed buffered fields; streamed text deltas; raw JSON access |
| Provider `/v4/ai/evaluation-model` | `Evaluate` | Validated boolean, choice, and score answers |

**Unreleased:** hermetic fixtures cover these APIs. The full credentialed hosted contract suites have **not run**; the only hosted generation evidence is the narrow sanitized structural corroboration for the two exact Responses search declarations, which does not establish general generation compatibility. See [search evidence](docs/x-search.md#sanitized-structural-evidence), [live evidence](docs/evaluation-live-evidence.md), and [release readiness](docs/releasing.md). This is not a full port of the JavaScript AI SDK or a general OpenAI client.

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
- [Client configuration](docs/client.md): credentials, endpoints, retries, errors, cancellation, and privacy.
- Runnable examples: [generation](examples/generate/main.go) and [evaluation](examples/evaluate/main.go).
- Maintainers: [contributing](CONTRIBUTING.md), [changelog](CHANGELOG.md), and [release policy](docs/releasing.md).

## Important boundaries

- **Separate endpoints:** `WithPublicBaseURL` configures generation; `WithBaseURL` configures provider evaluation. Neither redirects the other. `Evaluate` does **not** call public `/v1/evaluate`.
- **Retries are off by default.** Buffered calls can opt in; retries may duplicate billable work. Streams are not resumed or replayed. Always close a stream and check its final `Err()`.
- **Search declarations are request-only and server-executed.** Responses opt-in built-in-tools methods accept fixed low-context `ResponseWebSearchTool{}` and fieldless `ResponseXSearchTool{}`; the exact evidenced routes are `openai/gpt-5.4-mini` and `spacexai/grok-4.6`. Chat separately accepts four `vercel:...` server tools through its opt-in methods. The SDK executes none of these tools automatically. Search calls/results/actions/posts/sources/citations/annotations/refusals/provider errors/usage/cost and search lifecycle events have no typed contract. Responses preserves buffered payloads through `RawJSON()` and streams typed text deltas while retaining unsupported events as `RawResponseEvent`. Chat returns Chat completion chunks, each with its own `RawJSON()` preservation, and never uses `RawResponseEvent`. See [search support](docs/x-search.md).

Not implemented:

- Public `/v1/evaluate`; configurable `x_search` options; `web_search_preview`, non-low or omitted web-search context, other current/preview forms, and undocumented search options; wider model compatibility, search-specific tool choice/`allowed_tools`, and Gateway fallback behavior.
- The internal `/v4/ai/language-model` protocol and direct-provider clients are outside this SDK.
- Embeddings, image/video generation, reranking, speech, transcription, realtime, batches, and management APIs (credits, spend, generation lookup, model discovery).
- Agents, orchestration, automatic function-tool execution, UI helpers, schema frameworks, and a global provider registry.
