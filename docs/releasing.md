# Release readiness and policy

## Current status

**Unreleased; do not create a candidate tag yet.** Release is blocked on:

- an owner-selected, committed license;
- hosted repository metadata, protected CI/environment, and publication/provenance review;
- separately authorized public-generation and provider-evaluation live results;
- owner authorization to tag and publish.

The protected public-generation and provider-evaluation [live evidence record](evaluation-live-evidence.md) is still **NOT RUN / PENDING LIVE RUN**. The dedicated configurable Gateway `x_search` harness is implemented, but both paid attempts on 2026-09-21 are **INCONCLUSIVE**. The first was cancelled after one sanitized `canonical-all-six` HTTP 400 record; the second timed out at the fieldless control after exactly 30.02 seconds under the prior 30-second deadline with sanitized `dispatch failed`, so zero configurable option cases ran. Neither attempt cleared or rejected an option field; corrected verification and an owner-authorized rerun remain pending. Narrow Responses search probes establish only exact fieldless request declarations and observed structural discriminators; they are not hosted contract results. Local fixtures and workflow definitions likewise do not establish hosted success, and a matching local remote URL is not verification of the hosted repository.

## Pre-tag checklist

Record evidence from the release commit for each gate. This checklist does not itself authorize creating, moving, pushing, or publishing tags.

### Offline verification

The exact pins in [ci.yml](../.github/workflows/ci.yml) are:

| Go version | Required checks |
| --- | --- |
| 1.26.8 and 1.27.1 | Ordinary suite, including compilation of example packages |
| 1.27.1 | Race detector, `go vet`, clean external-consumer check |

