//go:build acs

package cycle518

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

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
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

func TestC518_001_CreatedTodoStampsExpiresAt(t *testing.T) {
	out, code := runGoTest(t,
		"TestRecordFailureLearning_CarryoverTodoStampsExpiresAt", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("recordFailureLearning does NOT stamp ExpiresAt on the CarryoverTodo it creates (exit=%d) — the loop-start prune pass then keeps every failure stub forever\n%s", code, out)
	}
}

func TestC518_002_FailedRecordStampsExpiresAt(t *testing.T) {
	out, code := runGoTest(t,
		"TestRecordFailureLearning_FailedRecordStampsExpiresAt", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("recordFailureLearning does NOT stamp the FailedRecord's ExpiresAt (exit=%d) — an unstamped source record silently poisons ApplyDefectsAsCarryoverTodos inheritance\n%s", code, out)
	}
}

func TestC518_003_FreshTodoSurvivesImmediatePrune(t *testing.T) {
	out, code := runGoTest(t,
		"TestRecordFailureLearning_CreatedTodoSurvivesImmediatePrune", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("a fresh real-path todo does not compose with the real prune pass (exit=%d) — creation-time stamp and prune-time read do not agree on production data\n%s", code, out)
	}
}

func TestC518_004_DefectTodoInheritsRecordStamp(t *testing.T) {
	out, code := runGoTest(t,
		"TestApplyDefectsAsCarryoverTodos_StampsExpiryFromRecord", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("ApplyDefectsAsCarryoverTodos does NOT inherit the record's ExpiresAt onto the todo (exit=%d) — defect todos become unprunable\n%s", code, out)
	}
}

func TestC518_005_NoRecordExpiryLeavesTodoUnstamped(t *testing.T) {
	out, code := runGoTest(t,
		"TestApplyDefectsAsCarryoverTodos_NoRecordExpiryLeavesTodoUnstamped", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("a record with no ExpiresAt fabricates a TTL stamp on its todo (exit=%d) — anti-fabrication broken; the stamp must be inherited, not invented\n%s", code, out)
	}
}
