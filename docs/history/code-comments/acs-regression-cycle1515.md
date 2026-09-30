# Comment history: `acs/regression/cycle1515`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle1515/predicates_test.go:3` — above `package cycle1515`

```text
// Package cycle1515 materialises the cycle-1515 acceptance criteria for the
// fleet-scoped todo `park-consume-releases-continuation-binding`, whose triage
// split it into three top_n tasks:
//
//   - registry-release-on-park-consume — an item leaving the pending pool
//     (park/quarantine, ship-time consume) MUST release its scope-keyed
//     continuation-registry binding in the SAME operation, with the binding
//     VALUE preserved into the item file (`released_continuations[]`).
//   - planner-adoption-live-item-guard — the scope-keyed registry read
//     (`inboxmover.ResolveContinuationForScope`) MUST refuse a binding whose
//     scope id names no live pending item, and release the ghost.
//   - continuation-operator-cli — `evolve continuation list` /
//     `evolve continuation release <scope-id>` must exist so console never
//     hand-edits continuation-registry.json under its flock sidecar again.
//
// Standing state at RED time (verified live, not assumed from the filed item):
// the first two tasks ALREADY landed on this branch as cycle-1507's
// continuation work — `internal/inboxmover/continuation_retire.go`,
// `internal/phases/ship/consume.go:96-105`, and the guard at
// `internal/inboxmover/continuation_resolve.go:86`. Predicates 001-003 are
// therefore PRE-EXISTING GREEN and stand as regression pins: this cycle must
// not regress the wired lifecycle while adding the operator surface. Predicates
// 004-008 are the genuine RED — no `evolve continuation` subcommand exists
// (`ls go/cmd/evolve | grep -i continu` → no match).
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): every predicate
// here drives a REAL production entry point — the exported inboxmover /
// continuation functions, or the `evolve` binary built from THIS worktree in
// TestMain — and asserts on its return value, exit code, stderr, or the
// resulting on-disk bytes. There is no load-bearing source-text assertion in
// this file.
//
// The CLI predicates run the binary with an explicit cmd.Dir set to the fixture
// project root and assert through the process boundary, so they are a
// REACHABILITY proof of the dispatcher wiring (registry.go's commands table),
// not a direct call into an unreachable handler.
//
// Reliability (flaky-predicate-shape rules): no `/...` sweep, no multi-package
// `go test`, no known 40s+ suite, no wall-clock deadline, no literal PID; every
// subprocess carries an explicit cmd.Dir and never inherits process cwd.
```

### `go/acs/regression/cycle1515/predicates_test.go:59` — above `const scopeID = "context-fill-telemetry-and-cap"`

```text
// scopeID is the parked/consumed scope under test — the shape of the live burn
// (cycle-1487 re-dispatched this exact id from an immortal binding).
```

### `go/acs/regression/cycle1515/predicates_test.go:327` — above `func TestC1515_003_ScopeResolveRefusesRetiredBinding(t *testing.T) {`

```text
// TestC1515_003_ScopeResolveRefusesRetiredBinding drives the ONE seam the wave
// planner's lane minting and the post-triage adoption path both go through and
// asserts the cycle-1487 shape is refused: item parked in quarantine/, binding
// still live ⇒ nil return, WARN on stderr, ghost binding released. The live
// sibling in the same call must still resolve — a guard that refuses
// everything would trade re-dispatch for salvage loss.
```

### `go/acs/regression/cycle1515/predicates_test.go:366` — above `func TestC1515_004_ContinuationListShowsBindings(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// Task: continuation-operator-cli  (the genuine RED for cycle 1515)
// ---------------------------------------------------------------------------
```

### `go/acs/regression/cycle1515/predicates_test.go:408` — above `func TestC1515_006_ContinuationReleaseReleasesAndAnnotates(t *testing.T) {`

```text
// TestC1515_006_ContinuationReleaseReleasesAndAnnotates drives `evolve
// continuation release <scope-id>`: the binding must go, the unrelated live
// sibling's binding must survive, and the released VALUE must be preserved into
// the scope's item file — the same preserve-then-release contract predicate 001
// pins for the park path, reached from the operator surface rather than
// duplicated inside it.
//
// The -operator flag is cycle 1684's authority precondition, not a relaxation
// of this predicate. Release shipped in cycle 1515 with no gate at all, so this
// predicate's ungated invocation incidentally pinned "ungated release exits 0"
// — a contract cycle 1684 was commissioned to supersede, because dropping a
// binding erases the lineage the defect-ledger gate reads as anti-tamper
// evidence (ADR-0085/0089). That negative contract is now pinned explicitly and
// far more strongly by go/acs/cycle1684 TestC1684_001, which asserts the
// ungated call is non-zero, leaves the binding intact, names both authority
// paths, and writes no release record. What THIS predicate pins is unchanged:
// preserve-then-release, and unrelated-sibling isolation. Only the precondition
// for reaching that behavior is new.
```

### `go/acs/regression/cycle1515/predicates_test.go:473` — above `func TestC1515_008_ContinuationRejectsMalformedInvocations(t *testing.T) {`

```text
// TestC1515_008_ContinuationRejectsMalformedInvocations covers the remaining
// malformed-input edges through the real dispatcher: a bare `continuation`, an
// unknown subcommand, and `release` with no scope argument must each fail
// loudly rather than defaulting to some destructive interpretation.
//
// The registration precondition is load-bearing: without it this predicate is
// vacuously GREEN on a repo that has no `continuation` command at all (every
// invocation fails as an unknown command), which is exactly the no-op-passable
// shape the cycle-85 ban exists to catch.
```
