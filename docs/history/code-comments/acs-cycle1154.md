# Comment history: `acs/cycle1154`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1154/predicates_test.go:3` — above `package cycle1154`

```text
// Package cycle1154 materialises the acceptance criteria for the single task
// triage committed to THIS cycle:
//
//   - router-drop-unknown-phase-entries → `router.ClampPlanToFloorWith`
//     (go/internal/router/floor.go:70) must DROP any PhasePlanEntry whose phase
//     is not in the plan's known-phase set (the same set `ValidatePlan` already
//     computes via `knownPhaseSet`, validate.go:84), recording each removal as a
//     `Clamp` with rule token "drop-unknown-phase" so the drop is never silent.
//
// Every other id in state.json's carryoverTodos was DEFERRED by triage and
// therefore carries ZERO predicates here — R9.3: predicates bind only to
// triage-committed work, and a predicate gating deferred work starves the
// committed task (the cycle-280 failure mode).
//
// Why this task exists: cycles 1151 and 1152 of this exact fleet lane
// (docs-floor-architecture-change-gate) both hard-failed at dispatch with
// "phase gate-wiring-proof: profile not found" — the advisor hallucinated a
// phase name out of policy prose, ValidatePlan flagged it "unknown-phase" but
// is PURE and REPORT-ONLY, and the one plan-mutating step (the floor clamp)
// only ever ADDS phases. The unknown entry therefore survived into dispatch and
// crashed the cycle.
//
// Predicate strategy — all six are BEHAVIORAL over the exported production
// function; none can be greened by adding a magic string:
//
//   - 001 is the crux: a run:true unknown entry must be gone from Entries.
//   - 002 is the non-silence half: exactly one Clamp per drop, carrying the
//     rule token and naming the dropped phase.
//   - 003 is the ANTI-NO-OP negative: an all-known plan must be byte-identical
//     to today's output with zero drop clamps. Without it, "drop everything" or
//     "drop every skipped entry" would green 001/002 while destroying the
//     floor's actual contract.
//   - 004 is the mint-awareness + passthrough edge: a phase minted IN THIS PLAN
//     is KNOWN (ValidatePlan's must-fix rule), and MintPhases itself is carried
//     through untouched — the clamp governs Entries only.
//   - 005 is the edge/OOD axis: run:false unknowns, the empty phase name, and
//     several unknowns at once — plus the PURITY contract (the caller's input
//     plan must be unmutated), which a naive in-place `slices.Delete` breaks.
//   - 006 is the end-to-end regression for the literal incident: a plan
//     carrying "gate-wiring-proof" alongside a real ship chain must come back
//     with the bogus phase gone AND the integrity floor still complete.
```

### `go/acs/cycle1154/predicates_test.go:62` — above `const hallucinated = "gate-wiring-proof"`

```text
// hallucinated is the exact phase name the advisor emitted in cycles 1151 and
// 1152. It is prose from docs/operations/operating-policy.md:22, never a phase:
// no registry entry, no .evolve/profiles/gate-wiring-proof.json.
```

### `go/acs/cycle1154/predicates_test.go:119` — above `func TestC1154_001_clamp_drops_unknown_run_true_entry(t *testing.T) {`

```text
// TestC1154_001_clamp_drops_unknown_run_true_entry is the crux predicate: an
// advisor-hallucinated phase scheduled to RUN must not survive the clamp. This
// is the exact shape that reached dispatch in cycles 1151/1152.
//
// Behavioral: calls the production ClampPlanToFloorWith and inspects the
// returned plan. RED today — the clamp only ever adds entries, never removes.
```

### `go/acs/cycle1154/predicates_test.go:148` — above `func TestC1154_002_drop_is_recorded_as_a_clamp(t *testing.T) {`

```text
// TestC1154_002_drop_is_recorded_as_a_clamp is the non-silence half. floor.go's
// documented pattern is "SKIP is not silent": every disposition the floor makes
// is visible in clamp telemetry. A drop that vanishes an advisor's phase with
// no record makes the next such incident undiagnosable from the run artifacts.
//
// Exactly ONE clamp per dropped phase — a duplicate would double-count in the
// telemetry the router already emits.
```

### `go/acs/cycle1154/predicates_test.go:300` — above `func TestC1154_006_gate_wiring_proof_regression(t *testing.T) {`

```text
// TestC1154_006_gate_wiring_proof_regression is the end-to-end regression for
// the literal incident. It reproduces the failing plan shape from cycles 1151
// and 1152 — a real ship-bound chain with the hallucinated phase spliced in —
// and asserts BOTH halves of correctness at once:
//
//  1. the bogus phase is gone (so dispatch never looks for its missing profile);
//  2. the integrity floor is still complete (tdd/build/audit/ship all run) — the
//     drop must not be implemented in a way that also strips the floor's own
//     forced entries.
```

### `go/acs/cycle1154/predicates_test.go:314` — above `pe(hallucinated, true),`

```text
// ← what crashed cycles 1151 and 1152
```
