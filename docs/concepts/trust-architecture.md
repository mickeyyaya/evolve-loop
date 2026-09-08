# Trust architecture

The Go host orchestrates phases and checks evidence. Model prose is a proposal; executable results, cycle identity, and audited tree bindings determine whether shipping is allowed.

## Evidence and authority

The host produces predicate results and binds their exact bytes and execution identity into audit evidence. Audit and Ship verify the same modern contract. Missing, malformed, wrong-cycle, substituted, or incomplete results require a new valid audit; historical records remain readable but are not permission to ship new work. A digest establishes identity only when its producer is trusted.

Audit also applies explanation, report, and repository checks. A passing predicate result alone does not authorize an arbitrary tree. Ship retains its independent audit-report ledger and tree bindings.

## Filesystem boundaries

Per-cycle worktrees separate source changes. Profiles describe supported write denials and separate read denials; the bridge must propagate those into the actual OS sandbox. macOS and Linux capabilities differ. Denied reads are not the same as read-only mounts. An environment marker claiming nesting does not prove an outer sandbox confines a child.

Required but unavailable confinement must fail explicitly. An operator sandbox opt-out is a degraded execution choice, not a security guarantee. See [isolation policy](../architecture/recovery-isolation-policy.md) for exact supported modes and migration consequences.

Read-only phase fences detect and restore worktree changes after a phase. They cannot undo a secret read, network side effect, or arbitrary external write. Tool hooks also depend on the CLI actually invoking them; do not describe every CLI tool path as identically guarded.

## Review and learning

The pipeline uses bounded repair and adversarial review. Different model families are a routing/setup preference, not a universal invariant under single-provider operation or failover. No combination of tests and reviews guarantees all bugs are detected.

See [the current runtime contract](../architecture/current-runtime-contract.md), [predicate evidence](../architecture/recovery-predicate-authority.md), and the [historical description](../private/research/archived-2026-09-09/concepts-trust-architecture.md).
