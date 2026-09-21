# Live-contract evidence

## Current status

| Contract | Status | Required coverage |
| --- | --- | --- |
| Public generation | **NOT RUN** | Buffered and streaming Responses; buffered and streaming Chat |
| Provider evaluation | **NOT RUN** | `Evaluate` with boolean, choice, and score questions |
| Configurable Gateway `x_search` options | **EVIDENCE CHAIN COMPLETE; PLAN REVIEW GATE CLEAN; NEXT ACTION IS THE FIRST DEPENDENCY-READY ITEM IN THE AUTHORITATIVE CHUNK 35–52 QUEUE** | `ResponseXSearchOptionsTool` six singleton fields, exact neutral combination, allowed and excluded maximal five-field combinations, and local non-empty handle-list mutual exclusion cleared only in the tested forms |

No owner-authorized public-generation or provider-evaluation paid live-contract run or sanitized hosted result is recorded; those protected records remain **PENDING LIVE RUN**. The separately authorized configurable Gateway `x_search` evidence chain is complete: the reviewed 14-call matrix, reviewed four-call interaction run, and reviewed focused two-call run all occurred on 2026-09-21. The focused run returned HTTP 200 completed responses for its fieldless control and the excluded maximal five-field case and passed in 132.84 seconds. Together, the runs support request acceptance only for `ResponseXSearchOptionsTool`'s six singleton fields in their tested forms, the exact neutral combination, and both maximal five-field combinations, plus local validation rejecting simultaneous non-empty handle lists. Gate A and Chunks 31, 31A, and 32 through accounting `2ff0005` are complete. Chunk 33 reconciliation, verification, reviews, and its tracker-only accounting handoff `0dcb51dfb676e8fa7b838560603fd7802f33446f` (`docs: record gateway x search option reconciliation`) are complete. Chunk 34 is complete. The Chunk 35 plan review gate is clean, and the next action is the first dependency-ready item in the authoritative Chunk 35–52 queue. Arbitrary subsets and every semantic and output boundary listed below remain unresolved.

### Narrow Responses search evidence

Owner-authorized public Gateway probes on 2026-09-21 structurally corroborated fixed low-context `{"type":"web_search","search_context_size":"low"}` with `openai/gpt-5.4-mini` and fieldless `{"type":"x_search"}` with `spacexai/grok-4.6`, including the corresponding observed raw search-call discriminators. Those narrow probes support their exact request serialization only; configurable `x_search` acceptance on `spacexai/grok-4.6` is governed separately by the completed Gate A evidence below. None of this establishes `web_search_preview` or any other web-search form or option; wider model compatibility; direct-xAI compatibility; typed buffered search outputs, citations, or annotations; typed stream events or lifecycle semantics; optional Chat live corroboration; provider-side option efficacy; limits; date semantics; wrong-kind behavior; or any untested request form.

The durable repository evidence retains only sanitized structural facts. A non-publishable internal orchestration artifact is excluded because it contains exact prompt inputs; this record does not reproduce or paraphrase them. No credentials, authorization data, prompts, generated prose, full bodies, headers, identifiers, or raw payloads are retained here.
### Configurable Gateway `x_search` option evidence gate

The dedicated Gate A evidence chain is complete. Owner authorization and cost acceptance are recorded without retaining any credential or credential value. Independent adjudication clears only request acceptance for `ResponseXSearchOptionsTool`'s `allowed_x_handles`, `excluded_x_handles`, `from_date`, `to_date`, `enable_image_understanding`, and `enable_video_understanding` in their tested singleton forms; the exact neutral combination containing both empty handle lists and both explicit false booleans; and the two maximal five-field combinations, each containing exactly one non-empty handle list, both date strings, and both true booleans. Omission is accepted through the separate fieldless `ResponseXSearchTool` controls. The reviewed safe handle-pair-only HTTP 400 requires local rejection when both handle lists are simultaneously non-empty. The canonical all-six HTTP 400 remains indivisibly ambiguous; the three wrong-kind HTTP 500 responses establish neither attributable validation rejection nor accepted wrong-kind behavior. This evidence proves request acceptance only for `spacexai/grok-4.6`, not arbitrary subsets, semantics, limits, date behavior, output/event typing, or direct-xAI behavior.

The completed matrix selector is exactly `^TestGatewayXSearchOptionsContract$`. It sent 14 serial, buffered calls with `-count=1` to `POST https://ai-gateway.vercel.sh/v1/responses` using only `spacexai/grok-4.6`, with no retry or fallback. Each request used a strict 90-second deadline derived from one 22-minute overall test deadline. The fieldless control and nine option-bearing acceptance cases returned HTTP 200 completed responses; the canonical all-six case returned ambiguous HTTP 400; and the three wrong-kind families returned inconclusive HTTP 500. The observed structural output discriminator set does not establish output schema, semantics, requiredness, nullability, citations, results, or stream events.

