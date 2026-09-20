# Release readiness and policy

## Current pre-tag status

No candidate tag may be created yet. The following external or authorized prerequisites remain unresolved:

- **Owner-selected license:** blocked; repository ownership has not selected a license, so no `LICENSE` is present. Do not infer or add one.
- **Remote and repository metadata:** blocked; no remote or hosting-repository metadata has been configured. Do not invent a remote.
- **Authorized paid live contract:** blocked; the sanitized evidence record remains `NOT RUN` and requires the protected live workflow or the exact authorized local command.
- **Publication and provenance configuration:** blocked pending the hosting repository, its release environment/protection policy, and owner decisions. Review artifact attestations and any package-publication permissions after the remote exists; do not grant write or identity-token permissions to ordinary CI.

The pinned CI and local-consumer checks are configured but their results are not recorded here as passes. Run and record every pre-tag gate from a clean checkout before proceeding to Chunk 12. This chunk must not create, move, delete, push, or publish a tag or release.

## Pre-tag gate

Before creating a prerelease tag, an operator must confirm all of the following:

- [ ] Ordinary CI passes on exactly Go 1.26.8 and Go 1.27.1 with `AI_GATEWAY_API_KEY`, `VERCEL_OIDC_TOKEN`, and `AI_GATEWAY_LIVE_COST_ACK` explicitly unset. The test-wide transport guard rejects every non-loopback destination while loopback fixtures remain usable.
- [ ] Go 1.27.1 passes the race detector and `go vet`; both pinned versions pass the ordinary suite and example compile check.
- [ ] The protected live workflow passes with exactly one authorized credential and the workflow-defined `AI_GATEWAY_LIVE_COST_ACK=I_ACCEPT_LIVE_EVALUATION_COSTS`. The acknowledgement is never an input or repository variable.
- [ ] `./scripts/verify-local-consumer.sh` passes from a clean local checkout. It creates a temporary module outside the repository, uses a local `replace`, disables proxy access, compiles all supported request/result/metadata/retry/error use, and rejects `ConfigError`, exported retry hooks, and retry-hook aliases.
- [ ] Exported API review matches the supported surface and contains exactly the five documented error types.
- [ ] `CHANGELOG.md` and draft release notes describe supported behavior, non-goals, experimental risk, x_search status, and any v0 migration steps.
- [ ] The repository owner has selected and committed `LICENSE`.
- [ ] The remote exists and its repository metadata matches the module path.
- [ ] Publication provenance, release-environment protection, and least-privilege workflow permissions have been reviewed against the configured host.
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

`.github/workflows/ci.yml` is the ordinary path. It has read-only repository permission, explicitly scrubs all credential/cost variables for every command, and relies on the mandatory test-wide loopback-only transport guard. It must never receive service credentials or permission to contact non-loopback services.

`.github/workflows/live-contract.yml` is the sole CI path permitted to contact a non-loopback service. It is manual/scheduled only, has no pull-request trigger, reads one API key from the protected `live-evaluation` environment, fixes the cost acknowledgement in workflow source, rejects a second credential, and runs only the exact live contract test. Forked pull requests cannot invoke this workflow or receive its environment secret.

Updating either Go pin requires a reviewed plan and evidence update. Floating selectors such as `stable`, `oldstable`, `1.26.x`, and `1.27.x` are prohibited.
