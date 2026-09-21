# Live-contract evidence

## Current status

| Contract | Status | Required coverage |
| --- | --- | --- |
| Public generation | **NOT RUN** | Buffered and streaming Responses; buffered and streaming Chat |
| Provider evaluation | **NOT RUN** | `Evaluate` with boolean, choice, and score questions |
| Configurable Gateway `x_search` options | **14-CALL RUN REVIEWED; INTERACTION PROBE IMPLEMENTED / NOT RUN** | Six singleton fields and the exact neutral combination accepted; ambiguous interactions remain blocked |

No owner-authorized public-generation or provider-evaluation paid live-contract run or sanitized hosted result is recorded; those protected records remain **PENDING LIVE RUN**. The separately authorized third configurable Gateway `x_search` run completed 14 serial calls on 2026-09-21 and was independently adjudicated from its sanitized structural record. It establishes request acceptance only for the six singleton fields in their tested forms and for the exact combination of both empty handle lists with both booleans false. The canonical all-six HTTP 400 is ambiguous, and the three wrong-kind HTTP 500 responses are inconclusive. No field semantics or typed output/event contract is established. The separate four-call interaction probe is implemented but **NOT RUN**. None of this clears the public-generation or provider-evaluation gate.

### Narrow Responses search evidence

Owner-authorized public Gateway probes on 2026-09-21 structurally corroborated fixed low-context `{"type":"web_search","search_context_size":"low"}` with `openai/gpt-5.4-mini` and fieldless `{"type":"x_search"}` with `spacexai/grok-4.6`, including the corresponding observed raw search-call discriminators. Those narrow probes support their exact request serialization only; configurable `x_search` acceptance is governed separately by the reviewed matrix and pending interaction gate below. None of this establishes `web_search_preview` or any other web-search form or option; wider model compatibility; direct-xAI compatibility; typed buffered search outputs, citations, or annotations; typed stream events or lifecycle semantics; optional Chat live corroboration; provider-evaluation behavior; or any other owner-authorized paid-live gate.

The durable repository evidence retains only sanitized structural facts. A non-publishable internal orchestration artifact is excluded because it contains exact prompt inputs; this record does not reproduce or paraphrase them. No credentials, authorization data, prompts, generated prose, full bodies, headers, identifiers, or raw payloads are retained here.
### Configurable Gateway `x_search` option evidence gate

The dedicated Gate A matrix harness is implemented, and its third authorized paid run completed. Owner authorization and cost acceptance are recorded without retaining any credential or credential value. Independent adjudication clears only request acceptance for `allowed_x_handles`, `excluded_x_handles`, `from_date`, `to_date`, `enable_image_understanding`, and `enable_video_understanding` in their tested singleton forms, plus the exact neutral combination containing both empty handle lists and both explicit false booleans. The canonical all-six HTTP 400 remains indivisibly ambiguous; the three wrong-kind HTTP 500 responses establish neither attributable validation rejection nor accepted wrong-kind behavior. This is a third, separate live-evidence path: it does not run in, replace, or clear either the general `public-generation` or `provider-evaluation` job.

The completed matrix selector is exactly `^TestGatewayXSearchOptionsContract$`. It sent 14 serial, buffered calls with `-count=1` to `POST https://ai-gateway.vercel.sh/v1/responses` using only `spacexai/grok-4.6`, with no retry or fallback. Each request used a strict 90-second deadline derived from one 22-minute overall test deadline. The fieldless control and nine option-bearing acceptance cases returned HTTP 200 completed responses; the canonical all-six case returned ambiguous HTTP 400; and the three wrong-kind families returned inconclusive HTTP 500. The observed structural output discriminator set does not establish output schema, semantics, requiredness, nullability, citations, results, or stream events.

Execution fails closed unless exactly one nonblank `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN` is present, the dedicated acknowledgement is exactly `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS`, both `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` and `AI_GATEWAY_LIVE_COST_ACK` are absent, and all five runtime-only private inputs are nonblank: `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`. The protected workflow supplies those variables only from GitHub secrets with the same exact names; it never accepts workflow-dispatch values. The credential-free selector `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` covers missing or wrong dedicated acknowledgement, either forbidden general acknowledgement, zero credentials, multiple credentials, and each private input missing or blank on an otherwise authorized configuration; every mismatch must fail before HTTP client or request construction and record zero `RoundTrip` calls.

An isolated wrong-type case may count as attributable request rejection only after the fieldless control succeeded, exactly one option has a different JSON kind, the response is HTTP 400 or 422, and a structurally present error object is observed. Authentication/authorization failures (401/403), timeout/conflict/rate-limit responses (408/409/429), all 5xx responses, transport failures, and known errors attributable to another category fail the evidence case; transient or service results never satisfy wrong-type rejection.

Retained output is allowlisted to the case label; structural option class set/count; HTTP status; safe top-level object/status classification; safe output discriminator/status sets; an `error_present` boolean; and sanitized error category/code classes initialized explicitly to `absent`. Missing values remain `absent` and unknown/raw provider values map to `other` rather than being copied. The harness and retained output must exclude credentials; prompts and queries; handle and date literals; request/response bodies and headers; request/response or provider IDs; generated prose; tool arguments/results; provider metadata; usage; raw events; and raw errors.

