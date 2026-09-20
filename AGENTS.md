# Repository Agent Guidance

## Scope

This repository is planned as an evaluation-first Go SDK for Vercel's AI SDK Gateway provider protocol. Read `planning/IMPLEMENTATION_PLAN.md` before implementation. Follow its dependency order, supported-surface boundary, acceptance checks, and commit boundaries. Do not broaden the project into a general OpenAI-compatible client or claim parity with the JavaScript AI SDK.

## Lessons

- The evidence baseline recorded on 2026-09-20 is `ai@7.0.107`, `@ai-sdk/gateway@4.0.87`, `@ai-sdk/provider@4.0.17`, and `@ai-sdk/provider-utils@5.0.45`. Evaluation entered `ai` in 7.0.103 and Gateway evaluation entered `@ai-sdk/gateway` in 4.0.85. Re-check these versions and upstream source before changing the wire contract.
- The official `@ai-sdk/gateway` provider protocol is not the public OpenAI-compatible `/v1` API. Its default base URL is `https://ai-gateway.vercel.sh/v4/ai`.
- Evaluation Model V4 uses `POST /v4/ai/evaluation-model`. The model ID is carried in `ai-model-id`, not in the JSON body. Required protocol headers include `ai-gateway-protocol-version: 0.0.1`, `ai-evaluation-model-specification-version: 4`, and `ai-gateway-auth-method: api-key|oidc`.
- The evaluation JSON body contains one shared JSON-compatible `state`, a nonempty keyed `questions` map, and optional `providerOptions`. Supported question/answer variants are boolean, choice, and score. The contract includes response validation, optional rounding, usage, warnings, provider metadata, and response metadata.
- `POST https://ai-gateway.vercel.sh/v1/evaluate` is a separate public REST surface whose body includes `model`. Do not conflate it with the AI SDK provider protocol, and do not route evaluation through OpenAI-compatible chat/responses endpoints.
- Authentication precedence modeled from the official provider is explicit API key, `AI_GATEWAY_API_KEY`, then OIDC when no API key is available. An explicitly selected API key is not silently replaced by OIDC after server rejection. OIDC needs a refresh-capable token source for long-lived clients.
- Model IDs are dynamic `provider/model` strings. `typesafe-ai/jev-latest` is currently known, but the SDK must accept arbitrary nonempty IDs instead of freezing a closed enum.
- Native xAI `x_search` is confirmed for direct xAI Responses (`POST https://api.x.ai/v1/responses`) and current `@ai-sdk/xai@5.0.4`. Its raw tool is `type: "x_search"` with fields including `allowed_x_handles`, `excluded_x_handles`, `from_date`, `to_date`, `enable_image_understanding`, and `enable_video_understanding`; output can include provider-executed `x_search_call` items.
- Current direct xAI documentation allows up to 20 included/excluded handles, while `@ai-sdk/xai@5.0.4` locally limits each list to 10. This mismatch matters only if a future direct-xAI package is planned; record which contract is targeted rather than blending them.
- Native xAI `x_search` through Vercel AI Gateway is unconfirmed. Current `@ai-sdk/gateway@4.0.87` exports Exa, Parallel, Perplexity, and Tako search helpers, not `xSearch`; public Gateway `/v1` documentation and model metadata do not establish an xAI-native wire contract. Never promise or implement Gateway x_search based only on generic tool capability. Require an authenticated live contract test and first-party wire evidence, or keep it unsupported.
- Evaluation is experimental and may change in patch releases. Pin hermetic fixtures to cited upstream versions, keep releases at v0 while the contract is unstable, and require a separately gated live evaluation smoke before release.
- Default retries must remain off because evaluation may be billable and is not known to be idempotent. If callers opt in, retry only classified transient failures with bounded backoff, Retry-After support, and immediate context cancellation.
- Prefer a boring design: one root `gateway` package, small internal HTTP/test helpers, standard library first, private wire DTOs, no JavaScript monorepo mirroring, no automatic tool execution, and no language/streaming surface unless a later approved goal justifies it.
