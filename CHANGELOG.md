# Changelog

All notable changes, including every exported breaking change during v0, are recorded here.

## Unreleased

### Supported behavior

- Experimental Evaluation Model V4 Go client for boolean, choice, and score evaluation questions.
- Public Gateway generation through `POST /v1/responses` and `POST /v1/chat/completions`, with typed buffered and caller-owned SSE streaming calls.
- Separate `WithBaseURL` routing for provider evaluation and `WithPublicBaseURL` routing for public generation; neither base URL redirects the other contract.
- Typed request, result, metadata, retry-policy, warning, stream, and error surfaces. Responses and Chat function-tool definitions, calls, arguments, and results are transported as data; the SDK does not execute tools.
- Credential-free hermetic contract gates cover evaluation plus buffered and streaming Responses and Chat over loopback, alongside separately gated paid live-contract workflows and a clean local-consumer compile gate.
- Successful raw generation JSON, stream events, evaluation bodies, prompts, tool data, headers, identifiers, and provider metadata remain sensitive evidence boundaries; repository gates and sanitized structural output do not establish hosted-service success.

### Explicit non-goals

- No public `POST /v1/evaluate` client; its success schema remains blocked on first-party evidence.
- No AI SDK internal `POST /v4/ai/language-model` provider client; generation uses the distinct public `/v1/responses` and `/v1/chat/completions` contracts.
- No Responses built-in search or Chat Gateway server-search tools without sufficient first-party wire and live evidence.
- No Gateway-native xAI `x_search` API. Confirmed direct-xAI behavior does not establish Gateway support; first-party Gateway wire evidence and an authorized native live probe are still required.
- No automatic tool execution, agents, or orchestration; function-tool payload support is data transport only.
- No claim of full Vercel AI SDK or provider feature parity.
- Public generation and provider evaluation live execution remain **NOT RUN / PENDING LIVE RUN**. Hermetic fixtures are not hosted-service evidence, and the two isolated live jobs, their distinct acknowledgements, and their credential checks do not establish success until authorized runs produce sanitized evidence covering buffered and streaming Responses and Chat plus provider Evaluation. The exported continuation API also remains subject to a pre-release contract audit.

### Experimental status and risk

This is an experimental v0 surface. Exported APIs may change incompatibly during v0, and no v1 compatibility is promised while the evaluation contract remains experimental. The checks and workflows named above describe repository support and release gates; they are not evidence of a published release or a successful credentialed live run.

### Release status

No public version has been published. Local `origin` is already `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git`; it is not a blocker. Release remains blocked by the owner-selected license, verification of the hosted repository metadata, completed authorized live-contract evidence, and reviewed publication/provenance configuration. No hosted CI, live contract, tag, proxy publication, license selection, or release success is claimed or implied by this changelog.

### Migration

Before the initial public release, the module path changed from `github.com/EveGoodEvening/vercel-ai-gateway-go-sdk` to `github.com/EveGoodEvening/vercel-ai-go-sdk`. Consumers of earlier checkouts must update `go.mod` `require`/`replace` directives and Go imports. No exported Go identifiers changed.
