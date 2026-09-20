# Native xAI `x_search` support decision

**Evidence reviewed:** 2026-09-20

## Release decision

Gateway-native xAI `x_search` is unsupported in this SDK. The SDK exports no `x_search` API, sends no `x_search` request, and contains no live Gateway `x_search` probe. This decision does not deny that direct xAI supports the tool; it keeps a direct-xAI contract separate from the unconfirmed Vercel AI Gateway contract.

The implemented public surface is the separate [Evaluation Model V4 provider protocol](evaluation.md). Evaluation support does not provide, imply, or transport either Gateway-native or direct-xAI `x_search`.

## Confirmed direct-xAI behavior

The official xAI X Search contract documents the direct Responses API request: `POST https://api.x.ai/v1/responses`, with native X search supplied in the top-level `tools` array as an object with `type: "x_search"`. Its wire options use snake_case, including `allowed_x_handles`, `excluded_x_handles`, `from_date`, `to_date`, `enable_image_understanding`, and `enable_video_understanding`. xAI's separate tool-usage documentation identifies provider-executed response items with `type: "x_search_call"`.

Separately, the pinned direct provider declaration, `@ai-sdk/xai@5.0.4`, documents only the JavaScript helper surface: an `xSearch` provider-executed tool with corresponding camelCase options. That helper declaration is not the source for the native wire-level facts above, and direct xAI support is not evidence that Vercel AI Gateway accepts or preserves the same native wire contract.

## Why Gateway support remains unconfirmed

The evidence baseline for this repository is `ai@7.0.107`, `@ai-sdk/gateway@4.0.87`, `@ai-sdk/provider@4.0.17`, and `@ai-sdk/provider-utils@5.0.45`. At that baseline:

- `@ai-sdk/gateway@4.0.87` has Gateway helpers for Exa, Parallel, Perplexity, and Tako search, but no `xSearch` helper.
- Public Gateway `/v1` documentation and model metadata do not establish a native xAI `x_search` endpoint, request schema, or native response evidence shape.
- A model advertising generic tool capability is insufficient evidence for a provider-native tool. Generic tool calling does not prove that Gateway accepts `type: "x_search"` or returns native `x_search_call` evidence.

Accordingly, this evaluation-focused Go SDK does not infer a Gateway contract, alias direct xAI behavior into Gateway, or add speculative API surface.

## Evidence required to reconsider

Only a future reviewed implementation plan may change this decision. It must be backed by both:

1. a first-party Gateway wire contract identifying the endpoint, protocol/request schema, native response evidence shape, and an explicit suitable model ID; and
2. an authenticated contract result demonstrating native `x_search_call` evidence rather than merely generated text.

Any future probe must separately define credential inputs, explicit cost acknowledgement, sanitization rules, and success criteria. Until all prerequisites exist, no placeholder test, skipped live test, invented request, exported API, or implementation claim belongs in this repository. The blocked probe is outside the evaluation release and is not a release prerequisite.

## Reviewed sources

- Official xAI X Search wire contract: <https://docs.x.ai/developers/tools/x-search>
- Official xAI provider-executed output types: <https://docs.x.ai/developers/tools/tool-usage-details>
- Direct-provider JavaScript helper declaration, `@ai-sdk/xai@5.0.4`: <https://unpkg.com/@ai-sdk/xai@5.0.4/dist/index.d.ts>
- Gateway provider source, `ai@7.0.107`: <https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-provider.ts>
- Gateway tool registry, `ai@7.0.107`: <https://github.com/vercel/ai/blob/ai%407.0.107/packages/gateway/src/gateway-tools.ts>
- Vercel AI Gateway documentation: <https://vercel.com/docs/ai-gateway>

These citations record the pinned evidence used for the 2026-09-20 release decision; later direct-provider or documentation changes do not silently broaden this SDK's supported surface.
