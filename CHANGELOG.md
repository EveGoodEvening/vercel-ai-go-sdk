# Changelog

All notable changes, including every exported breaking change during v0, are recorded here.

## Unreleased

### Supported behavior

- Experimental Evaluation Model V4 Go client for boolean, choice, and score evaluation questions.
- Typed request, result, metadata, retry-policy, warning, and error surfaces for evaluation calls.
- Credential-free hermetic checks, a separately gated paid live-contract workflow, and a clean local-consumer compile gate.

### Explicit non-goals

- No OpenAI-compatible `/v1/chat/completions`, `/v1/responses`, or public `/v1/evaluate` client.
- No language-generation or streaming API.
- No claim of full Vercel AI SDK or provider feature parity.
- No Gateway-native `x_search` API. Gateway support for native xAI `x_search` is unconfirmed, so it remains unsupported here; confirmed direct-xAI behavior does not establish Gateway support.

### Experimental status and risk

This is an experimental v0 surface. Exported APIs may change incompatibly during v0, and no v1 compatibility is promised while the evaluation contract remains experimental. The checks and workflows named above describe repository support and release gates; they are not evidence of a published release or a successful credentialed live run.

### Release status

No public version has been published. Release remains blocked by the owner-selected license, a hosted-repository rename and matching `origin`, completed authorized live-contract evidence, and reviewed publication/provenance configuration. No tag is created or implied by this changelog.

### Migration

Before the initial public release, the module path changed from `github.com/EveGoodEvening/vercel-ai-gateway-go-sdk` to `github.com/EveGoodEvening/vercel-ai-go-sdk`. Consumers of earlier checkouts must update `go.mod` `require`/`replace` directives and Go imports. No exported Go identifiers changed.
