# Live-contract evidence

## Current status

| Contract | Status | Required coverage |
| --- | --- | --- |
| Public generation | **NOT RUN** | Buffered and streaming Responses; buffered and streaming Chat |
| Provider evaluation | **NOT RUN** | `Evaluate` with boolean, choice, and score questions |
| Configurable Gateway `x_search` options | **IMPLEMENTED; FIRST ATTEMPT INCONCLUSIVE / CANCELLED** | Corrected fail-closed prerequisite gate plus bounded structural option matrix; rerun pending |

No owner-authorized public-generation or provider-evaluation paid live-contract run or sanitized hosted result is recorded; those protected records remain **PENDING LIVE RUN**. The owner separately authorized the dedicated configurable Gateway `x_search` evidence run and accepted its bounded cost. Its first paid attempt on 2026-09-21 is **INCONCLUSIVE / CANCELLED** after one sanitized canonical record; it cleared or rejected no option field. Hermetic tests, successful compilation, credential-gate failures, search probe/harness implementation, and this inconclusive attempt do not clear any live-contract gate.

### Narrow Responses search evidence

Owner-authorized public Gateway probes on 2026-09-21 structurally corroborated only fixed low-context `{"type":"web_search","search_context_size":"low"}` with `openai/gpt-5.4-mini` and fieldless `{"type":"x_search"}` with `spacexai/grok-4.6`, including the corresponding observed raw search-call discriminators. This supports exact request serialization only. It does not establish configurable `x_search` options; `web_search_preview` or any other web-search form or option; wider model compatibility; direct-xAI compatibility; typed buffered search outputs, citations, or annotations; typed stream events or lifecycle semantics; optional Chat live corroboration; provider-evaluation behavior; or any other owner-authorized paid-live gate.

The durable repository evidence retains only sanitized structural facts. A non-publishable internal orchestration artifact is excluded because it contains exact prompt inputs; this record does not reproduce or paraphrase them. No credentials, authorization data, prompts, generated prose, full bodies, headers, identifiers, or raw payloads are retained here.
### Configurable Gateway `x_search` option evidence gate

The dedicated Gate A harness is implemented. Owner authorization and cost acceptance are recorded without retaining any credential or credential value. The first paid attempt is **INCONCLUSIVE / CANCELLED** and cleared no option field; the corrected verification and authorized rerun remain pending. This is a third, separate live-evidence path: it does not run in, replace, or clear either the general `public-generation` or `provider-evaluation` job.

The paid selector is exactly `^TestGatewayXSearchOptionsContract$`. It sends serial, buffered requests with `-count=1` to `POST https://ai-gateway.vercel.sh/v1/responses` using only `spacexai/grok-4.6`. It makes no retry and performs no fallback. Every request has a strict 30-second deadline derived from one 8-minute overall test deadline; the protected workflow job has `timeout-minutes: 10`, so worst-case test time remains below the workflow bound. The corrected sequence sends the fieldless control first and aborts before every option-bearing case unless that control succeeds. Its static option matrix contains one combined canonical request with all six candidate fields present; explicit empty-list and explicit-false forms; `from_date` alone; `to_date` alone; and grouped wrong-type attribution cases for handle lists, dates, and booleans. Only if the combined canonical case fails may six independently controlled one-field valid diagnostics run; that canonical failure does not itself clear, reject, or attribute any field. The hard-coded ceiling remains 14 requests.
Canonical diagnostics expand only for HTTP 400/422 with structurally present error and no known excluded auth/permission/rate/timeout/conflict/server/transient/service category/code; absent errors or excluded classes fail without expansion.

