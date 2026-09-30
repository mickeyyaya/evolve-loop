# Comment history: `acs/cycle1559`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1559/predicates_test.go:3` — above `package cycle1559`

```text
// Package cycle1559 continues the red-first-deliverable-reds-main lane
// (live inbox record; same inbox item as cycle-1555's `ship-added-test-*`
// tasks). Cycle-1555 authored the crux detection (T1 AC1/AC2) and the
// production-caller wiring proof plus the skip convention (T2 AC1/AC2); they
// are RED in the tree today (the green implementation never landed) and stay
// pinned here as pre-existing coverage — this cycle does not re-author them.
//
// The one remaining materialized gap in this lane's fleet-scoped task
// (`ship-added-red-test-guard`, triage top_n) is AC3: a `require-tmux`-style,
// environment-exclusive added test must never be silently claimed green (a
// lone-file package excluded by its own `//go:build` constraint would
// otherwise either false-RED as a build failure or vanish with no record) —
// the gate must instead leave an explicit, durable backstop record naming
// the file and its exclusion reason. AC4 (a transient scanner failure stays
// distinct from a genuine RED with the existing one-retry behavior) already
// has structural coverage: the added-test candidates flow through the SAME
// `runRepoContractGate` retry/classification path the pre-existing
// `TestRepoContractGate_TransientFailureRetriesOnceThenShips` /
// `TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns`
// pin — no scope-specific retry logic to test separately.
//
// Predicate strategy — behavioral, never source-grep (cycle-85 ban): each
// predicate DRIVES the system by running the named Go unit test in ONE
// named package with `-run` narrowing and requiring its `--- PASS:` line
// (per the flaky-predicate-shape rules — no `./...` sweeps, no wall-clock
// bounds, cmd.Dir always set explicitly). The PASS-line assertion is the
// anti-vacuous guard: a non-matching `-run` pattern exits 0 with "no tests
// to run", so exit-code-only checking would pass on an EMPTY repo.
//
// Predicate map:
//
//	001 — pre-existing: added failing test blocks ship (T1 AC1)
//	002 — pre-existing: selection bounded to newly-added test files (T1 AC2)
//	003 — pre-existing: the fixed four-package gate behaviours are unweakened
//	004 — pre-existing WIRING PROOF: Phase.runNative stops before git/ship (T2 AC1)
//	005 — pre-existing: a t.Skip-ped newly added test does not block (T2 AC2)
//	006 — NEW this cycle: an env-exclusive (`requires_tmux`) added test gets
//	      an explicit backstop record, never a silent false green (T1 AC3)
```

### `go/acs/cycle1559/predicates_test.go:92` — above `func TestC1559_001_AddedFailingTestBlocksShip(t *testing.T) {`

```text
// TestC1559_001_AddedFailingTestBlocksShip — T1 AC1 (pre-existing, cycle-1555).
```

### `go/acs/cycle1559/predicates_test.go:124` — above `func TestC1559_006_EnvExclusiveCandidateGetsExplicitBackstop(t *testing.T) {`

```text
// TestC1559_006_EnvExclusiveCandidateGetsExplicitBackstop — T1 AC3, the
// cycle-1559 addition. A `requires_tmux`-tagged added test must not block
// the ship (it is honestly unrunnable here) AND must not be silently
// dropped — the scan log must explicitly name it and its exclusion reason.
```
