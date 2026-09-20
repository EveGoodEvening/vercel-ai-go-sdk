# Vercel AI Gateway Go SDK

Experimental Go SDK for the Vercel AI Gateway **AI SDK Evaluation Model V4 provider protocol**.

The module requires Go 1.26.0 or newer. The initial supported toolchain window is the maintained Go 1.26 and Go 1.27 families; CI will be pinned to Go 1.26.8 and Go 1.27.1.

The public contract is being implemented in dependency-ordered chunks. The current module establishes the evaluation data model, client configuration contract, retry policy, and typed error hierarchy. It does not yet publish a runnable evaluation example.

## Support matrix

| Surface | Status |
| --- | --- |
| AI SDK Gateway provider protocol, `POST /v4/ai/evaluation-model` | Supported target; implementation in progress |
| Evaluation Model V4 boolean, choice, and score contracts | Supported target; implementation in progress |
| Arbitrary nonempty `provider/model` model IDs | Supported target |
| API-key and OIDC authentication | Supported target |
| Public REST `POST /v1/evaluate` | Not supported |
| OpenAI-compatible `/v1/chat/completions` or `/v1/responses` | Not supported |
| Language generation, streaming, embeddings, images, video, audio, reranking, realtime, or batches | Not supported |
| Agents, automatic tool execution, UI helpers, or provider registry | Not supported |
| Gateway-native xAI `x_search` | Unsupported and unconfirmed; see [`docs/x-search.md`](docs/x-search.md) |
| Direct xAI `x_search` | Confirmed upstream behavior, but no direct xAI client is provided by this SDK |
| Full parity with JavaScript `ai` or `@ai-sdk/gateway` | Not claimed |

## Experimental status

Evaluation support in the upstream AI SDK is experimental and compatibility-sensitive. This module will remain at v0 while that contract is unstable. The implementation is pinned to the upstream evidence baseline recorded in `planning/IMPLEMENTATION_PLAN.md` and deliberately avoids expanding into a general AI or OpenAI-compatible client.

## Release blocker: license

No license has been selected by the repository owner. A public release or tag is blocked until the owner selects and commits a license; this project does not guess one.
