//go:build acs

// Package cycle518 materialises the cycle-518 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle (triage-decision.json):
//
//	carryover-todo-expiry-never-set (bug — CarryoverTodo.ExpiresAt is the field
//	the loop-start failurelog.PruneExpiredCarryoverTodos pass reads to age
//	entries out; without it state.json:carryoverTodos grows unboundedly). The
//	fix must stamp a TTL on both records the failure-learning path creates —
//	the per-failed-phase CarryoverTodo + its sibling FailedRecord — and must
//	inherit (never fabricate) that stamp for defect-derived todos.
//
// (tasks wire-fleet-width-topn-selection / immediate-binary-drift-self-repin and
// all cycle-N-failed-* stubs are DEFERRED — no predicates authored for them, per
// R9.3: predicates bind ONLY to triage-committed work.)
//
// ── PRE-EXISTING GREEN (transparently reported) ────────────────────────────
// The production fix for this exact task landed in cycle 516 (HEAD 8808db17):
// recordFailureLearning stamps record.ExpiresAt = failurelog.ComputeExpiresAt(...)
// and shares it onto the created todo (failure_learning.go:272-286), and
// ApplyDefectsAsCarryoverTodos inherits record.ExpiresAt (failure_learning.go:578).
// Triage re-committed the lingering carryover stub whose root cause already
// shipped. These predicates therefore PIN an already-satisfied contract — they
// are GREEN at TDD time. They are NOT degenerate: each drives a real in-package
// test that CALLS the creation path and asserts on the stamped field, so reverting
// the stamp (or fabricating a bogus one) turns the driven test — and thus the
// predicate — RED. The Builder has no production code to write; it must not modify
// the tests. See test-report.md "## RED Run Output" for the non-degeneracy proof.
//
// Predicate strategy (mirrors cycle507/cycle514): BEHAVIORAL predicates drive the
// system under test through its in-package tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85
// "no tests to run" trap) — never a source grep. The driven tests:
//
//	internal/core/failure_learning_expiry_test.go  (creation-site stamp: positive + edge/compose)
//	internal/core/carryover_ttl_stamp_test.go      (inheritance: positive + negative/anti-fabrication)
package cycle518

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

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
// matching test (renamed/unwritten) — or a package that fails to build — exits
// without running the required tests, which must NOT green the predicate.
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

// TestC518_001_CreatedTodoStampsExpiresAt (AC-1, positive): the CarryoverTodo
// recordFailureLearning creates for a failed phase carries a non-empty, future
// RFC3339 ExpiresAt — the field failurelog.PruneExpiredCarryoverTodos reads to
// age it out. Drives internal/core failure_learning_expiry_test.go. RED if the
// creation site stops stamping the todo (state.json todos grow forever again).
func TestC518_001_CreatedTodoStampsExpiresAt(t *testing.T) {
	out, code := runGoTest(t,
		"TestRecordFailureLearning_CarryoverTodoStampsExpiresAt", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("recordFailureLearning does NOT stamp ExpiresAt on the CarryoverTodo it creates (exit=%d) — the loop-start prune pass then keeps every failure stub forever\n%s", code, out)
	}
}

// TestC518_002_FailedRecordStampsExpiresAt (AC-2, positive): the FailedRecord
// appended to state.FailedAt also carries a non-empty, future ExpiresAt, so the
// single-sourced TTL logic that defect-derived todos inherit from is itself
// populated. Drives internal/core failure_learning_expiry_test.go.
func TestC518_002_FailedRecordStampsExpiresAt(t *testing.T) {
	out, code := runGoTest(t,
		"TestRecordFailureLearning_FailedRecordStampsExpiresAt", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("recordFailureLearning does NOT stamp the FailedRecord's ExpiresAt (exit=%d) — an unstamped source record silently poisons ApplyDefectsAsCarryoverTodos inheritance\n%s", code, out)
	}
}

// TestC518_003_FreshTodoSurvivesImmediatePrune (AC-3, edge/compose): a todo
// created THIS SECOND by the REAL recordFailureLearning path survives an
// immediate run of the REAL PruneExpiredCarryoverTodos, and survives BECAUSE it
// carries a real, not-yet-elapsed TTL stamp (not by the legacy "age unknown,
// never delete" rule). Proves creation-stamp + prune-read compose on production
// data, not just hand-built fixtures. Drives internal/core failure_learning_expiry_test.go.
func TestC518_003_FreshTodoSurvivesImmediatePrune(t *testing.T) {
	out, code := runGoTest(t,
		"TestRecordFailureLearning_CreatedTodoSurvivesImmediatePrune", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("a fresh real-path todo does not compose with the real prune pass (exit=%d) — creation-time stamp and prune-time read do not agree on production data\n%s", code, out)
	}
}

// TestC518_004_DefectTodoInheritsRecordStamp (AC-4, positive/inheritance): a
// defect-derived carryover todo inherits the failed record's TTL stamp so it
// becomes prune-eligible after the retention window — the two arrays' TTL logic
// stays single-sourced (never recompute). Drives internal/core carryover_ttl_stamp_test.go.
func TestC518_004_DefectTodoInheritsRecordStamp(t *testing.T) {
	out, code := runGoTest(t,
		"TestApplyDefectsAsCarryoverTodos_StampsExpiryFromRecord", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("ApplyDefectsAsCarryoverTodos does NOT inherit the record's ExpiresAt onto the todo (exit=%d) — defect todos become unprunable\n%s", code, out)
	}
}

// TestC518_005_NoRecordExpiryLeavesTodoUnstamped (AC-5, negative / anti-
// fabrication): when the failed record carries NO expiry (a true legacy record),
// the created todo carries none either — the prune keeps age-unknown entries, so
// a fabricated stamp would wrongly age out data whose age we cannot know. This is
// the anti-no-op predicate: a naive "always stamp now()" implementation FAILS
// here. Drives internal/core carryover_ttl_stamp_test.go.
func TestC518_005_NoRecordExpiryLeavesTodoUnstamped(t *testing.T) {
	out, code := runGoTest(t,
		"TestApplyDefectsAsCarryoverTodos_NoRecordExpiryLeavesTodoUnstamped", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("a record with no ExpiresAt fabricates a TTL stamp on its todo (exit=%d) — anti-fabrication broken; the stamp must be inherited, not invented\n%s", code, out)
	}
}
