# Build Explanation — Cycle 1684

## Build Binding
- Cycle: 1684
- Base SHA: 96d576a358e7f8eaf743156a4ae18ba7fd48b75d

## Summary
`evolve continuation release <scope-id>` now requires explicit operator
authority, refuses while the bound cycle's lane is still alive unless `-force`
is passed, and records who authorized every release, when, and why.

## Rationale
Releasing a continuation binding erases the lineage the defect-ledger gate reads
as anti-tamper evidence (ADR-0085/0089, cycle-1285), and the subcommand shipped
in cycle 1515 with no gate whatsoever — no flag, no environment fallback, no
liveness check, and a hardcoded reason string naming no caller. The phase guard
denies in-process `Agent`/`Task` dispatch during a cycle but says nothing about a
Bash invocation of the `evolve` binary, so the registry's ORCHESTRATOR-side-only
authority invariant had been widened by omission rather than by decision.

The authority check lives in the `continuation` package rather than in
`cmd/evolve` so that a future in-process caller cannot route around the gate the
CLI honours — a private copy inside the command is the drift that produced audit
cycle-1507's H2. The environment fallback is read by VALUE rather than presence,
because a gate written as `os.Getenv(...) != ""` accepts an exported-but-empty
or explicitly negative variable as consent.

Two over-correction risks were weighed and closed deliberately. A lease staler
than `runlease.DefaultTTL` does not block at all: heartbeat freshness is the only
liveness signal `runlease.Lease` documents, so treating a dead cycle's leftover
`.lease` as liveness would brick that scope permanently and make the gate a worse
stall than the gap it closes. An unreadable lease is loud but likewise
non-blocking, for the same reason.

The rejected alternative was recording the release into the `.evolve/ledger.jsonl`
hash chain instead of the item. `core.LedgerEntry` carries no actor field, so
that route could answer when and why but not who — the one question a lineage
erasure most needs to answer — and widening the chained entry schema is a far
larger blast radius than one field on the record that already existed.

## Changed Areas
- `go/internal/continuation/authority.go` — new: `RequireOperatorAuthority` and
  the `OperatorConfirmEnv` name, the single shared home of the gate and of the
  refusal text the CLI prints, so the flag and its guidance cannot drift apart.
- `go/internal/continuation/authority_test.go` — new: covers all four states of
  the gate, including the set-but-negative environment value that a
  presence-only check would misread as consent.
- `go/cmd/evolve/cmd_continuation.go` — the release subcommand gained
  `-operator` and `-force`, calls the shared helper before it reads the registry,
  refuses a binding whose cycle holds a fresh lease, and passes the authority and
  a force-distinguishable reason into the release transaction.
- `go/internal/inboxmover/continuation_retire.go` — the preserved record gained
  `released_by`, the WHO beside the WHEN and WHY it already carried.
- `go/internal/inboxmover/continuation_release.go` — `ReleaseContinuationBinding`
  now requires its caller to declare the authority the release is made under.
- `go/internal/inboxmover/continuation_resolve.go` — the scope read guard names
  itself as the releasing authority.
- `go/internal/inboxmover/continuation_release_test.go` — asserts the preserved
  record answers who and when, not only what.
- `go/cmd/evolve/cmd_inbox_consume.go` — names its authority; this path is not
  gated, since consumption is a documented transaction rather than a bare
  erasure.
- `go/internal/flagregistry/registry_table.go` — registers the environment
  variable as a consent token rather than an operator feature dial.
- `go/internal/flagregistry/registry_ceiling_test.go` — records the completeness
  bump this ceiling explicitly permits; the campaign's real ratchet is unchanged.
- `docs/architecture/control-flags.md` — regenerated from the registry, which is
  its source of truth.
- `docs/architecture/adr/0089-continuation-retirement-and-live-scope-guard.md` — amends the ADR whose deferred CLI this gate completes, so the
  decision record names the operator gate the shipped subcommand went without.

## Design Decisions
The gate is applied at the operator surface, not inside the release transaction.
The runtime's own release paths — the consumed-corpus reconciler and the scope
read guard — are orchestrator-side by construction and must keep releasing
without operator consent; gating the shared transaction would have broken them.
What every caller now shares is the obligation to NAME its authority, which is
the property the record needs.

`EVOLVE_OPERATOR_CONFIRM` is classified as an internal consent token rather than
an operator feature dial. It configures no behavior and has no consolidation
target: persisting "the operator agrees" into `policy.json` would permanently
disarm the gate. `-operator` is the primary surface and the environment variable
is its non-interactive spelling, following the `EVOLVE_LANE` precedent already in
the registry.

## Verification
The cycle's acceptance predicates pass in full, driving the real binary built
from this worktree rather than calling the gate directly: the ungated refusal
leaves the binding intact and writes no record, both authority paths release and
are evidenced, a set-but-negative environment value is refused, a live lease
refuses a fully authorized operator, `-force` overrides and is recorded
distinguishably, a stale lease does not block, and the helper refuses
unauthorized in-process callers. The package's own unit tests cover the gate's
four states directly.

## Compatibility
`released_by` is omitted when empty, so records written before the field existed
still parse and no reader requires it. The `-project-root` flag, the exit codes
for an unknown scope and a malformed invocation, and the preserve-then-delete
ordering are all unchanged.

## Limitations
Cycle 1515's `TestC1515_006` previously pinned that an ungated release exits 0 —
the contract this cycle supersedes. It was adjudicated and re-authored inside
this same diff by the TDD-engineer phase, which owns predicate authoring; it now
reaches the release path through `-operator`, and every assertion it made about
preserve-then-release and unrelated-sibling isolation is unchanged. Only the
precondition for reaching that behavior is new. The superseded negative contract
is now pinned explicitly, and far more strongly, by `go/acs/cycle1684`
`TestC1684_001`, which asserts the ungated call is non-zero, leaves the binding
intact, and writes no release record.

The live-lease refusal is enforced at the CLI only: an in-process caller that
reaches `ReleaseContinuationBinding` directly is gated on authority but not on
liveness.
