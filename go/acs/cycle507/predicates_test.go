//go:build acs

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

func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

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

func TestC507_001_BootRecoveryFunctionsBehave(t *testing.T) {
	out, code := runGoTest(t,
		"TestClassifyDirtyPaths|TestQuarantineDirtyTree_LeavesStatusCleanAndPreservesContent|TestShipSHAMismatch_DetectsTamperNotFalsePositive",
		corePkg)
	requireTestsRan(t, out, 5)
	if code != 0 {
		t.Errorf("boot-recovery primitives are red (exit=%d) — QuarantineDirtyTree/ShipSHAMismatch/classifyDirtyPaths missing or wrong\n%s", code, out)
	}
}

func TestC507_002_StaleMarkerAutosealBehaves(t *testing.T) {
	out, code := runGoTest(t,
		"TestMarkerShouldAutoseal|TestAutosealStaleMarker_DeadOwnerSealsViaSealCycleAndClearsBlock",
		corePkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("stale-marker autoseal is red (exit=%d) — markerShouldAutoseal/AutosealStaleMarker missing or wrong\n%s", code, out)
	}
}

func TestC507_003_BootRecoveryWiredIntoRunLoop(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery|TestRunLoop_InvokesBootRecoveryBeforeGate",
		cmdEvolvePkg)
	requireTestsRan(t, out, 5)
	if code != 0 {
		t.Errorf("boot recovery is NOT wired into runLoop (exit=%d) — the exact cycle-506 dead-code failure; wire bootRecoverFn into runLoop's boot path\n%s", code, out)
	}
}

func TestC507_004_CarryoverTodosPruneExpired(t *testing.T) {
	out, code := runGoTest(t, "TestPruneExpiredCarryoverTodos", failurelogPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("carryoverTodos TTL prune is red (exit=%d) — PruneExpiredCarryoverTodos missing or wrong (expired-removed / legacy-kept / missing-noop)\n%s", code, out)
	}
}

func TestC507_005_CarryoverTodoTTLStamped(t *testing.T) {
	out, code := runGoTest(t, "TestApplyDefectsAsCarryoverTodos_StampsExpiryFromRecord|TestApplyDefectsAsCarryoverTodos_NoRecordExpiryLeavesTodoUnstamped", corePkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("carryover TTL stamp is red (exit=%d) — CarryoverTodo.ExpiresAt missing or ApplyDefectsAsCarryoverTodos does not inherit record.ExpiresAt\n%s", code, out)
	}
}

func TestC507_006_CarryoverPromptSelectsSevereRecent(t *testing.T) {
	out, code := runGoTest(t,
		"TestWriteCarryoverTodos_SevereRecentSurvivesTheCut|TestWriteCarryoverTodos_MalformedPriorityDoesNotPanic|TestWriteCarryoverTodos_CapBoundaryExact",
		corePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("carryover prompt ordering is red (exit=%d) — writeCarryoverTodos still slices insertion-order todos[:cap] and hides the newest/most-severe\n%s", code, out)
	}
}

func TestC507_007_CarryoverPruneWiredIntoRunLoop(t *testing.T) {
	out, code := runGoTest(t, "TestRunLoop_AutoPrunesExpiredCarryoverTodos", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("carryover prune is NOT wired into runLoop startup (exit=%d) — wire PruneExpiredCarryoverTodos beside PruneExpired in the AutoPrune block\n%s", code, out)
	}
}
