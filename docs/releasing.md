# Release readiness and policy

## Current status

**Unreleased; do not create a candidate tag yet.** Release is blocked on:

- an owner-selected, committed license;
- hosted repository metadata, protected CI/environment, and publication/provenance review;
- separately authorized public-generation and provider-evaluation live results;
- owner authorization to tag and publish.

The protected public-generation and provider-evaluation [live evidence record](evaluation-live-evidence.md) is still **NOT RUN / PENDING LIVE RUN**. The dedicated configurable Gateway `x_search` evidence harness is separately **IMPLEMENTED / NOT RUN**: its owner authorization and bounded-cost acceptance are recorded, but it has produced no result and clears no option field. Narrow Responses search probes establish only exact fieldless request declarations and observed structural discriminators; they are not hosted contract results. Local fixtures and workflow definitions likewise do not establish hosted success, and a matching local remote URL is not verification of the hosted repository.

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
- [ ] Separately, verify `^TestGatewayXSearchOptionsPrerequisiteMismatchesZeroDispatch$` with credentials and acknowledgements absent, including each missing or blank private input on an otherwise authorized configuration, then have the authorized orchestrator run `^TestGatewayXSearchOptionsContract$` once with `AI_GATEWAY_X_SEARCH_LIVE_COST_ACK=I_ACCEPT_LIVE_X_SEARCH_COSTS`, both general acknowledgements absent, exactly one protected nonblank credential, and all five nonblank runtime-only private inputs. Those inputs are injected only from protected GitHub secrets named exactly `AI_GATEWAY_X_SEARCH_PROBE_INPUT`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_A`, `AI_GATEWAY_X_SEARCH_PROBE_HANDLE_B`, `AI_GATEWAY_X_SEARCH_PROBE_FROM_DATE`, and `AI_GATEWAY_X_SEARCH_PROBE_TO_DATE`; values must never be supplied as dispatch inputs, printed, or retained. This dedicated serial, buffered, bounded, no-retry/no-fallback gate targets only `POST https://ai-gateway.vercel.sh/v1/responses` with `spacexai/grok-4.6`; it does not substitute for either general live job.
- [ ] The owner has selected and committed `LICENSE`.
- [ ] The local remote matches `github.com/EveGoodEvening/vercel-ai-go-sdk`, and hosted name, visibility, default branch, protection rules, and release settings are independently verified.
- [ ] Release-environment protection, proxy visibility, artifact attestations/provenance, and publication permissions are reviewed. Ordinary CI stays read-only; do not grant it write or identity-token permissions.
- [ ] Retained evidence passes the [sanitization checklist](evaluation-live-evidence.md#sanitization-review), and the owner authorizes publication.

After these gates and authorization, follow the [continuation release sequence](../planning/IMPLEMENTATION_PLAN.md#chunk-24--continuation-release-verification): create the immutable candidate, verify direct-VCS and public-proxy imports, compile the fetched module, retain checksum/provenance evidence, and review before promotion/publication. None of those results is claimed here.

## Live workflow boundary

[`live-contract.yml`](../.github/workflows/live-contract.yml) remains the only CI workflow permitted to make the general paid Gateway calls. Its manual/scheduled `public-generation` and `provider-evaluation` jobs remain isolated and use the protected `live-evaluation` environment, pinned toolchain, API-key secret, exact job-specific acknowledgement, and absent opposite acknowledgement. The configurable `x_search` Gate A harness is a separate authorized selector and is not yet a completed workflow result. Its workflow job is conditional on `github.event_name == 'workflow_dispatch'`, so the weekly schedule cannot dispatch it. It additionally requires the exact dedicated acknowledgement, absence of both general acknowledgements, exactly one API-key or OIDC credential, all five nonblank runtime-only private inputs from same-named protected secrets, and its own fail-closed zero-dispatch prerequisite verification before HTTP client or request construction. Environment protection and authorization still require hosted review.

Chat server-search request encoding is covered hermetically; optional live corroboration has not run and is not part of that feature's implementation acceptance. Responses built-in search remains request-only for exact `{"type":"web_search","search_context_size":"low"}` and fieldless `{"type":"x_search"}` declarations. The existing structural probes corroborate only those declarations and structural discriminators. The configurable-options harness uses a static eight-case matrix: fieldless control, combined all-six presence, explicit empty lists and false booleans, from-only, to-only, and grouped wrong handle-list/date/boolean types. Only a failed combined canonical case may add six one-field valid diagnostics; the hard cap is 14 serial buffered requests. A wrong-type case counts as an attributable validation rejection only for HTTP 400 or 422; 401/403/408/409/429, 5xx, transport failures, and unrelated error categories fail the case. Its sanitized output may retain only case label, structural option class set/count, HTTP status, safe top-level object/status, safe output discriminator/status sets, and safe error category/code. Missing/unknown raw values map to `absent`/`other`; private input values, bodies, headers, IDs, generated content, provider metadata, usage, and raw errors are excluded.

Because that harness is **NOT RUN**, all six configurable `x_search` candidates and every unobserved presence form remain blocked. Even after execution, results may clear only exact request fields and presence forms actually observed; they cannot establish typed outputs/events, wider model compatibility, fallback, or direct-xAI support.

Request-only search support and the unexecuted dedicated harness do not clear any release gate. Protected hosted CI/environment evidence, owner-authorized public-generation and provider-evaluation contracts, reviewed configurable-option evidence, an owner-selected license, hosted metadata/provenance/permissions/publication review, hosting authorization, immutable tagging, direct-VCS and public-proxy imports, checksum evidence, final promotion, and publication all remain pending.


## v0 compatibility and migration

Releases begin at `v0.x`; no v1 stability is promised while evaluation remains experimental. Every exported breaking change, including during v0, requires a version increment, a changelog entry, updated examples/docs, and a release-note **Migration** section naming each changed API and exact caller action. A nonbreaking release still includes a Migration section stating that no action is required.

Evaluation callers retain `Evaluate` and `WithBaseURL`. Generation is additive and uses explicit Responses/Chat methods plus `WithPublicBaseURL`; there are no compatibility aliases or endpoint auto-detection. Earlier checkouts using the old module path must follow the [changelog migration](../CHANGELOG.md#migration).

## Immutable tags and defective releases

Never move, delete as a rollback, or reuse a published tag, including a failed prerelease candidate.

Correct a defective version with a new patch release. Add an appropriate `retract` directive in `go.mod` naming the actual bad version/range and rationale, mark the hosted release as affected, publish corrected notes, and direct consumers to the new version. Issue a security advisory for security-impacting defects. Keep the old tag intact; never add speculative retractions.
