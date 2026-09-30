# Comment history: `acs/cycle1693`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1693/predicates_test.go:3` — above `package cycle1693`

```text
// Package cycle1693 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `iteration-state-coherence-sentinel`
// (.evolve/inbox/processing/cycle-1693/2026-07-22T14-02-00Z-iteration-state-coherence-sentinel.json).
//
// THE DEFECT. unfinishedCycle (go/cmd/evolve/cmd_loop_control.go) is the only
// guard that reads canonical cycle-state as possibly stale, and it runs once
// per batch in prepareFreshBatch. loopBatchCoordinator.prepareIteration
// (go/cmd/evolve/cmd_loop_window.go) — the chokepoint cmd_loop_batch.go runs
// before EVERY dispatch, sequential and fleet — never reads it. A fleet lane
// SIGKILLed by cmd_fleet.go's WaitDelay escalation skips its abnormalEpilogue
// state floor, so the canonical record keeps claiming a live phase for a dead
// cycle the batch already passed (CycleID <= lastCycleNumber — invisible to
// unfinishedCycle).
//
// WHY THESE PREDICATES SHELL A FROZEN IN-PACKAGE SUITE. prepareIteration is an
// unexported method of package main: no external package can call it, and a
// predicate that built the binary could not seed a SIGKILLed lane's state
// between two iterations. The behavioral contract therefore lives in the
// frozen, TDD-authored file
// go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go, which drives the
// REAL chokepoint (and the production entrypoint runLoop) against seeded
// canonical state. Each predicate below runs that suite (ONE named package,
// -run narrowed to the exact frozen names, -race per AC2) and requires the
// exact `--- PASS: <name> (` marker for every test and subtest it binds — a
// renamed, deleted or skipped test cannot pass vacuously, because
// `go test -run` matching nothing still exits 0.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - POSITIVE : 001 (stale record → WARN + aborted, three residue shapes).
//   - NEGATIVE : 002 (resumable / live-owner / completed / terminal records
//     byte-identical, no write, no WARN) — the over-correction guards, GREEN on
//     the unpatched tree by design and required to STAY green.
//   - EDGE/OOD : 005 (unreadable record never overwritten; read and write
//     errors surface on the loop console, never swallowed).
//   - WIRING   : 003 (runLoop entrypoint + wave + pool configs, under -race).
//   - SCOPE    : 004 (production diff confined to the three iteration-top
//     files, protected control-plane surfaces untouched, non-vacuous).
```

### `go/acs/cycle1693/predicates_test.go:186` — above `func TestC1693_004_ScopeConfinedToIterationTopFiles(t *testing.T) {`

```text
// TestC1693_004_ScopeConfinedToIterationTopFiles — AC3. Every production .go
// file this lane changes is one of the three iteration-top files; the
// protected control-plane surfaces are untouched; the fix actually landed in
// the allowed surface (non-vacuous); and the frozen contract is git-tracked
// (an untracked test is dropped at ship — cycle-93).
```
