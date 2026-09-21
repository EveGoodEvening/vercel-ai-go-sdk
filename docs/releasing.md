# Release readiness and policy

## Current pre-tag status

No candidate tag may be created yet. Local `origin` is already `git@github.com:EveGoodEvening/vercel-ai-go-sdk.git`, matching the module path, so local remote configuration is not a blocker. The following external, hosted, or authorized prerequisites remain unresolved:

- **Owner-selected license:** blocked; repository ownership has not selected a license, so no `LICENSE` is present. Do not infer or add one.
- **Hosted repository metadata:** unverified; confirm that the repository hosted at the configured `origin` has the expected name, visibility, default branch, release settings, and other metadata before release. Do not treat the matching local URL as hosted verification.
- **Authorized paid live contracts:** blocked; the sanitized evidence record remains **NOT RUN / PENDING LIVE RUN**. Authorized, isolated runs must cover all four public generation paths and provider Evaluation.
- **Publication and provenance configuration:** blocked pending verification of the hosted repository, its release environment/protection policy, proxy visibility, and owner decisions. Review artifact attestations and any package-publication permissions; do not grant write or identity-token permissions to ordinary CI.

The pinned CI and local-consumer checks are configured but their results are not recorded here as passes. Run and record every pre-tag gate from a clean checkout before proceeding to release. This checklist must not create, move, delete, push, or publish a tag or release.

## Pre-tag gate

Before creating a prerelease tag, an operator must confirm all of the following:

- [ ] Ordinary CI passes on exactly Go 1.26.8 and Go 1.27.1 with both credential variables (`AI_GATEWAY_API_KEY` and `VERCEL_OIDC_TOKEN`) and both acknowledgement variables (`AI_GATEWAY_PUBLIC_LIVE_COST_ACK` and `AI_GATEWAY_LIVE_COST_ACK`) explicitly unset. The test-wide transport guard rejects every non-loopback destination while loopback fixtures remain usable.
- [ ] Go 1.27.1 passes the race detector and `go vet`; both pinned versions pass the ordinary suite and example compile check.
- [ ] The protected `public-generation` live job passes with exactly one authorized credential path and `AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS`, while `AI_GATEWAY_LIVE_COST_ACK` is unset. The configured workflow injects `AI_GATEWAY_API_KEY` and rejects a nonblank `VERCEL_OIDC_TOKEN`; any separately authorized local run must likewise select exactly one of those credential variables.
- [ ] The isolated public-generation run produces sanitized evidence for all four paths: buffered Responses (`CreateResponse`), streaming Responses (`StreamResponse`), buffered Chat Completions (`CreateChatCompletion`), and streaming Chat Completions (`StreamChatCompletion`).
- [ ] The protected `provider-evaluation` live job passes with exactly one authorized credential path and `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS`, while `AI_GATEWAY_PUBLIC_LIVE_COST_ACK` is unset. The configured workflow injects `AI_GATEWAY_API_KEY` and rejects a nonblank `VERCEL_OIDC_TOKEN`; any separately authorized local run must likewise select exactly one of those credential variables.
- [ ] The isolated provider-evaluation run produces sanitized evidence for `Evaluate`; generation evidence and evaluation evidence are not interchangeable.
- [ ] The exported continuation API, including Responses `PreviousResponseID` and any documented caller-managed Chat history, is audited against current first-party contract evidence and accurately described without inferring hosted acceptance from hermetic fixtures.
- [ ] `./scripts/verify-local-consumer.sh` passes from a clean local checkout. It creates a temporary module outside the repository, uses a local `replace`, disables proxy access, compiles all supported request/result/metadata/retry/error use, and rejects `ConfigError`, exported retry hooks, and retry-hook aliases.
- [ ] Exported API review matches the supported surface and contains exactly the five documented error types.
- [ ] `CHANGELOG.md` and draft release notes describe supported behavior, non-goals, experimental risk, x_search status, and any v0 migration steps.
- [ ] The repository owner has selected and committed `LICENSE`.
- [ ] The local remote exists and matches the module path, and the hosted repository metadata has been independently verified.
- [ ] Publication provenance, release-environment protection, proxy visibility, and least-privilege workflow permissions have been reviewed against the configured host.
- [ ] Sanitized evidence contains no credentials, authorization headers, raw diagnostic bodies, raw live payloads, provider-metadata values, or cost-bearing answer content.

A checked box must point to evidence produced after the release commit. A workflow definition or local script is not itself a passing result.

## v0 compatibility and migration policy

Releases begin at `v0.x`. Evaluation remains experimental, and v1 compatibility is not promised while the upstream evaluation contract is unstable.

Every exported breaking change, including one made during v0, requires all of the following:

1. an appropriate semantic version increment;
2. a changelog entry naming every removed or changed API;
3. a release-note **Migration** section naming those APIs and the exact caller actions needed; and
4. updated examples and reference documentation in the same release.

A release with no breaking change still includes a Migration section stating that no caller action is required.

## Immutable tags and defective releases

Published tags are immutable. Never move, delete as a rollback, or reuse a published tag, including a failed prerelease candidate.

Correct a defective published version by issuing a new patch version. The correction must add an appropriate `retract` directive to `go.mod` for the bad version or range with a concise rationale, mark the hosting release as affected rather than rewriting history, and publish corrected release notes. Issue a security advisory when the defect has security impact. Consumers must be directed to the new version; the old tag remains intact.

Do not add a speculative `retract` directive before a real defective version exists. Its version/range and rationale must identify the actual immutable bad release.

## Workflow boundary

`.github/workflows/ci.yml` is the ordinary path. It has read-only repository permission, explicitly scrubs both credential variables (`AI_GATEWAY_API_KEY` and `VERCEL_OIDC_TOKEN`) and both acknowledgement variables (`AI_GATEWAY_PUBLIC_LIVE_COST_ACK` and `AI_GATEWAY_LIVE_COST_ACK`) for every command, and relies on the mandatory test-wide loopback-only transport guard. It must never receive service credentials or permission to contact non-loopback services.

`.github/workflows/live-contract.yml` is the sole CI path permitted to contact a non-loopback service. It is manual/scheduled only, has no pull-request trigger, and contains two isolated jobs. `public-generation` fixes `AI_GATEWAY_PUBLIC_LIVE_COST_ACK=I_ACCEPT_LIVE_PUBLIC_API_COSTS`, requires the opposite acknowledgement to be unset, and exercises `CreateResponse`, `StreamResponse`, `CreateChatCompletion`, and `StreamChatCompletion`. `provider-evaluation` fixes `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS`, requires the opposite acknowledgement to be unset, and exercises `Evaluate`. Each configured job reads `AI_GATEWAY_API_KEY` from the protected `live-evaluation` environment and rejects a nonblank `VERCEL_OIDC_TOKEN`, enforcing one credential path; the client also recognizes both credential variables, so an authorized local execution must select exactly one. Neither acknowledgement is a workflow input or repository variable. Forked pull requests cannot invoke this workflow or receive its environment secret. The workflow definition remains configuration only: live execution is explicitly **NOT RUN / PENDING LIVE RUN** until an authorized run produces reviewed, sanitized evidence.

Updating either Go pin requires a reviewed plan and evidence update. Floating selectors such as `stable`, `oldstable`, `1.26.x`, and `1.27.x` are prohibited.