Execution fails closed unless exactly one nonblank `AI_GATEWAY_API_KEY` or `VERCEL_OIDC_TOKEN` is present, the dedicated acknowledgement is exactly `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS`, both `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` and `AI_GATEWAY_LIVE_COST_ACK` are absent, and all five runtime-only private inputs are nonblank: `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`. The protected workflow supplies those variables only from GitHub secrets with the same exact names; it never accepts workflow-dispatch values. The credential-free selector `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` covers missing or wrong dedicated acknowledgement, either forbidden general acknowledgement, zero credentials, multiple credentials, and each private input missing or blank on an otherwise authorized configuration; every mismatch must fail before HTTP client or request construction and record zero `RoundTrip` calls.

An isolated wrong-type case may count as attributable request rejection only after the fieldless control succeeded, exactly one option has a different JSON kind, the response is HTTP 400 or 422, and a structurally present error object is observed. Authentication/authorization failures (401/403), timeout/conflict/rate-limit responses (408/409/429), all 5xx responses, transport failures, and known errors attributable to another category fail the evidence case; transient or service results never satisfy wrong-type rejection.

Retained output is allowlisted to the case label; structural option class set/count; HTTP status; safe top-level object/status classification; safe output discriminator/status sets; an `error_present` boolean; and sanitized error category/code classes initialized explicitly to `absent`. Missing values remain `absent` and unknown/raw provider values map to `other` rather than being copied. The harness and retained output must exclude credentials; prompts and queries; handle and date literals; request/response bodies and headers; request/response or provider IDs; generated prose; tool arguments/results; provider metadata; usage; raw events; and raw errors.

Gate A remains blocked pending corrected prerequisite verification, an owner-authorized rerun of the corrected paid selector, sanitation review, and independent evidence review. Results may clear only the exact request fields and exact presence forms actually observed. A canonical combined failure must not clear, reject, or attribute any individual field; its independently controlled one-field valid diagnostics may still run. No result may silently clear an unisolated field, omitted/null behavior, an untested value class, validation rule, cross-field interaction, other model, fallback route, typed output/event contract, or direct-xAI behavior.
Canonical diagnostics expand only for HTTP 400/422 with structurally present error and no known excluded auth/permission/rate/timeout/conflict/server/transient/service category/code; absent errors or excluded classes fail without expansion.

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

Configurable Gateway `x_search` options (**authorized; first attempt INCONCLUSIVE / CANCELLED; corrected rerun pending**):

```sh
env -u VERCEL_OIDC_TOKEN -u AI_GATEWAY_PUBLIC_LIVE_COST_ACK -u AI_GATEWAY_LIVE_COST_ACK \
  AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS \
  go test -tags=livecontract ./internal/livecontract \
  -run '^TestGatewayXSearchOptionsContract$' -count=1
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

Complete only from the dedicated paid selector after the prerequisite selector passes. The first paid attempt below is retained solely as sanitized inconclusive accounting; it is not field evidence.

| Field | Result | Retain only |
| --- | --- | --- |
| Authorization / cost acceptance | **RECORDED** | Boolean authorization and acceptance state; never a credential value |
| First paid attempt | **2026-09-21 — INCONCLUSIVE / CANCELLED** | One sanitized record only; the remaining serial diagnostic matrix was cancelled |
| Route | `POST https://ai-gateway.vercel.sh/v1/responses`; `spacexai/grok-4.6` | Exact endpoint and model only |
| Case | `canonical-all-six` | Case label only; no option values |
| HTTP / top-level shape | HTTP 400; object `absent`; status `absent` | Status and safe structural classes only |
| Output shape | No outputs | Presence only; no generated content |
| Error shape | Error category `other`; code `absent` | Safe classes only; no unknown/raw value or body |
| Cleared or rejected request surface | **NONE** | The ambiguous canonical 400 does not attribute, clear, or reject any field |
| Next action | **Corrected verification, then owner-authorized rerun** | Run the fieldless control first under the corrected bounds before option expansion |
The first-attempt record remains sanitized accounting only. Canonical diagnostics expand only for HTTP 400/422 with structurally present error and no known excluded auth/permission/rate/timeout/conflict/server/transient/service category/code; absent errors or excluded classes fail without expansion.

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
