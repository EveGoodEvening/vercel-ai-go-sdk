# Release readiness and policy

## Current status

**Unreleased; do not create a candidate tag yet.** Release is blocked on:

- an owner-selected, committed license;
- hosted repository metadata, protected CI/environment, and publication/provenance review;
- separately authorized public-generation and provider-evaluation live results;
- owner authorization to tag and publish.

The protected public-generation and provider-evaluation [live evidence record](evaluation-live-evidence.md) is still **NOT RUN / PENDING LIVE RUN**. The configurable Gateway `x_search` evidence now includes the reviewed 14-call matrix and reviewed four-call interaction run from 2026-09-21. The interaction run recorded fieldless HTTP 200, allowed maximal five-field HTTP 200, safe HTTP 400 rejection for simultaneous non-empty allowed and excluded handle lists, and an execution failure for the excluded maximal five-field case; the test failed after all fixed cases were attempted. This supports the allowed maximal combination and local validation rejecting simultaneous non-empty allowed and excluded lists, while the excluded maximal combination remains pending. No semantic or typed-output claim follows. The focused two-call follow-up is **IMPLEMENTED / NOT RUN**, so configurable production work remains blocked. Local fixtures and workflow definitions likewise do not establish hosted success, and a matching local remote URL is not verification of the hosted repository.

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
- [ ] Verify the focused harness credential-free, then have the authorized orchestrator run exact selector `^TestGatewayXSearchOptionsExcludedInteractionContract$` once with the existing dedicated acknowledgement, both general acknowledgements absent, exactly one protected nonblank credential, and all five runtime-only private inputs. Do not rerun the completed matrix or the other interaction cases. The selector is capped at two serial calls: fieldless control, then the excluded maximal five-field case expected to return HTTP 200. It retains the same endpoint/model, 90-second per-request and 22-minute overall bounds, sanitation, and no retry/fallback. Independently review the sanitized result without claiming semantics or output typing.
- [ ] The owner has selected and committed `LICENSE`.
- [ ] The local remote matches `github.com/EveGoodEvening/vercel-ai-go-sdk`, and hosted name, visibility, default branch, protection rules, and release settings are independently verified.
- [ ] Release-environment protection, proxy visibility, artifact attestations/provenance, and publication permissions are reviewed. Ordinary CI stays read-only; do not grant it write or identity-token permissions.
- [ ] Retained evidence passes the [sanitization checklist](evaluation-live-evidence.md#sanitization-review), and the owner authorizes publication.

After these gates and authorization, follow the [continuation release sequence](../planning/IMPLEMENTATION_PLAN.md#chunk-24--continuation-release-verification): create the immutable candidate, verify direct-VCS and public-proxy imports, compile the fetched module, retain checksum/provenance evidence, and review before promotion/publication. None of those results is claimed here.

## Live workflow boundary

[`live-contract.yml`](../.github/workflows/live-contract.yml) remains the only CI workflow permitted to make the general paid Gateway calls. Its manual/scheduled `public-generation` and `provider-evaluation` jobs remain isolated and use the protected `live-evaluation` environment, pinned toolchain, API-key secret, exact job-specific acknowledgement, and absent opposite acknowledgement. Configurable `x_search` evidence is manual-only through one required `workflow_dispatch` choice, `x_search_probe`, with values `matrix`, `interaction`, and `excluded-interaction` and default `matrix`. The three values exact-select mutually exclusive manual-only jobs; scheduled events run none. This prevents the focused follow-up from repeating the completed matrix or resolved interaction cases.

The reviewed evidence supports request acceptance for the six exact singleton fields in their tested forms, the exact neutral combination of both empty lists with both explicit false booleans, and the allowed maximal five-field combination. It also supports local validation rejecting simultaneous non-empty `allowed_x_handles` and `excluded_x_handles` based on the safe handle-pair-only HTTP 400. The excluded maximal five-field combination remains pending because its case ended in an execution failure. The matrix's wrong-kind HTTP 500 responses establish neither attributable request validation nor an exclusive JSON type. No retained evidence establishes semantic efficacy, complete output schemas, citations/results, requiredness/nullability, or streaming events.

The next configurable-option action is credential-free verification of `^TestGatewayXSearchOptionsExcludedInteractionContract$`, followed by its separately authorized paid run and independent sanitation/evidence review. Its two-call maximum sends only the fieldless control and excluded maximal case, with HTTP 200 required for both. All other combinations, untested forms, semantics, typed outputs/events, wider model compatibility, fallback, and direct-xAI behavior remain blocked.

Request-only search support and the reviewed matrix do not clear any release gate. Protected hosted CI/environment evidence, owner-authorized public-generation and provider-evaluation contracts, reviewed interaction evidence, an owner-selected license, hosted metadata/provenance/permissions/publication review, hosting authorization, immutable tagging, direct-VCS and public-proxy imports, checksum evidence, final promotion, and publication all remain pending.


## v0 compatibility and migration

Releases begin at `v0.x`; no v1 stability is promised while evaluation remains experimental. Every exported breaking change, including during v0, requires a version increment, a changelog entry, updated examples/docs, and a release-note **Migration** section naming each changed API and exact caller action. A nonbreaking release still includes a Migration section stating that no action is required.

Evaluation callers retain `Evaluate` and `WithBaseURL`. Generation is additive and uses explicit Responses/Chat methods plus `WithPublicBaseURL`; there are no compatibility aliases or endpoint auto-detection. Earlier checkouts using the old module path must follow the [changelog migration](../CHANGELOG.md#migration).

## Immutable tags and defective releases

Never move, delete as a rollback, or reuse a published tag, including a failed prerelease candidate.

Correct a defective version with a new patch release. Add an appropriate `retract` directive in `go.mod` naming the actual bad version/range and rationale, mark the hosted release as affected, publish corrected notes, and direct consumers to the new version. Issue a security advisory for security-impacting defects. Keep the old tag intact; never add speculative retractions.
