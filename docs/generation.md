# Public generation guide

The public generation APIs are experimental. They use the public `/v1` base URL and are separate from the provider-protocol [`Evaluate`](evaluation.md) endpoint. Authorized paid generation evidence is still **NOT RUN / PENDING LIVE RUN**; see [`evaluation-live-evidence.md`](evaluation-live-evidence.md).

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

For exhaustive fields and variants, see [Responses types](../responses.go). Function tools are serialized but never executed by the SDK. Responses built-in `web_search` and `web_search_preview` are not exported; see [search support](x-search.md).

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

Gateway server search is an opt-in **request serialization** extension. It does not type search results, lifecycle events, citations, offsets, costs, refusals, or provider errors. Server execution remains behind the ordinary typed/raw Chat result boundary. It does not imply Responses built-in search, public `/v1/evaluate`, or Gateway-native xAI `x_search` support.

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
