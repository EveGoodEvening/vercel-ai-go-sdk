# Contributing

## Dependency policy

Prefer the Go standard library. A new direct or transitive dependency is accepted only when the change documents why the standard library and existing code are insufficient, identifies the exact version and purpose, and records a license review before merge. Dependency upgrades require the same compatibility and license review; convenience alone is not sufficient rationale.

Keep evaluation behavior hermetic by default. Ordinary tests must not depend on credentials, paid services, DNS, or non-loopback network access. Live service checks belong only in the separately gated live-contract workflow.
