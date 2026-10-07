# Trust architecture

The Go host orchestrates phases and checks evidence. Model prose is a proposal. Executable results, cycle identity and audited tree bindings determine if a ship is allowed.

## Evidence and authority

The host produces predicate results. It binds their exact bytes and execution identity into audit evidence. Audit and Ship verify the same modern contract. Results that are absent, malformed, from the wrong cycle, substituted or incomplete require a new valid audit. Historical records stay readable, but they do not give permission to ship new work. A digest establishes identity only when its producer is trusted.

Audit also applies explanation, report and repository checks. A predicate result that passes does not alone authorize an arbitrary tree. Ship keeps its independent audit-report ledger and tree bindings.

## Filesystem boundaries

Per-cycle worktrees separate source changes. Profiles describe the supported write denials and, separately, the read denials. The bridge must propagate those into the actual OS sandbox. macOS and Linux capabilities differ. Denied reads are not the same as read-only mounts. An environment marker that says the process is nested does not prove that an outer sandbox confines a child.

If confinement is required but not available, it must fail explicitly. An operator sandbox opt-out is a degraded execution choice, not a security guarantee. See [isolation policy](../architecture/recovery-isolation-policy.md) for the exact supported modes and the migration consequences.

Read-only phase fences detect and restore worktree changes after a phase. They cannot undo a secret read, a network side effect or an arbitrary external write. Tool hooks also work only when the CLI actually invokes them. Do not describe every CLI tool path as identically guarded.

## Review and learning

The pipeline uses bounded repair and adversarial review. Different model families are a routing/setup preference. They are not a universal invariant under single-provider operation or failover. No combination of tests and reviews guarantees to detect all bugs.

See [the current runtime contract](../architecture/current-runtime-contract.md), [predicate evidence](../architecture/recovery-predicate-authority.md), and the [historical description](../private/research/archived-2026-09-09/concepts-trust-architecture.md).