Gate A remains blocked on the unresolved interaction. The separate interaction harness is **IMPLEMENTED / NOT RUN** under exact selector `^TestGatewayXSearchOptionsInteractionContract$`. It is capped at four serial calls: fieldless control; `allowed_x_handles` plus both dates and both true booleans with `excluded_x_handles` omitted; `excluded_x_handles` plus both dates and both true booleans with `allowed_x_handles` omitted; and both non-empty handle lists with dates and booleans omitted. The two five-field cases must return HTTP 200. The handle-pair-only case may establish rejection only through safe ambiguous HTTP 400/422 with a structurally present non-excluded error; otherwise it fails closed without attributing a rule. Results may not establish option semantics, typed outputs/events, untested presence/value forms, other models, fallback, or direct-xAI behavior. The immediate next action is credential-free verification of this interaction harness, followed by its separately authorized paid run and independent sanitation/evidence review. The shared fixed-list execution/accounting contract is: after the control succeeds, all three non-control cases execute exactly once despite earlier non-control failures; structural records are logged where available, sanitized errors are accumulated, one final failure is emitted only after the fixed list when failures remain, no case is replayed, and the hard cap is four calls.

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

Separate interaction probe (**IMPLEMENTED / NOT RUN; paid execution requires owner authorization**):

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_LIVE_COST_ACK \
  AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS \
  go test -tags=livecontract ./internal/livecontract \
  -run '^TestGatewayXSearchOptionsInteractionContract$' -count=1
```

The orchestrator selects the already-present protected credential and the five private probe inputs only at execution time. The private inputs are required environment variables named `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`; their values must exist only in protected GitHub secrets with matching names and must never appear in documentation, workflow-dispatch inputs, arguments, logs, retained evidence, or repository files. Before the paid selector, run the credential-free `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` selector with credentials and acknowledgements absent; its zero-dispatch matrix also exercises each missing or blank private input on an otherwise authorized configuration. It is verification of fail-closed dispatch behavior, not paid evidence.

The [protected workflow](../.github/workflows/live-contract.yml) retains separate public-generation and provider-evaluation jobs and the API-key path. The `x-search-options` job is manual-only through `workflow_dispatch`; scheduled workflow runs cannot execute it. See [release readiness](releasing.md) for hosted-environment requirements. Those text/evaluation probes do not establish Chat server-search support or any Responses search support beyond the two exact request declarations recorded above; neither the narrow search probes nor the dedicated configurable-options gate substitutes for either protected general live job.


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

The third paid matrix run below is retained only through its sanitized structural facts and independent adjudication. The interaction probe remains **NOT RUN**.

| Field | Result | Retain only |
| --- | --- | --- |
| Authorization / cost acceptance | **RECORDED** | Boolean authorization and acceptance state; never a credential value |
| First paid attempt | **2026-09-21 — INCONCLUSIVE / CANCELLED** | One sanitized canonical record only; remaining calls cancelled |
| Second paid attempt | **2026-09-21 — INCONCLUSIVE / TIMED OUT AT FIELDLESS CONTROL** | Zero configurable option calls under the former 30-second request deadline |
| Third paid matrix run | **2026-09-21 — 14 CALLS COMPLETED; TEST FAILED ON THREE WRONG-KIND EXPECTATIONS** | 10 HTTP 200, one HTTP 400, three HTTP 500; 564.18 seconds wall-clock |
| Route / model | `POST https://ai-gateway.vercel.sh/v1/responses`; `spacexai/grok-4.6` | Exact public route and pinned model only |
| Accepted request forms | **Six singleton fields in tested forms; exact empty-lists/false-booleans combination** | HTTP 200 completed response structure; acceptance only, not semantic effect |
| Canonical all-six | **HTTP 400 — AMBIGUOUS** | Error present; category `other`; code `absent`; no object/status/output; attributes no field or interaction |
| Wrong-kind families | **HTTP 500 — INCONCLUSIVE** | Error present; category `other`; code `absent`; establishes no validation boundary or exclusive JSON type |
| Semantic/output claim | **NONE** | Structural discriminator observations do not establish option efficacy or typed outputs/events |
| Interaction probe | **IMPLEMENTED / NOT RUN** | Exact selector `^TestGatewayXSearchOptionsInteractionContract$`; four serial calls maximum |
| Next action | **Credential-free interaction verification, then authorized paid interaction run** | Do not rerun the 14-case matrix; review only sanitized structural evidence |

The four-call interaction probe uses the same fail-closed prerequisites, runtime-only private inputs, sanitation, endpoint/model, 90-second per-request deadline, 22-minute overall upper bound, and no retry/fallback. Its two maximal five-field cases must return HTTP 200; its non-empty handle-pair-only case may establish rejection only on safe ambiguous HTTP 400/422 with a structurally present non-excluded error. It otherwise fails closed without overclaim. The shared fixed-list execution/accounting contract also applies: after the control succeeds, all three non-control cases execute exactly once despite earlier non-control failures; structural records are logged where available, sanitized errors are accumulated, one final failure is emitted only after the fixed list when failures remain, no case is replayed, and the hard cap is four calls.

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
