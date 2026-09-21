# Live-contract evidence

## Current status

| Contract | Status | Required coverage |
| --- | --- | --- |
| Public generation | **NOT RUN** | Buffered and streaming Responses; buffered and streaming Chat |
| Provider evaluation | **NOT RUN** | `Evaluate` with boolean, choice, and score questions |

No owner-authorized paid live-contract run or sanitized hosted result is recorded; the protected public-generation and provider-evaluation records below remain **PENDING LIVE RUN**. Hermetic tests, successful compilation, credential-gate failures, and the separate narrow search probes below are not hosted evidence and do not clear either live-contract gate.

### Narrow Responses search evidence

Owner-authorized public Gateway probes on 2026-09-21 structurally corroborated only fixed low-context `{"type":"web_search","search_context_size":"low"}` with `openai/gpt-5.4-mini` and fieldless `{"type":"x_search"}` with `spacexai/grok-4.6`, including the corresponding observed raw search-call discriminators. This supports exact request serialization only. It does not establish configurable `x_search` options; `web_search_preview` or any other web-search form or option; wider model compatibility; direct-xAI compatibility; typed buffered search outputs, citations, or annotations; typed stream events or lifecycle semantics; optional Chat live corroboration; provider-evaluation behavior; or any other owner-authorized paid-live gate.

The durable repository evidence retains only sanitized structural facts. A non-publishable internal orchestration artifact is excluded because it contains exact prompt inputs; this record does not reproduce or paraphrase them. No credentials, authorization data, prompts, generated prose, full bodies, headers, identifiers, or raw payloads are retained here.

## Pinned contracts

| Contract | Model | Endpoint |
| --- | --- | --- |
| Responses | `openai/gpt-5-nano` | `POST https://ai-gateway.vercel.sh/v1/responses` |
| Chat | `openai/gpt-5-nano` | `POST https://ai-gateway.vercel.sh/v1/chat/completions` |
| Provider evaluation | `typesafe-ai/jev-latest` | `POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model` |

Evaluation protocol baseline, recorded 2026-09-20: `ai@7.0.107`, `@ai-sdk/gateway@4.0.87`, `@ai-sdk/provider@4.0.17`, and `@ai-sdk/provider-utils@5.0.45`. These are evidence pins, not Go dependencies or a claim of JavaScript SDK parity.

## Authorized isolated commands

**Paid operations: run only with owner authorization.** Each test process requires exactly one nonblank credential (`AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN`), its exact cost acknowledgement, and the opposite acknowledgement entirely unset—even an empty value is rejected. Missing or conflicting prerequisites fail before network work.

These commands select an already-present API key. For an authorized OIDC run, unset `AI_GATEWAY_API_KEY` instead of `VERCEL_OIDC_TOKEN`. Never run both contract groups in one process or set both acknowledgements.

Public generation:

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_LIVE_COST_ACK \
  AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS \
  go test -tags=livecontract ./internal/livecontract \
  -run '^TestGateway(Responses|Chat)Contract$' -count=1
```

Provider evaluation:

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK \
  AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS \
  go test -tags=livecontract ./internal/livecontract \
  -run '^TestGatewayEvaluationContract$' -count=1
```

The [protected workflow](../.github/workflows/live-contract.yml) uses separate jobs and the API-key path. See [release readiness](releasing.md) for hosted-environment requirements. These text/evaluation probes do not establish Chat server-search support or any Responses search support beyond the two exact request declarations recorded above; the narrow search probes do not substitute for these protected live jobs.


## Sanitized public generation record

Complete only from actual runs of [public_test.go](../internal/livecontract/public_test.go). The probe checks the pinned model wherever exposed, nonblank buffered Chat finish reasons, buffered Responses output-item discriminators, and a terminal `response.completed` event in the Responses stream. Those are live-test assertions, not extra typed SDK output guarantees.

| Field | Result | Retain only |
| --- | --- | --- |
| Execution date / run reference | **PENDING LIVE RUN** | UTC date and non-secret operator/run reference |
| Model | **PENDING LIVE RUN** | Exact-match pass/fail for the pinned model, not unexpected model values |
| Responses buffered | **PENDING LIVE RUN** | Pass/fail and structural output-item types |
| Responses stream | **PENDING LIVE RUN** | Pass/fail, event types, terminal `response.completed` presence |
| Chat buffered | **PENDING LIVE RUN** | Pass/fail and finish-reason presence, not values |
| Chat stream | **PENDING LIVE RUN** | Pass/fail and pinned-model match where exposed |
| Usage | **PENDING LIVE RUN** | Presence and field names, not counts |
| Response metadata | **PENDING LIVE RUN** | Model-match and metadata-presence flags, not IDs/headers/body |
| Protocol drift | **PENDING LIVE RUN** | Sanitized structural differences; `none observed` only after all four paths pass |

## Sanitized provider evaluation record

Complete only from actual runs of [evaluation_test.go](../internal/livecontract/evaluation_test.go).

| Field | Result | Retain only |
| --- | --- | --- |
| Execution date / run reference | **PENDING LIVE RUN** | UTC date and non-secret operator/run reference |
| Result / model | **PENDING LIVE RUN** | Pass/fail and exact pinned-model match |
| Answers | **PENDING LIVE RUN** | Boolean, choice, and score type presence, not values |
| Rounding / usage | **PENDING LIVE RUN** | Presence and integer-field names, not values |
| Warnings | **PENDING LIVE RUN** | Warning discriminators, not payloads |
| Provider metadata | **PENDING LIVE RUN** | Presence; provider names only after sensitivity review |
| Response metadata | **PENDING LIVE RUN** | Model-match, non-nil headers/body, body byte length |
| Protocol drift | **PENDING LIVE RUN** | Sanitized structural differences; `none observed` only after success |

Stop release-candidate promotion on protocol drift. Never infer missing observations or reconstruct them from fixtures.

## Sanitization review

Apply to this record and every retained CI artifact:

- [ ] No credentials, acknowledgement values, authorization headers, or request/response IDs.
- [ ] No prompts, state, generated output, answer values, finish-reason values, tool arguments/results, or raw payloads/headers.
- [ ] No provider-option/metadata values, token counts, warning payloads, or other billable-content details.
- [ ] Model evidence records only pinned-model match; no unexpected model value is retained.
- [ ] Each result is attributable to its exact authorized command and isolated job, without the opposite acknowledgement.
- [ ] An independent reviewer checked the retained evidence; fixtures and skipped/failed gates are not recorded as live success.
