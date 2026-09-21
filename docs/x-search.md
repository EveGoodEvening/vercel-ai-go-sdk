# Search support and native xAI `x_search`

Search support depends on the endpoint and tool contract—not just the model's generic tool capability.

| Surface | SDK support |
| --- | --- |
| Chat `vercel:exa_search`, `vercel:parallel_search`, `vercel:perplexity_search`, `vercel:tako_search` | Request declarations through the two opt-in server-tools methods; no typed search outputs or metadata |
| Responses built-in `web_search` / `web_search_preview` | Not implemented; Gateway wire evidence is insufficient |
| Gateway-native xAI `x_search` | Not implemented or confirmed |
| Direct xAI `x_search` | Documented by xAI, but this SDK provides no direct-xAI client |

For Chat usage and configuration rules, see [generation](generation.md#request-only-chat-server-search). Search execution/results stay behind the existing raw output boundary; inline citations are model text, not a typed citation contract. Hosted corroboration has not run.

## Why native `x_search` is separate

The reviewed direct-xAI contract uses `POST https://api.x.ai/v1/responses` and a top-level tool with `type: "x_search"`. Its snake-case options include `allowed_x_handles`, `excluded_x_handles`, `from_date`, `to_date`, `enable_image_understanding`, and `enable_video_understanding`; provider-executed output can include `x_search_call`.

The direct-provider `@ai-sdk/xai@5.0.4` helper exposes corresponding camelCase options. Neither that helper nor the direct HTTP contract proves that Vercel AI Gateway accepts or preserves the same wire format.

At the repository's [pinned evidence baseline](evaluation-live-evidence.md#pinned-contracts), `@ai-sdk/gateway@4.0.87` has Exa, Parallel, Perplexity, and Tako helpers, but no `xSearch` helper. The reviewed Gateway docs and model metadata do not establish a native `x_search` request/result contract. This SDK therefore exports no native `x_search` API and contains no Gateway `x_search` live probe.

## Evidence required to add support

Native Gateway `x_search` requires both:

1. First-party Gateway evidence for the exact endpoint, request fields/limits, suitable model ID, and native response/event shape.
2. An authorized Gateway contract result demonstrating native `x_search_call` evidence—not merely generated prose.

A probe can be planned only after the schema evidence exists, with explicit credential inputs, cost acknowledgement, sanitization, and success criteria. Responses built-in search has its own request and output evidence gates; Chat request support clears neither. See the [current evidence decisions](../planning/IMPLEMENTATION_PLAN.md#2026-09-21-evidence-continuation--authoritative-current-disposition).

These unsupported surfaces are not prerequisites for releasing the implemented SDK. Do not add placeholder tests or infer support from OpenAI documentation, direct-provider SDKs, or generic model capabilities.

## Sources

Evidence reviewed 2026-09-20–21; these links record that baseline, not a promise about later upstream changes.

- [Gateway Chat web search](https://vercel.com/docs/ai-gateway/models-and-providers/web-search)
- [Gateway Responses tool calling](https://vercel.com/docs/ai-gateway/sdks-and-apis/responses/tool-calling)
- [xAI X Search wire contract](https://docs.x.ai/developers/tools/x-search)
- [xAI provider-executed output types](https://docs.x.ai/developers/tools/tool-usage-details)
- [Direct xAI helper declarations, 5.0.4](https://unpkg.com/@ai-sdk/xai@5.0.4/dist/index.d.ts)
- [Gateway tool registry at ai@7.0.107](https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-tools.ts)