For every ordinary command, unset `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, `AI_GATEWAY_PUBLIC_LIVE_COST_ACK`, and `AI_GATEWAY_LIVE_COST_ACK`. The test-wide transport guard rejects non-loopback destinations. Updating a Go pin requires reviewed plan/evidence changes; do not use floating versions.

- [ ] Protected hosted CI passes the matrix above from the release commit; local results alone are insufficient.
- [ ] [`scripts/verify-local-consumer.sh`](../scripts/verify-local-consumer.sh) passes from a clean checkout. It builds an external module with a local `replace` and proxy access disabled, audits supported exports, and rejects obsolete `ConfigError` and exported retry hooks/aliases.
- [ ] The API review covers every exported surface and exactly the five SDK error types. Audit Responses `PreviousResponseID` and caller-managed Chat history against current first-party evidence; do not infer hosted continuation support from fixtures.
- [ ] Examples, guides, [changelog](../CHANGELOG.md), and release notes agree that Responses search support is request-only for fixed low-context `web_search` and fieldless `x_search`; configurable options, other web-search forms, typed outputs/events, wider compatibility, and direct-xAI support remain blocked.

### Hosted contracts and publication

- [ ] The isolated `public-generation` live job passes all four text paths: buffered/streaming Responses and buffered/streaming Chat.
- [ ] The isolated `provider-evaluation` live job passes `Evaluate`. Neither job's evidence substitutes for the other. Use the exact authorization rules and commands in [live-contract evidence](evaluation-live-evidence.md), then independently review the sanitized results.
- [ ] Separately, verify `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` with credentials and acknowledgements absent, including each missing or blank private input on an otherwise authorized configuration, then have the authorized orchestrator rerun `^TestGatewayXSearchOptionsContract$` once with `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS`, both general acknowledgements absent, exactly one protected nonblank credential, and all five nonblank runtime-only private inputs. The corrected selector must use one 22-minute overall deadline, derive each strict 90-second request context from it, run the fieldless control first, and abort before option-bearing cases unless that control succeeds. The job has a 25-minute workflow timeout, no retry/fallback, and at most 14 serial requests. Review only the structural allowlist; retain no private value, body, header, ID, output, or raw error.
- [ ] The owner has selected and committed `LICENSE`.
- [ ] The local remote matches `github.com/EveGoodEvening/vercel-ai-go-sdk`, and hosted name, visibility, default branch, protection rules, and release settings are independently verified.
- [ ] Release-environment protection, proxy visibility, artifact attestations/provenance, and publication permissions are reviewed. Ordinary CI stays read-only; do not grant it write or identity-token permissions.
- [ ] Retained evidence passes the [sanitization checklist](evaluation-live-evidence.md#sanitization-review), and the owner authorizes publication.

After these gates and authorization, follow the [continuation release sequence](../planning/IMPLEMENTATION_PLAN.md#chunk-24--continuation-release-verification): create the immutable candidate, verify direct-VCS and public-proxy imports, compile the fetched module, retain checksum/provenance evidence, and review before promotion/publication. None of those results is claimed here.

## Live workflow boundary

[`live-contract.yml`](../.github/workflows/live-contract.yml) remains the only CI workflow permitted to make the general paid Gateway calls. Its manual/scheduled `public-generation` and `provider-evaluation` jobs remain isolated and use the protected `live-evaluation` environment, pinned toolchain, API-key secret, exact job-specific acknowledgement, and absent opposite acknowledgement. The configurable `x_search` Gate A harness is a separate authorized selector whose two paid attempts were **INCONCLUSIVE**, not completed workflow results. Its workflow job is conditional on `github.event_name == 'workflow_dispatch'`, so the weekly schedule cannot dispatch it, and has `timeout-minutes: 25`. It additionally requires the exact dedicated acknowledgement, absence of both general acknowledgements, exactly one API-key or OIDC credential, all five runtime-only protected inputs, and the same pinned toolchain and environment.

Chat server-search request encoding is covered hermetically; optional live corroboration has not run and is not part of that feature's implementation acceptance. Responses built-in search remains request-only for exact `{"type":"web_search","search_context_size":"low"}` and fieldless `{"type":"x_search"}` declarations. The existing structural probes corroborate only those declarations and structural discriminators. The corrected configurable-options harness runs the fieldless control before any option-bearing request and aborts expansion if it fails. Each request has a strict 90-second deadline derived from one 22-minute overall deadline. The option matrix covers combined all-six presence, explicit empty lists and false booleans, from-only, to-only, and grouped wrong handle-list/date/boolean types; a failed canonical combined case may add six independently controlled one-field valid diagnostics, but cannot itself clear, reject, or attribute any field. An isolated wrong-type 400/422 counts only after control success, when exactly one option changes JSON kind and a structural error object is present; auth, permission, rate, timeout, conflict, server, transport, or other transient/service outcomes fail rather than proving rejection. The hard cap remains 14 serial buffered requests.

The first paid attempt retained only its date, endpoint/model, `canonical-all-six` label, HTTP 400, absent object/status classes, no outputs, error category `other`, and absent code. The second paid attempt retained only its date, that the fieldless control ended after exactly 30.02 seconds under the prior 30-second request deadline with sanitized `dispatch failed`, and that zero configurable option cases were dispatched. Neither retained a private value, credential, body, header, ID, raw error, or generated content; neither cleared or rejected any field. All six configurable `x_search` candidates and every unobserved presence form remain blocked. Even after the corrected rerun, results may clear only exact request fields and presence forms actually observed; they cannot establish typed outputs/events, wider model compatibility, fallback, or direct-xAI support.

Request-only search support and the two inconclusive dedicated attempts do not clear any release gate. Protected hosted CI/environment evidence, owner-authorized public-generation and provider-evaluation contracts, reviewed configurable-option evidence, an owner-selected license, hosted metadata/provenance/permissions/publication review, hosting authorization, immutable tagging, direct-VCS and public-proxy imports, checksum evidence, final promotion, and publication all remain pending.


## v0 compatibility and migration

Releases begin at `v0.x`; no v1 stability is promised while evaluation remains experimental. Every exported breaking change, including during v0, requires a version increment, a changelog entry, updated examples/docs, and a release-note **Migration** section naming each changed API and exact caller action. A nonbreaking release still includes a Migration section stating that no action is required.

Evaluation callers retain `Evaluate` and `WithBaseURL`. Generation is additive and uses explicit Responses/Chat methods plus `WithPublicBaseURL`; there are no compatibility aliases or endpoint auto-detection. Earlier checkouts using the old module path must follow the [changelog migration](../CHANGELOG.md#migration).

## Immutable tags and defective releases

Never move, delete as a rollback, or reuse a published tag, including a failed prerelease candidate.

Correct a defective version with a new patch release. Add an appropriate `retract` directive in `go.mod` naming the actual bad version/range and rationale, mark the hosted release as affected, publish corrected notes, and direct consumers to the new version. Issue a security advisory for security-impacting defects. Keep the old tag intact; never add speculative retractions.
