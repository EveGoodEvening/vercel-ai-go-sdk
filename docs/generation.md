# Public generation guide

The public generation APIs are experimental. They use the public `/v1` base URL and are separate from the provider-protocol [`Evaluate`](evaluation.md) endpoint. The full authorized paid generation contract suite remains **NOT RUN / PENDING LIVE RUN**; only the narrow sanitized Responses search-request corroboration recorded in [`x-search.md`](x-search.md#sanitized-structural-evidence) has run, and it does not establish general buffered/streaming generation compatibility.

Examples below are independent snippets inside a function returning `error`, with a configured `client` and live `ctx`. See [client configuration](client.md) for shared authentication, HTTP, retry, error, and privacy behavior.

## Choose a surface

| Need | Use |
| --- | --- |
| New, feature-rich generation; item-based input; reasoning, caching, or Responses tool controls | Responses: `CreateResponse` / `StreamResponse` |
| OpenAI Chat Completions compatibility; multimodal message parts; Gateway routing; request-only server search | Chat: `CreateChatCompletion` / `StreamChatCompletion` |

The request and result types are intentionally separate. There is no generic `Generate` method or automatic conversion between them. Both use the public base URL (`/responses` or `/chat/completions`); `WithBaseURL` configures only provider evaluation, while `WithPublicBaseURL` configures generation.

## Responses

### Buffered request

```go
request := gateway.ResponsesRequest{
    Model: "openai/gpt-5-nano",
    Input: gateway.ResponseTextInput("Explain Go contexts briefly."),
}

result, err := client.CreateResponse(ctx, request)
if err != nil {
    return err
}
fmt.Printf("response_bytes=%d\n", len(result.RawJSON()))
```

`CreateResponse` sends `stream:false`. `ResponseResult` deliberately exposes only `RawJSON()`, which returns a defensive copy of the complete bounded status-200 JSON object. The SDK does not guess a stable typed Responses result schema; do not log the raw value.

Supported request areas are:

- required `provider/model` model ID and either text or typed item input;
- sampling/token controls and instructions;
- function tools, named/mode tool choice, parallel tool calls, and allowed tools;
- reasoning effort/summary and text, JSON-object, or JSON-schema output formats;
- truncation, previous-response linkage, storage, metadata, and Gateway cache controls.

For exhaustive fields and variants, see [Responses types](../responses.go). Function tools are serialized but never executed by the SDK. The opt-in built-in-tools methods additionally support only fixed low-context `web_search` and fieldless `x_search` request declarations; see [Responses server search](#request-only-responses-server-search) and [search support](x-search.md).

### Streaming

```go
request := gateway.ResponsesRequest{
    Model: "openai/gpt-5-nano",
    Input: gateway.ResponseTextInput("Explain Go contexts briefly."),
}

stream, err := client.StreamResponse(ctx, request)
if err != nil {
    return err
}
defer stream.Close()

textDeltas, otherEvents := 0, 0
for stream.Next() {
    switch stream.Event().(type) {
    case gateway.ResponseOutputTextDeltaEvent:
        textDeltas++
    case gateway.RawResponseEvent:
        otherEvents++
    }
}
if err := stream.Err(); err != nil {
    return err
}
fmt.Printf("text_delta_events=%d other_events=%d\n", textDeltas, otherEvents)
```

`StreamResponse` sends `stream:true`. Only `response.output_text.delta` is typed as `ResponseOutputTextDeltaEvent`; every other valid event object is a `RawResponseEvent` with a defensive `RawJSON()` accessor. The stream retains no transcript and starts no producer goroutine.

A clean, completely framed HTTP EOF is successful for Responses; no application terminal event is required. Malformed/truncated framing, invalid event JSON, a missing type discriminator, cancellation, limits, or body read/close failure makes `Err()` non-nil.

## Request-only Responses server search

Responses search is an opt-in **request serialization** surface. Use `ResponsesBuiltInToolsRequest` with `CreateResponseWithBuiltInTools` or `StreamResponseWithBuiltInTools`; the existing `ResponsesRequest`, `CreateResponse`, and `StreamResponse` contracts are unchanged. Gateway/provider infrastructure executes the declared search tools server-side. The Go SDK neither executes tools nor turns search calls, results, sources, or lifecycle events into typed values.

The only supported declarations are exact and fieldless:

- `ResponseWebSearchTool{}` emits `{"type":"web_search","search_context_size":"low"}`. Omitting `search_context_size`, choosing another size, and using `web_search_preview` or another current/preview form are unsupported.
- `ResponseXSearchTool{}` emits exactly `{"type":"x_search"}`. It has no configurable fields.

Ordinary function tools remain in `ResponsesRequest.Tools` and may coexist with built-in tools. The encoder emits function tools first, then built-in tools in caller order.

### Buffered search request

```go
request := gateway.ResponsesBuiltInToolsRequest{
    Request: gateway.ResponsesRequest{
        Model: "openai/gpt-5.4-mini",
        Input: gateway.ResponseTextInput("Find current public information."),
        Tools: []gateway.ResponseTool{
            {Name: "lookup_local", Parameters: map[string]any{"type": "object"}},
        },
    },
    Tools: []gateway.ResponseBuiltInTool{
        gateway.ResponseWebSearchTool{},
    },
}

result, err := client.CreateResponseWithBuiltInTools(ctx, request)
if err != nil {
    return err
}
fmt.Printf("response_bytes=%d\n", len(result.RawJSON()))
```

For fieldless X search, use the positively probed route and replace the built-in tool:

```go
request.Request.Model = "spacexai/grok-4.6"
request.Tools = []gateway.ResponseBuiltInTool{gateway.ResponseXSearchTool{}}
```

`ResponseResult.RawJSON()` remains the complete buffered search-output boundary. Inspect only structural properties appropriate for your application; the bytes are not sanitized and may contain sensitive input, generated prose, tool data, sources, identifiers, usage, costs, or provider extensions.

### Streaming search request

```go
request := gateway.ResponsesBuiltInToolsRequest{
    Request: gateway.ResponsesRequest{
        Model: "spacexai/grok-4.6",
        Input: gateway.ResponseTextInput("Find current public information."),
        Tools: []gateway.ResponseTool{
            {Name: "lookup_local", Parameters: map[string]any{"type": "object"}},
        },
    },
    Tools: []gateway.ResponseBuiltInTool{
        gateway.ResponseXSearchTool{},
    },
}

stream, err := client.StreamResponseWithBuiltInTools(ctx, request)
if err != nil {
    return err
}
defer stream.Close()

textDeltas, rawEvents := 0, 0
for stream.Next() {
    switch stream.Event().(type) {
    case gateway.ResponseOutputTextDeltaEvent:
        textDeltas++
    case gateway.RawResponseEvent:
        rawEvents++
    }
}
if err := stream.Err(); err != nil {
    return err
}
fmt.Printf("text_delta_events=%d raw_events=%d\n", textDeltas, rawEvents)
```

The same streaming method accepts `ResponseWebSearchTool{}` with `openai/gpt-5.4-mini`. Only text deltas are typed; search-call and all other valid event objects fall back to `RawResponseEvent`. No search-specific event names, ordering, status, completion, error, citation, or terminal semantics are promised.

Model compatibility is evidence-bounded:

| Declaration | Exact evidenced route | Boundary |
| --- | --- | --- |
| fixed low-context `web_search` | `openai/gpt-5.4-mini` | Documented by Vercel and structurally corroborated through public Gateway Responses |
| fieldless `x_search` | `spacexai/grok-4.6` | Positively probed through public Gateway Responses |

The Gateway catalog is dynamic. These rows do not promise universal OpenAI, SpaceXAI, or cross-provider support; the SDK has no model allowlist, automatic fallback, or routing compatibility guarantee, so unsupported model/tool combinations may fail server-side. Configurable `x_search` options, search-specific tool choice and `allowed_tools`, and all typed search calls/results/actions/posts/sources/citations/annotations/refusals/provider errors/usage/cost remain blocked. This surface is not a direct-xAI client and imports no direct-xAI option or output contract.

## Chat Completions

### Buffered request

```go
request := gateway.ChatCompletionRequest{
    Model: "openai/gpt-5-nano",
    Messages: []gateway.ChatMessage{
        {Role: "user", Content: gateway.ChatTextContent("Explain Go contexts briefly.")},
    },
}

result, err := client.CreateChatCompletion(ctx, request)
if err != nil {
    return err
}
choicesPresent := result.Choices.Present && !result.Choices.Null
fmt.Printf("choices_present=%t\n", choicesPresent)
```

`CreateChatCompletion` sends `stream:false`. Chat has typed, presence-preserving fields plus `RawJSON()` for the complete bounded object. Unknown response members remain only in raw JSON.

`JSONField[T]` distinguishes all wire states:

| JSON state | `Present` | `Null` | `Value` |
| --- | ---: | ---: | --- |
| member absent | `false` | `false` | zero value |
| member present as `null` | `true` | `true` | zero value |
| member present with a value | `true` | `false` | decoded value |

Always test `Present` and `Null` before using `Value`; for example, assistant `content:null` is different from absent content and from `content:""`.

Supported request areas are:

- required model and typed `system`, `developer`, `user`, or `assistant` messages;
- text content or text/image-URL/inline-file parts;
- sampling, token, stop, penalty, and safety controls;
- ordinary function tools and tool choice;
- text, JSON, JSON-schema, and legacy JSON response formats;
- fallback models and Gateway provider routing/timeouts.

See [Chat request, message, tool, format, and result types](../chat.go) for the complete field inventory.

### Streaming

```go
request := gateway.ChatCompletionRequest{
    Model: "openai/gpt-5-nano",
    Messages: []gateway.ChatMessage{
        {Role: "user", Content: gateway.ChatTextContent("Explain Go contexts briefly.")},
    },
}

stream, err := client.StreamChatCompletion(ctx, request)
if err != nil {
    return err
}
defer stream.Close()

chunks, choices := 0, 0
for stream.Next() {
    chunks++
    choices += len(stream.Event().Choices)
}
if err := stream.Err(); err != nil {
    return err
}
fmt.Printf("chunks=%d choices=%d\n", chunks, choices)
```

Each event is a validated `chat.completion.chunk`. Typed data is limited to ordered choices and their text delta; `RawJSON()` retains the rest. Chat requires the exact `data: [DONE]` marker for successful termination. EOF before `[DONE]` is an error.

Both stream types accept at most 10,000 events. An explicit `Close()` ends reading; it does not prove the server completed generation. See [stream ownership and cancellation](client.md#cancellation-and-streams).

## Request-only Chat server search

Gateway Chat server search is a separate opt-in **request serialization** extension. It does not type search results, lifecycle events, citations, offsets, costs, refusals, or provider errors. Server execution remains behind the ordinary typed/raw Chat result boundary. It does not imply support for Responses search forms beyond the exact fixed declarations documented above, public `/v1/evaluate`, or any direct-xAI client.

The four concrete wire declarations are:

- `ChatExaSearchTool{Config: ...}` → `type: "vercel:exa_search"`
- `ChatParallelSearchTool{Config: ...}` → `type: "vercel:parallel_search"`
- `ChatPerplexitySearchTool{Config: ...}` → `type: "vercel:perplexity_search"`
- `ChatTakoSearchTool{Config: ...}` → `type: "vercel:tako_search"`

Use the explicit wrapper and methods; the existing `ChatCompletionRequest`, `CreateChatCompletion`, and `StreamChatCompletion` signatures are unchanged:

```go
request := gateway.ChatServerToolsRequest{
    Request: gateway.ChatCompletionRequest{
        Model: "openai/gpt-5-nano",
        Messages: []gateway.ChatMessage{
            {Role: "user", Content: gateway.ChatTextContent("Summarize current Go news.")},
        },
        ToolChoice: gateway.ChatToolChoiceRequired,
    },
    ServerTools: []gateway.ChatServerTool{
        gateway.ChatExaSearchTool{Config: gateway.ChatExaSearchConfig{Query: "Go language news"}},
    },
}

result, err := client.CreateChatCompletionWithServerTools(ctx, request)
if err != nil {
    return err
}
fmt.Printf("choices_present=%t\n", result.Choices.Present && !result.Choices.Null)
```

For streaming, call `StreamChatCompletionWithServerTools(ctx, request)` and use the same `Next` / `Event` / `Err` / `Close` lifecycle shown above. Both methods still target `/v1/chat/completions`.

Ordinary function tools are emitted first, then server tools, preserving caller order and duplicate server declarations. See [config types and enums](../chat.go), [validation](../chat_validate.go), and [encoding](../chat_wire.go) for the full contract. Required queries/objectives must be nonempty; Tako `Strict: true` requires present, nonempty `NodeIDs`.

Presence rules are intentional:

- optional scalar pointers: nil omits the member; non-nil emits the value, including zero or `false`;
- optional slice pointers: nil pointer omits; pointer-to-nil and pointer-to-empty both emit `[]`, never `null`;
- optional union members: nil interface omits; supported variants emit their scalar, object, or array shape;
- `ChatExaContents.SubpageTarget`: `ChatExaSubpageTargetStrings(nil)` and a non-nil pointer to a nil slice emit `[]`; a typed-nil pointer is rejected before credentials or network work; array order is preserved;
- present nested config pointers emit an object even when all their optional members are omitted.

Ordinary functions may coexist with server tools, subject to name rules. If a server tool is present, an ordinary function with its short identifier (`exa_search`, `parallel_search`, `perplexity_search`, or `tako_search`) is a collision. `ChatSpecificToolChoice` may select neither that short identifier nor its `vercel:...` form. A different function name is allowed; `ChatToolChoiceAuto`, `ChatToolChoiceRequired`, and `ChatToolChoiceNone` remain available. When the matching server tool is absent, the short ordinary-function name and matching specific choice are valid.

## Limits and raw-data boundary

Requests are validated before credential or network work. Shared resource policy caps JSON nesting at 64, individual string/byte values and buffered bodies/events at 1 MiB, and arrays/objects/request collections/stream events at 10,000 where applicable. Responses metadata is capped at 16 entries. SSE lines are capped at 64 KiB.

Raw result and event accessors return defensive copies, not sanitized data. They may contain prompts, generated text, tool arguments/results, search configuration/output, identifiers, usage, routing details, or provider extensions. Prefer typed presence flags and structural counts in diagnostics; do not print or log raw JSON or generated content.

## Runnable example

With credentials already set, [examples/generate](../examples/generate/main.go) can run any of the four text paths:

```sh
go run ./examples/generate -surface responses -mode buffered -model openai/gpt-5-nano
```

Choose `responses` or `chat` and `buffered` or `stream`. Each invocation sends a real, potentially billable request and prints only structural summaries.