Execution fails closed unless exactly one nonblank `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN` is present, the dedicated acknowledgement is exactly `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS`, both `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` and `AI_GATEWAY_LIVE_COST_ACK` are absent, and all five runtime-only private inputs are nonblank: `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`. The protected workflow supplies those variables only from GitHub secrets with the same exact names; it never accepts workflow-dispatch values. The credential-free selector `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` covers missing or wrong dedicated acknowledgement, either forbidden general acknowledgement, zero credentials, multiple credentials, and each private input missing or blank on an otherwise authorized configuration; every mismatch must fail before HTTP client or request construction and record zero `RoundTrip` calls.

An isolated wrong-type case may count as attributable request rejection only after the fieldless control succeeded, exactly one option has a different JSON kind, the response is HTTP 400 or 422, and a structurally present error object is observed. Authentication/authorization failures (401/403), timeout/conflict/rate-limit responses (408/409/429), all 5xx responses, transport failures, and known errors attributable to another category fail the evidence case; transient or service results never satisfy wrong-type rejection.

Retained output is allowlisted to the case label; structural option class set/count; HTTP status; safe top-level object/status classification; safe output discriminator/status sets; an `error_present` boolean; and sanitized error category/code classes initialized explicitly to `absent`. Missing values remain `absent` and unknown/raw provider values map to `other` rather than being copied. The harness and retained output must exclude credentials; prompts and queries; handle and date literals; request/response bodies and headers; request/response or provider IDs; generated prose; tool arguments/results; provider metadata; usage; raw events; and raw errors.

The reviewed four-call interaction run used exact selector `^TestGatewayXSearchOptionsInteractionContract$` and the same fail-closed prerequisites, runtime-only private inputs, sanitation, endpoint/model, 90-second per-request contexts, 22-minute overall bound, and no retry/fallback. Its fixed cases produced: fieldless control HTTP 200; allowed handles plus both dates and both true booleans HTTP 200; both non-empty handle lists with dates and booleans omitted HTTP 400 under the safe rejection rule; and an execution failure for excluded handles plus both dates and both true booleans. The test failed only after all fixed cases were attempted. The reviewed evidence therefore supports the allowed maximal combination and local validation rejecting simultaneous non-empty allowed and excluded lists; the focused run below resolves the excluded maximal case. Results establish no option semantics, typed outputs/events, untested presence/value forms, other models, fallback, or direct-xAI behavior.

The reviewed focused two-call run used exact selector `^TestGatewayXSearchOptionsExcludedInteractionContract$` with the same fail-closed prerequisites, runtime-only private inputs, sanitation, endpoint/model, 90-second per-request contexts, 22-minute overall bound, and no retry/fallback. Its fieldless control and excluded handles plus both dates and both true booleans both returned HTTP 200 completed responses; the test passed in 132.84 seconds. This clears only that exact excluded maximal five-field combination. All positive responses across the complete evidence chain structurally contained `x_search_call`; that observation does not establish semantic efficacy, child schemas, citations/results, or typed buffered/streaming output.

## Pinned contracts

| Contract | Model | Endpoint |
| --- | --- | --- |
| Responses | `openai/gpt-5-nano` | `POST https://ai-gateway.vercel.sh/v1/responses` |
| Chat | `openai/gpt-5-nano` | `POST https://ai-gateway.vercel.sh/v1/chat/completions` |
| Provider evaluation | `typesafe-ai/jev` | `POST https://ai-gateway.vercel.sh/v4/ai/evaluation-model` |

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

Completed configurable Gateway `x_search` matrix (do not rerun for interaction evidence):

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_LIVE_COST_ACK \
  AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS \
  go test -tags=livecontract ./internal/livecontract \
  -run '^TestGatewayXSearchOptionsContract$' -count=1
```

Focused excluded-interaction follow-up (**COMPLETED; do not rerun for Gate A evidence**):

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_LIVE_COST_ACK \
  AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS \
  go test -tags=livecontract ./internal/livecontract \
  -run '^TestGatewayXSearchOptionsExcludedInteractionContract$' -count=1
```

The orchestrator selects the already-present protected credential and the five private probe inputs only at execution time. The private inputs are required environment variables named `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`; their values must exist only in protected GitHub secrets with matching names and must never appear in documentation, workflow-dispatch inputs, arguments, logs, retained evidence, or repository files. Before the paid selector, run the credential-free `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` selector with credentials and acknowledgements absent; its zero-dispatch matrix also exercises each missing or blank private input on an otherwise authorized configuration. It is verification of fail-closed dispatch behavior, not paid evidence.

The [protected workflow](../.github/workflows/live-contract.yml) retains separate public-generation and provider-evaluation jobs and the API-key path. Configurable `x_search` execution is manual-only through the single required `workflow_dispatch` choice `x_search_probe`, whose values are `matrix`, `interaction`, and `excluded-interaction` and whose default is `matrix`. Those values exact-select three mutually exclusive manual-only jobs; scheduled workflow runs execute none of them. See [release readiness](releasing.md) for hosted-environment requirements. These search probes do not establish Chat server-search support or any Responses search support beyond the exact request declarations and reviewed configurable boundaries recorded here; none substitutes for either protected general live job.


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

## Sanitized configurable `x_search` option record

