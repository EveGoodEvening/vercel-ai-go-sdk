# Changelog

All notable changes, including every exported breaking change during v0, are recorded here.

## Unreleased

### Supported behavior

- Experimental Evaluation Model V4 Go client for boolean, choice, and score evaluation questions.
- Public Gateway generation through `POST /v1/responses` and `POST /v1/chat/completions`, with typed buffered and caller-owned SSE streaming calls.
- Separate `WithBaseURL` routing for provider evaluation and `WithPublicBaseURL` routing for public generation; neither base URL redirects the other contract.
- Typed request, result, metadata, retry-policy, warning, stream, and error surfaces. Responses and Chat function-tool definitions, calls, arguments, and results are transported as data; the SDK does not execute tools.
- Opt-in Chat request-only server-search declarations for `vercel:exa_search`, `vercel:parallel_search`, `vercel:perplexity_search`, and `vercel:tako_search` through `CreateChatCompletionWithServerTools` and `StreamChatCompletionWithServerTools`; hermetic acceptance covers request encoding and validation without claiming hosted live success.
- Additive request-only Responses search support through source-compatible opt-in `ResponsesBuiltInToolsRequest`, `CreateResponseWithBuiltInTools`, and `StreamResponseWithBuiltInTools`: `ResponseWebSearchTool{}` always emits fixed low-context `web_search`; fieldless `ResponseXSearchTool{}` remains unchanged; and `ResponseXSearchOptionsTool` adds the six presence-preserving `x_search` request fields `AllowedXHandles`, `ExcludedXHandles`, `FromDate`, `ToDate`, `EnableImageUnderstanding`, and `EnableVideoUnderstanding`. Nil fields are omitted, present-empty slices emit `[]`, and boolean pointers preserve explicit false/true. Existing methods, results, events, and raw-output boundaries are unchanged. Search executes server-side; the SDK performs no automatic tool execution.
- Credential-free hermetic contract gates cover evaluation plus buffered and streaming Responses and Chat over loopback, alongside separately gated paid live-contract workflows and a clean local-consumer compile gate.
- Successful raw generation JSON, stream events, evaluation bodies, prompts, tool data, headers, identifiers, and provider metadata remain sensitive evidence boundaries; repository gates and sanitized structural output do not establish hosted-service success.

### Explicit non-goals

- No public `POST /v1/evaluate` client; its success schema remains blocked on first-party evidence.
- No AI SDK internal `POST /v4/ai/language-model` provider client; generation uses the distinct public `/v1/responses` and `/v1/chat/completions` contracts.
- Responses search support is limited to the documented request fields and the evidenced routes `openai/gpt-5.4-mini` and `spacexai/grok-4.6`. For configurable `x_search`, local option-specific validation rejects only simultaneous non-empty allowed/excluded handle lists; provider-specific limits/defaults, handle syntax and duplicate semantics, date grammar/order/inclusivity/empty meaning, semantic efficacy, wrong-kind server behavior, and fields beyond the documented six remain blocked. `web_search_preview`, omitted/non-low context and other current/preview forms; wider model compatibility, fallback, search-specific selection; and typed search calls/results/actions/posts/sources/citations/annotations/refusals/provider errors/usage/cost or lifecycle events also remain blocked.
- No direct-xAI client or direct-provider compatibility promise. Direct xAI contracts do not establish Gateway option, output, validation, or model support.
- No automatic tool execution, agents, or orchestration; function-tool payload support is data transport only.
- No claim of full Vercel AI SDK or provider feature parity.
- The full public-generation and provider-evaluation live suites remain **NOT RUN / PENDING LIVE RUN**. The narrow owner-authorized Responses search evidence recorded in `docs/x-search.md` establishes only the documented request declarations on their exact model routes plus sanitized HTTP/status and output-discriminator structure; it does not establish general buffered/streaming Responses or Chat compatibility, Evaluation compatibility, configurable-option semantics or efficacy, wider-model behavior, or typed search outputs/events.

### Experimental status and risk

This is an experimental v0 surface. Exported APIs may change incompatibly during v0, and no v1 compatibility is promised while the evaluation contract remains experimental. The checks and workflows named above describe repository support and release gates; the narrow search probes are not evidence of a published release or of broader credentialed live-contract success.

### Release status

No public version has been published. Local `origin` is already `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git`; it is not a blocker. Release remains blocked by the owner-selected license, verification of the hosted repository metadata, completed broader authorized live-contract evidence, and reviewed publication/provenance configuration. The narrow search probes do not imply hosted CI, tag, proxy publication, license selection, or release success.

### Migration

Before the initial public release, the module path changed from `github.com/EveGoodEvening/vercel-ai-gateway-go-sdk` to `github.com/EveGoodEvening/vercel-ai-go-sdk`. Consumers of earlier checkouts must update `go.mod` `require`/`replace` directives and Go imports. No exported Go identifiers changed.

The configurable `x_search` request type is additive and source-compatible. Existing fieldless `ResponseXSearchTool{}` calls require no migration, and no method, result, or event contract changed.
