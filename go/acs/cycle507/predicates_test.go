//go:build acs

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
package cycle507

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg       = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	failurelogPkg = "github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	cmdEvolvePkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// runGoTest runs `go test` on pkg filtered by runFilter, returning combined
// output + exit code. Behavioral predicates invoke the system under test through
// its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with no
// matching test exits 0 with "no tests to run", which would green a predicate on
// unwritten (or renamed) work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// TestC507_001_BootRecoveryFunctionsBehave (Task 1 fn contract): the three
// recovery primitives behave — dirty tracked source is classified & quarantined
// (git status clean afterward, content preserved via stash), loop-managed dirs
// are excluded, and a ship-binary SHA mismatch is detected with no false
// positive. Drives internal/core boot_preflight_test.go. RED today:
// QuarantineDirtyTree / ShipSHAMismatch / classifyDirtyPaths are undefined
// (core test package fails to compile).
func TestC507_001_BootRecoveryFunctionsBehave(t *testing.T) {
	out, code := runGoTest(t,
		"TestClassifyDirtyPaths|TestQuarantineDirtyTree_LeavesStatusCleanAndPreservesContent|TestShipSHAMismatch_DetectsTamperNotFalsePositive",
		corePkg)
	requireTestsRan(t, out, 5)
	if code != 0 {
		t.Errorf("boot-recovery primitives are red (exit=%d) — QuarantineDirtyTree/ShipSHAMismatch/classifyDirtyPaths missing or wrong\n%s", code, out)
	}
}

// TestC507_002_StaleMarkerAutosealBehaves (Task 1 fn contract, edge): a marker
// owned by a DEAD pid auto-seals (fail-safe when pid is missing), a LIVE-owner
// marker is left untouched, and the end-to-end AutosealStaleMarker reuses
// SealCycle(Force) (exactly one ledger append) then clears the block. Drives
// internal/core stale_marker_autoseal_test.go. RED today: markerShouldAutoseal /
// AutosealStaleMarker undefined.
func TestC507_002_StaleMarkerAutosealBehaves(t *testing.T) {
	out, code := runGoTest(t,
		"TestMarkerShouldAutoseal|TestAutosealStaleMarker_DeadOwnerSealsViaSealCycleAndClearsBlock",
		corePkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("stale-marker autoseal is red (exit=%d) — markerShouldAutoseal/AutosealStaleMarker missing or wrong\n%s", code, out)
	}
}

// TestC507_003_BootRecoveryWiredIntoRunLoop (Task 1 WIRING — the CRITICAL
// criterion cycle 506 failed): runLoop actually INVOKES boot recovery before the
// readiness gate, and the orchestrator actually calls each primitive (dirty tree
// quarantined, dead-owner marker auto-sealed, ship-SHA mismatch flagged; clean
// state is a no-op). This is the anti-dead-code predicate: a recovery function
// that runLoop never calls is worthless (audit F1, warnship_apicover_ci_gap
// trap). Drives cmd/evolve cmd_loop_boot_recovery_test.go. RED today:
// bootRecoverFn / defaultBootRecovery / bootRecoveryResult undefined (package
// main test build fails).
func TestC507_003_BootRecoveryWiredIntoRunLoop(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery|TestRunLoop_InvokesBootRecoveryBeforeGate",
		cmdEvolvePkg)
	requireTestsRan(t, out, 5)
	if code != 0 {
		t.Errorf("boot recovery is NOT wired into runLoop (exit=%d) — the exact cycle-506 dead-code failure; wire bootRecoverFn into runLoop's boot path\n%s", code, out)
	}
}

// TestC507_004_CarryoverTodosPruneExpired (Task 2): PruneExpiredCarryoverTodos
// removes entries whose expiresAt is past, KEEPS untimestamped legacy entries
// (age unknowable), and is a safe no-op on a missing/empty state. Mirrors the
// failedApproaches sibling. Drives internal/failurelog prune_carryover_test.go.
// RED today: PruneExpiredCarryoverTodos undefined.
func TestC507_004_CarryoverTodosPruneExpired(t *testing.T) {
	out, code := runGoTest(t, "TestPruneExpiredCarryoverTodos", failurelogPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("carryoverTodos TTL prune is red (exit=%d) — PruneExpiredCarryoverTodos missing or wrong (expired-removed / legacy-kept / missing-noop)\n%s", code, out)
	}
}

// TestC507_005_CarryoverTodoTTLStamped (Task 2, creation half): a defect-derived
// carryover todo INHERITS the failed record's TTL stamp so the prune can age it
// out — and fabricates none when the record carries no expiry. Drives
// internal/core carryover_ttl_stamp_test.go. RED today: CarryoverTodo has no
// ExpiresAt field.
func TestC507_005_CarryoverTodoTTLStamped(t *testing.T) {
	out, code := runGoTest(t, "TestApplyDefectsAsCarryoverTodos_StampsExpiryFromRecord|TestApplyDefectsAsCarryoverTodos_NoRecordExpiryLeavesTodoUnstamped", corePkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("carryover TTL stamp is red (exit=%d) — CarryoverTodo.ExpiresAt missing or ApplyDefectsAsCarryoverTodos does not inherit record.ExpiresAt\n%s", code, out)
	}
}

// TestC507_006_CarryoverPromptSelectsSevereRecent (Task 3): when carryoverTodos
// exceeds the render cap, writeCarryoverTodos renders the highest-priority /
// most-recent entries (the cycle-502/505 items are no longer hidden behind the
// omitted-count), tolerates a malformed Priority without panic, and keeps the
// cap boundary exact. Drives internal/core carryover_prompt_order_test.go. RED
// today: insertion-order slicing hides the tail entry.
func TestC507_006_CarryoverPromptSelectsSevereRecent(t *testing.T) {
	out, code := runGoTest(t,
		"TestWriteCarryoverTodos_SevereRecentSurvivesTheCut|TestWriteCarryoverTodos_MalformedPriorityDoesNotPanic|TestWriteCarryoverTodos_CapBoundaryExact",
		corePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("carryover prompt ordering is red (exit=%d) — writeCarryoverTodos still slices insertion-order todos[:cap] and hides the newest/most-severe\n%s", code, out)
	}
}

// TestC507_007_CarryoverPruneWiredIntoRunLoop (Task 2 WIRING — anti-dead-code):
// runLoop's startup actually prunes an EXPIRED carryover todo from state.json
// (while keeping a fresh one and an untimestamped legacy one), proving
// PruneExpiredCarryoverTodos is wired into the AutoPrune block — not merely
// defined. Drives cmd/evolve cmd_loop_carryover_prune_test.go. RED today: the
// prune is not wired, so the expired entry survives (and package main also
// fails to build until the Task 1 seam lands — both are this cycle's top_n).
func TestC507_007_CarryoverPruneWiredIntoRunLoop(t *testing.T) {
	out, code := runGoTest(t, "TestRunLoop_AutoPrunesExpiredCarryoverTodos", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("carryover prune is NOT wired into runLoop startup (exit=%d) — wire PruneExpiredCarryoverTodos beside PruneExpired in the AutoPrune block\n%s", code, out)
	}
}