The third paid matrix run, four-call interaction run, and focused two-call run below are retained only through sanitized structural facts and independent adjudication. Gate A and Chunks 31, 31A, and 32 through accounting `2ff0005` are complete. Chunk 33 reconciliation, verification, reviews, and its tracker-only accounting handoff `0dcb51dfb676e8fa7b838560603fd7802f33446f` (`docs: record gateway x search option reconciliation`) are complete. Chunk 34 is complete. The Chunk 35 plan review gate is clean, and the next action is the first dependency-ready item in the authoritative Chunk 35–52 queue.

| Field | Result | Retain only |
| --- | --- | --- |
| Authorization / cost acceptance | **RECORDED** | Boolean authorization and acceptance state; never a credential value |
| First paid attempt | **2026-09-21 — INCONCLUSIVE / CANCELLED** | One sanitized canonical record only; remaining calls cancelled |
| Second paid attempt | **2026-09-21 — INCONCLUSIVE / TIMED OUT AT FIELDLESS CONTROL** | Zero configurable option calls under the former 30-second request deadline |
| Third paid matrix run | **2026-09-21 — 14 CALLS COMPLETED; TEST FAILED ON THREE WRONG-KIND EXPECTATIONS** | 10 HTTP 200, one HTTP 400, three HTTP 500; 564.18 seconds wall-clock |
| Route / model | `POST https://ai-gateway.vercel.sh/v1/responses`; `spacexai/grok-4.6` | Exact public route and pinned model only |
| Accepted request forms | **Six singleton fields in tested forms; exact empty-lists/false-booleans combination; both exact maximal five-field combinations** | HTTP 200 completed response structure; acceptance only, not semantic effect |
| Canonical all-six | **HTTP 400 — AMBIGUOUS** | Error present; category `other`; code `absent`; no object/status/output; attributes no field or interaction |
| Wrong-kind families | **HTTP 500 — INCONCLUSIVE** | Error present; category `other`; code `absent`; establishes no validation boundary or exclusive JSON type |
| Semantic/output claim | **NONE** | Structural discriminator observations do not establish option efficacy or typed outputs/events |
| Four-call interaction run | **2026-09-21 — TEST FAILED AFTER ALL FIXED CASES** | Fieldless HTTP 200; allowed maximal five-field HTTP 200; non-empty handle-pair-only HTTP 400 safe rejection; excluded maximal execution failure/inconclusive |
| Focused excluded-interaction run | **2026-09-21 — PASS; TWO CALLS COMPLETED** | Fieldless HTTP 200; excluded maximal five-field HTTP 200; 132.84 seconds wall-clock |
| Evidence adjudication | **GATE A AND CHUNKS 31/31A/32 COMPLETE** | `ResponseXSearchOptionsTool` six singletons, neutral empty/false combination, both maximal five-field combinations, omission through fieldless `ResponseXSearchTool`, and required local non-empty allow/exclude mutual exclusion only |
| Next action | **First dependency-ready item in the authoritative Chunk 35–52 queue** | Chunk 34 is complete; the Chunk 35 plan review gate is clean. Do not rerun completed probes |

The complete reviewed evidence supports request acceptance on `spacexai/grok-4.6` only for `ResponseXSearchOptionsTool`'s six tested singleton forms, the exact neutral empty-list/explicit-false form, and the allowed and excluded maximal five-field combinations, and requires local validation rejecting simultaneous non-empty `allowed_x_handles` and `excluded_x_handles`. The separate fieldless `ResponseXSearchTool` remains supported. The implemented Go API represents handle lists as `[]string`, dates as strings, and booleans with explicit presence support. It does not claim server-side rejection of wrong JSON kinds; those kinds are unrepresentable through the typed API. No evidence establishes arbitrary subsets, list limits, handle syntax, duplicates, null handling, date grammar/order/inclusivity, empty date strings, semantic effects, other combinations, other models, typed outputs/events, or direct-xAI behavior.

No private input value, credential, body, header, ID, raw error, or generated content was retained or reconstructed. Do not add an operator/run reference, literal option value, or any excluded content to this record.

Stop release-candidate promotion on protocol drift. Never infer missing observations or reconstruct them from fixtures.

## Sanitization review

Apply to this record and every retained CI artifact:

- [ ] No credentials, acknowledgement values, authorization headers, or request/response IDs.
- [ ] No prompts, state, generated output, answer values, finish-reason values, tool arguments/results, or raw payloads/headers.
- [ ] No provider-option/metadata values, token counts, warning payloads, or other billable-content details.
- [ ] Model evidence records only pinned-model match; no unexpected model value is retained.
- [ ] Each result is attributable to its exact authorized command and isolated job, without the opposite acknowledgement.
- [ ] An independent reviewer checked the retained evidence; fixtures and skipped/failed gates are not recorded as live success.
- [ ] Configurable `x_search` evidence contains only its explicit structural allowlist; unknown/raw values were categorized rather than copied, and no prompt/query/handle/date literal, body, header, ID, generated content, provider metadata, usage, or raw error was retained.
