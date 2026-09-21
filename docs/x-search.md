# Search support and native xAI `x_search`

Search support depends on the endpoint, exact request declaration, and model—not merely a model's generic tool capability.

| Surface | SDK support |
| --- | --- |
| Chat `vercel:exa_search`, `vercel:parallel_search`, `vercel:perplexity_search`, `vercel:tako_search` | Request declarations through the two opt-in server-tools methods; separate Chat contract and no typed search outputs or metadata |
| Responses fixed low-context `web_search` | `ResponseWebSearchTool{}` through the two opt-in built-in-tools methods |
| Responses fieldless Gateway `x_search` | `ResponseXSearchTool{}` through the two opt-in built-in-tools methods |
| Responses configurable Gateway `x_search` | `ResponseXSearchOptionsTool` through the same two opt-in methods; request fields only |
| Responses `web_search_preview` and other current/preview forms | Not supported |
| Direct xAI `x_search` | Not supported; this SDK has no direct-xAI client |

For code examples, see [request-only Responses server search](generation.md#request-only-responses-server-search) and [request-only Chat server search](generation.md#request-only-chat-server-search). Shared authentication, HTTP, retry, error, cancellation, and privacy behavior is documented in [client configuration](client.md).

## Exact Responses request boundary

Supported Responses declarations use `ResponsesBuiltInToolsRequest` with `CreateResponseWithBuiltInTools` or `StreamResponseWithBuiltInTools`. They are additive opt-in methods: existing `ResponsesRequest`, `CreateResponse`, and `StreamResponse` calls are unchanged. Gateway/provider infrastructure executes the tools server-side; the SDK performs no automatic client tool execution.

| Go declaration | Encoded tool object | Exact evidenced model route |
| --- | --- | --- |
| `ResponseWebSearchTool{}` | `{"type":"web_search","search_context_size":"low"}` | `openai/gpt-5.4-mini` |
| `ResponseXSearchTool{}` | `{"type":"x_search"}` | `spacexai/grok-4.6` |
| `ResponseXSearchOptionsTool{...}` | `{"type":"x_search",...}` with only present supported option fields | `spacexai/grok-4.6` |

`ResponseXSearchTool` remains fieldless. Use `ResponseXSearchOptionsTool` only when options are required; it has exactly these six fields:

| Go field | Wire field | Presence behavior |
| --- | --- | --- |
| `AllowedXHandles []string` | `allowed_x_handles` | A nil slice omits the member; a non-nil empty slice emits `[]` |
| `ExcludedXHandles []string` | `excluded_x_handles` | A nil slice omits the member; a non-nil empty slice emits `[]` |
| `FromDate *string` | `from_date` | Nil omits the member; any non-nil string, including `""`, is forwarded subject to generic string-size bounds |
| `ToDate *string` | `to_date` | Nil omits the member; any non-nil string, including `""`, is forwarded subject to generic string-size bounds |
| `EnableImageUnderstanding *bool` | `enable_image_understanding` | Nil omits the member; pointers encode explicit `false` or `true` |
| `EnableVideoUnderstanding *bool` | `enable_video_understanding` | Nil omits the member; pointers encode explicit `false` or `true` |

No option is encoded as `null`. Caller order and duplicate handles are preserved. Before credentials are resolved or a network request is sent, the SDK rejects only the option-specific case where both handle lists are non-empty. Generic request, collection, and string-size bounds still apply.

The SDK does **not** impose a provider-specific 10- or 20-handle limit, validate handle syntax, remove duplicates, validate date grammar or ordering, define date inclusivity or empty-date meaning, or claim that an accepted option changes search results. It also does not define server behavior for wrong JSON kinds. These omissions are deliberate request-boundary limits, not statements about a provider's contract.

The model catalog is dynamic and the implementation intentionally has no closed model enum. The exact routes above do not establish universal OpenAI, SpaceXAI, or Gateway-model compatibility. There is no automatic Gateway fallback or cross-provider portability promise; an unsupported model/tool combination may fail server-side.

Ordinary Responses function tools may coexist with these declarations. Function tools are encoded first, followed by built-in tools in caller order. This coexistence does not make the SDK execute any function or built-in tool.

## Result and event boundary

Support is request-only. Buffered responses remain `ResponseResult.RawJSON()`; streaming still types only `response.output_text.delta` as `ResponseOutputTextDeltaEvent` and preserves every other valid event object as `RawResponseEvent`. Raw bytes are defensive copies, not sanitized content.

The structural evidence proves server-side acceptance of the documented request declarations only. It does **not** define typed search calls, results, actions, posts, source lists, citations, annotations or offsets, refusals, provider errors, usage, cost, unknown variants, retention behavior, or buffered field requiredness/nullability. It also does not define search lifecycle event names, payloads, ordering, deltas, completion/failure effects, or terminal semantics.

## Sanitized structural evidence

Only non-sensitive structural facts are retained:

| Tool | Endpoint | Model | Exercised declaration | HTTP/status structure | Observed output discriminator/status set |
| --- | --- | --- | --- | --- | --- |
| `web_search` | public Gateway Responses | `openai/gpt-5.4-mini` | fixed low-context declaration | HTTP 200; top-level response object; incomplete response under a low output-token cap | `web_search_call`, `message` |
| `x_search` | public Gateway Responses | `spacexai/grok-4.6` | fieldless and supported configurable request declarations | HTTP 200; top-level response object; completed response | completed `x_search_call`; reasoning items; completed message |

This public evidence excludes credentials, authorization data, input text, generated prose, bodies, headers, identifiers, provider metadata, usage, cost, and raw payloads. It establishes request acceptance and structural output only, not option semantics or efficacy.

## Remaining blocked surfaces

- For `web_search`, omitting `search_context_size`, any value other than exact `"low"`, `web_search_preview`, external-web-access flags, filters/domains, approximate location, and other current/preview forms are blocked.
- For `x_search`, semantics beyond the six supported request fields remain blocked: limits/defaults beyond generic SDK bounds, handle/date interpretation, semantic efficacy, wrong-kind server behavior, and fields not listed above.
- Search-specific tool choice, `allowed_tools`, function/built-in precedence semantics beyond deterministic request ordering, Gateway fallback, routing behavior, and wider model/provider compatibility are blocked.
- Typed buffered search output and typed search lifecycle events remain blocked as detailed above. Raw buffered and streaming fallback is the supported observation boundary.

## Direct xAI and Chat remain distinct

Direct xAI documents `POST https://api.x.ai/v1/responses`, configurable `x_search` options, and provider-specific output forms. The direct `@ai-sdk/xai` helper likewise describes a direct-provider contract. This SDK's six Gateway request fields do not import direct-xAI limits, defaults, validation, semantics, output forms, authentication, base URLs, or compatibility promises, and this SDK does not provide a direct-xAI client.

Chat's four `vercel:...` server-search declarations are a separate `/v1/chat/completions` request contract with their own configuration types. They neither alias the Responses declarations nor clear any Responses output/event blocker.

## Sources

- [Vercel Gateway web search](https://vercel.com/docs/ai-gateway/models-and-providers/web-search), inspected 2026-09-21; source for the exact fixed `web_search` request and documented model route.
- [Vercel Gateway Responses tool calling](https://vercel.com/docs/ai-gateway/sdks-and-apis/responses/tool-calling), inspected 2026-09-21; endpoint/tool context only.
- [xAI X Search wire contract](https://docs.x.ai/developers/tools/x-search), direct-xAI comparison only.
- [xAI provider-executed output types](https://docs.x.ai/developers/tools/tool-usage-details), direct-xAI comparison only.
- [Direct xAI helper declarations, 5.0.4](https://unpkg.com/@ai-sdk/xai@5.0.4/dist/index.d.ts), direct-provider comparison only.
- [Gateway tool registry at ai@7.0.107](https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-tools.ts), historical package comparison only.
