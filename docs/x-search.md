# Search support and native xAI `x_search`

Search support depends on the endpoint, exact request declaration, and model—not merely a model's generic tool capability.

| Surface | SDK support |
| --- | --- |
| Chat `vercel:exa_search`, `vercel:parallel_search`, `vercel:perplexity_search`, `vercel:tako_search` | Request declarations through the two opt-in server-tools methods; separate Chat contract and no typed search outputs or metadata |
| Responses fixed low-context `web_search` | `ResponseWebSearchTool{}` through the two opt-in built-in-tools methods |
| Responses fieldless Gateway `x_search` | `ResponseXSearchTool{}` through the two opt-in built-in-tools methods |
| Responses `web_search_preview`, other current/preview forms, and configurable search options | Not supported |
| Direct xAI `x_search` | Not supported; this SDK has no direct-xAI client |

For code examples, see [request-only Responses server search](generation.md#request-only-responses-server-search) and [request-only Chat server search](generation.md#request-only-chat-server-search).

## Exact Responses request boundary

Both supported Responses declarations use `ResponsesBuiltInToolsRequest` with `CreateResponseWithBuiltInTools` or `StreamResponseWithBuiltInTools`. They are additive opt-in methods: existing `ResponsesRequest`, `CreateResponse`, and `StreamResponse` calls are unchanged. Gateway/provider infrastructure executes the tools server-side; the SDK performs no automatic client tool execution.

| Go declaration | Encoded tool object | Exact evidenced model route |
| --- | --- | --- |
| `ResponseWebSearchTool{}` | `{"type":"web_search","search_context_size":"low"}` | `openai/gpt-5.4-mini` |
| `ResponseXSearchTool{}` | `{"type":"x_search"}` | `spacexai/grok-4.6` |

The model catalog is dynamic and the implementation intentionally has no closed model enum. These exact routes do not establish universal OpenAI, SpaceXAI, or Gateway-model compatibility. There is no automatic Gateway fallback or cross-provider portability promise; an unsupported model/tool combination may fail server-side.

Ordinary Responses function tools may coexist with either declaration. Function tools are encoded first, followed by built-in tools in caller order. This coexistence does not make the SDK execute any function or built-in tool.

## Result and event boundary

Support is request-only. Buffered responses remain `ResponseResult.RawJSON()`; streaming still types only `response.output_text.delta` as `ResponseOutputTextDeltaEvent` and preserves every other valid event object as `RawResponseEvent`. Raw bytes are defensive copies, not sanitized content.

The structural evidence below proves server-side execution of the exact declarations only. It does **not** define typed search calls, results, actions, posts, source lists, citations, annotations or offsets, refusals, provider errors, usage, cost, unknown variants, retention behavior, or buffered field requiredness/nullability. It also does not define search lifecycle event names, payloads, ordering, deltas, completion/failure effects, or terminal semantics.

## Sanitized structural evidence

Evidence was reviewed and exercised on 2026-09-21. Only structural facts are retained here:

| Tool | Endpoint | Model | Exercised tool/options | HTTP/status structure | Observed output discriminator/status set |
| --- | --- | --- | --- | --- | --- |
| `web_search` | `POST https://ai-gateway.vercel.sh/v1/responses` | `openai/gpt-5.4-mini` | `{"type":"web_search","search_context_size":"low"}` only | HTTP 200; top-level `object:"response"`; response status `incomplete` under a low output-token cap | `web_search_call`, `message` |
| `x_search` | `POST https://ai-gateway.vercel.sh/v1/responses` | `spacexai/grok-4.6` | `{"type":"x_search"}` plus general `tool_choice:"required"`; no `x_search` option fields | HTTP 200; top-level `object:"response"`; response status `completed` | completed `x_search_call`; reasoning items; completed message |

This durable public evidence is limited to the structural fields in the table; credentials, authorization data, input text, generated prose, bodies, headers, identifiers, and raw payloads are excluded. Future reruns require explicit owner authorization and must append a dated structural record rather than replace this history.

## Remaining blocked surfaces

- Configurable `x_search` fields are blocked, including direct-xAI candidate names such as handle filters, date ranges, and image/video understanding flags. Direct-xAI limits, defaults, mutual-exclusion rules, validation, and presence/null behavior are not Gateway evidence.
- For `web_search`, omitting `search_context_size`, any value other than exact `"low"`, `web_search_preview`, external-web-access flags, filters/domains, approximate location, and other current/preview forms are blocked.
- Search-specific tool choice, `allowed_tools`, function/built-in precedence semantics beyond deterministic request ordering, Gateway fallback, routing behavior, and wider model/provider compatibility are blocked.
- Typed buffered search output and typed search lifecycle events remain blocked as detailed above. Raw buffered and streaming fallback is the supported observation boundary.

## Direct xAI and Chat remain distinct

Direct xAI documents `POST https://api.x.ai/v1/responses`, configurable `x_search` options, and provider-specific output forms. The direct `@ai-sdk/xai` helper likewise describes a direct-provider contract. Neither is imported into this SDK's Gateway validation or compatibility promises, and this SDK does not provide direct-xAI authentication, base URLs, or a direct client.

Chat's four `vercel:...` server-search declarations are a separate `/v1/chat/completions` request contract with their own configuration types. They neither alias the Responses declarations nor clear any Responses output/event blocker.

## Sources

- [Vercel Gateway web search](https://vercel.com/docs/ai-gateway/models-and-providers/web-search), inspected 2026-09-21; source for the exact fixed `web_search` request and documented model route.
- [Vercel Gateway Responses tool calling](https://vercel.com/docs/ai-gateway/sdks-and-apis/responses/tool-calling), inspected 2026-09-21; endpoint/tool context only.
- [xAI X Search wire contract](https://docs.x.ai/developers/tools/x-search), direct-xAI comparison only.
- [xAI provider-executed output types](https://docs.x.ai/developers/tools/tool-usage-details), direct-xAI comparison only.
- [Direct xAI helper declarations, 5.0.4](https://unpkg.com/@ai-sdk/xai@5.0.4/dist/index.d.ts), direct-provider comparison only.
- [Gateway tool registry at ai@7.0.107](https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-tools.ts), historical package comparison only.
