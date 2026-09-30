# Comment history: `acs/cycle507`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle507/predicates_test.go:3` — above `package cycle507`

```text
// Package cycle507 materialises the cycle-507 acceptance criteria.
//
// TRIAGE COMMITTED THREE ## top_n TASKS this cycle (triage-report.md):
//
//  1. wire-boot-recovery-functions  (bug, CRITICAL carryover from cycle 506's
//     FAILED audit) — QuarantineDirtyTree / ShipSHAMismatch / AutosealStaleMarker
//     wired into runLoop's boot path, with an integration test that fails if they
//     are not actually invoked (the piece cycle 506 lacked → audit F1).
//  2. prune-stale-carryover-todos   (goal-centric: context/token bloat) — a
//     TTL/expiry prune for state.json:carryoverTodos mirroring failurelog.PruneExpired.
//  3. fix-carryover-prompt-truncation-order (goal-centric: correctness) —
//     writeCarryoverTodos selects highest-priority/most-recent, not insertion order.
//
// Predicate strategy (mirrors cycle499/cycle503/cycle504): BEHAVIORAL predicates
// drive the system under test through its in-package RED tests via subprocess
// `go test`, asserting a non-degenerate pass (requireTestsRan closes the
// cycle-85 "no tests to run" trap) — never a source grep. The in-package tests
// were authored by the TDD engineer:
//
//	internal/core/boot_preflight_test.go          (Task 1 fn behavior)
//	internal/core/stale_marker_autoseal_test.go   (Task 1 fn behavior)
//	cmd/evolve/cmd_loop_boot_recovery_test.go      (Task 1 WIRING — the 506 gap)
//	internal/failurelog/prune_carryover_test.go   (Task 2 prune)
//	internal/core/carryover_ttl_stamp_test.go     (Task 2 creation-time stamp)
//	internal/core/carryover_prompt_order_test.go  (Task 3 ordering)
//
// The Builder implements production code ONLY (the seams named in those files);
// it must not modify the tests.
```

### `go/acs/cycle507/predicates_test.go:103` — above `func TestC507_003_BootRecoveryWiredIntoRunLoop(t *testing.T) {`

```text
// TestC507_003_BootRecoveryWiredIntoRunLoop (Task 1 WIRING — the CRITICAL
// criterion cycle 506 failed): runLoop actually INVOKES boot recovery before the
// readiness gate, and the orchestrator actually calls each primitive (dirty tree
// quarantined, dead-owner marker auto-sealed, ship-SHA mismatch flagged; clean
// state is a no-op). This is the anti-dead-code predicate: a recovery function
// that runLoop never calls is worthless (audit F1, warnship_apicover_ci_gap
// trap). Drives cmd/evolve cmd_loop_boot_recovery_test.go. RED today:
// bootRecoverFn / defaultBootRecovery / bootRecoveryResult undefined (package
// main test build fails).
```

### `go/acs/cycle507/predicates_test.go:148` — above `func TestC507_006_CarryoverPromptSelectsSevereRecent(t *testing.T) {`

```text
// TestC507_006_CarryoverPromptSelectsSevereRecent (Task 3): when carryoverTodos
// exceeds the render cap, writeCarryoverTodos renders the highest-priority /
// most-recent entries (the cycle-502/505 items are no longer hidden behind the
// omitted-count), tolerates a malformed Priority without panic, and keeps the
// cap boundary exact. Drives internal/core carryover_prompt_order_test.go. RED
// today: insertion-order slicing hides the tail entry.
```
