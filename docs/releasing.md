# Release readiness and policy

## Current status

**Unreleased; do not create a candidate tag yet.** Release is blocked on:

- an owner-selected, committed license;
- hosted repository metadata, protected CI/environment, and publication/provenance review;
- separately authorized public-generation and provider-evaluation live results;
- owner authorization to tag and publish.

The protected public-generation and provider-evaluation [live evidence record](evaluation-live-evidence.md) is still **NOT RUN / PENDING LIVE RUN**. The separately authorized third configurable Gateway `x_search` matrix run completed 14 serial calls on 2026-09-21 and passed independent structural adjudication: each of the six fields was request-accepted in its tested singleton form, and the exact combination of both empty handle lists with both booleans false was accepted. The canonical all-six HTTP 400 remains ambiguous, while the three wrong-kind HTTP 500 responses are inconclusive and establish no validation boundary. No semantic or typed-output claim follows. The narrow four-call interaction probe is **IMPLEMENTED / NOT RUN**, so configurable production work remains blocked. Local fixtures and workflow definitions likewise do not establish hosted success, and a matching local remote URL is not verification of the hosted repository.

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
- [ ] Separately, verify the interaction harness credential-free, then have the authorized orchestrator run exact selector `^TestGatewayXSearchOptionsInteractionContract$` once with the existing dedicated acknowledgement, both general acknowledgements absent, exactly one protected nonblank credential, and all five runtime-only private inputs. Do not rerun the completed 14-case matrix. The interaction selector is capped at four serial calls: fieldless control; each maximal five-field combination omitting one handle list; and the non-empty handle pair with dates/booleans omitted. It retains the same endpoint/model, 90-second per-request and 22-minute overall bounds, sanitation, and no retry/fallback. Both five-field cases must return HTTP 200; the handle pair may establish rejection only on safe ambiguous HTTP 400/422 with a structurally present non-excluded error. Independently review the sanitized result without claiming semantics or output typing. The shared fixed-list execution/accounting contract is: after the control succeeds, all three non-control cases execute exactly once despite earlier non-control failures; structural records are logged where available, sanitized errors are accumulated, one final failure is emitted only after the fixed list when failures remain, no case is replayed, and the hard cap is four calls.
- [ ] The owner has selected and committed `LICENSE`.
- [ ] The local remote matches `github.com/EveGoodEvening/vercel-ai-go-sdk`, and hosted name, visibility, default branch, protection rules, and release settings are independently verified.
- [ ] Release-environment protection, proxy visibility, artifact attestations/provenance, and publication permissions are reviewed. Ordinary CI stays read-only; do not grant it write or identity-token permissions.
- [ ] Retained evidence passes the [sanitization checklist](evaluation-live-evidence.md#sanitization-review), and the owner authorizes publication.

After these gates and authorization, follow the [continuation release sequence](../planning/IMPLEMENTATION_PLAN.md#chunk-24--continuation-release-verification): create the immutable candidate, verify direct-VCS and public-proxy imports, compile the fetched module, retain checksum/provenance evidence, and review before promotion/publication. None of those results is claimed here.

## Live workflow boundary

[`live-contract.yml`](../.github/workflows/live-contract.yml) remains the only CI workflow permitted to make the general paid Gateway calls. Its manual/scheduled `public-generation` and `provider-evaluation` jobs remain isolated and use the protected `live-evaluation` environment, pinned toolchain, API-key secret, exact job-specific acknowledgement, and absent opposite acknowledgement. Configurable `x_search` evidence remains a separate manual-only gate that cannot run on the weekly schedule or clear either general job. The completed 14-call matrix and the not-yet-run interaction selector are separate exact workflow steps so an interaction run cannot accidentally repeat the matrix.

The reviewed third matrix run establishes request acceptance only for the six exact singleton fields in their tested forms and the exact neutral combination of both empty lists with both explicit false booleans. The all-six HTTP 400 does not attribute any individual field, family, value, or interaction. The wrong-kind HTTP 500 responses establish neither attributable request validation nor an exclusive JSON type. Structural `message`, `reasoning`, and `x_search_call` observations do not establish semantic efficacy, complete output schemas, citations/results, requiredness/nullability, or streaming events.

The next configurable-option action is credential-free verification of `^TestGatewayXSearchOptionsInteractionContract$`, followed by its separately authorized paid run and independent sanitation/evidence review. Its four-call maximum isolates whether each maximal combination excluding one handle list succeeds and whether the non-empty handle lists coexist. The probe fails closed on any result outside its narrow evidence rules. The shared fixed-list execution/accounting contract is: after the control succeeds, all three non-control cases execute exactly once despite earlier non-control failures; structural records are logged where available, sanitized errors are accumulated, one final failure is emitted only after the fixed list when failures remain, no case is replayed, and the hard cap is four calls. All other combinations, untested forms, semantics, typed outputs/events, wider model compatibility, fallback, and direct-xAI behavior remain blocked.

Request-only search support and the reviewed matrix do not clear any release gate. Protected hosted CI/environment evidence, owner-authorized public-generation and provider-evaluation contracts, reviewed interaction evidence, an owner-selected license, hosted metadata/provenance/permissions/publication review, hosting authorization, immutable tagging, direct-VCS and public-proxy imports, checksum evidence, final promotion, and publication all remain pending.


## v0 compatibility and migration

Releases begin at `v0.x`; no v1 stability is promised while evaluation remains experimental. Every exported breaking change, including during v0, requires a version increment, a changelog entry, updated examples/docs, and a release-note **Migration** section naming each changed API and exact caller action. A nonbreaking release still includes a Migration section stating that no action is required.

Evaluation callers retain `Evaluate` and `WithBaseURL`. Generation is additive and uses explicit Responses/Chat methods plus `WithPublicBaseURL`; there are no compatibility aliases or endpoint auto-detection. Earlier checkouts using the old module path must follow the [changelog migration](../CHANGELOG.md#migration).

## Immutable tags and defective releases

Never move, delete as a rollback, or reuse a published tag, including a failed prerelease candidate.

Correct a defective version with a new patch release. Add an appropriate `retract` directive in `go.mod` naming the actual bad version/range and rationale, mark the hosted release as affected, publish corrected notes, and direct consumers to the new version. Issue a security advisory for security-impacting defects. Keep the old tag intact; never add speculative retractions.
