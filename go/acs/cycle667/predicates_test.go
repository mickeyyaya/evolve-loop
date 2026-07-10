//go:build acs

// Package cycle667 materialises the cycle-667 acceptance criteria for the single
// triage-committed (`## top_n`) task: chronicle-s4-carryover-orphan-merge (weight
// 0.92).
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-decision.json / triage-report.md commit exactly ONE task to this cycle:
//	  chronicle-s4-carryover-orphan-merge — C667_001..007
//	The scout report proposed the echo-veto-wiring family, but triage `## top_n`
//	(the sole task authority) DEFERRED those and committed chronicle-s4 instead, so
//	the predicates bind to THAT. Every non-committed / deferred item gets ZERO
//	predicates.
//
// ORPHAN CONTEXT — evolve-memo (the PASS-branch scribe, dispatched post-ship)
// writes <workspace>/carryover-todos.json every PASS cycle, and the retro path
// writes the same file on FAIL, but NO Go code ever reads it (grep: zero
// readers). The PASS-branch learning channel is fire-and-forget: queued todos
// never reach state.json:carryoverTodos, so the next cycle's planner never sees
// them. Fixed this cycle by a cycle-terminal hook (MergeWorkspaceCarryover, wired
// in finalizeCycle beside persistCycleEndState) that tolerant-decodes the file,
// caps + priority-maps + TTL-stamps each entry, and merges via the existing
// mergeCarryoverTodos (dedup by id ⇒ idempotent).
//
// PREDICATE QUALITY (cycle-85): every predicate EXERCISES the SUT. Each shells
// `go test -race -v -run <name>` against the real internal/core package and
// asserts the named TDD-authored behavioral test actually RAN and PASSED (the
// `--- PASS: <name>` marker) — a package that compiles but lacks the test prints
// "no tests to run" (exit 0) with NO marker, so a bare exit check would vacuously
// green. C667_006 additionally asserts the touched package builds against the new
// surface AND that no DATA RACE is reported.
//
// TEST-NAME CONTRACT — these behavioral tests are authored by the TDD engineer in
// go/internal/core/carryover_merge_test.go (RED now; Builder must NOT modify them,
// only add production code — go/internal/core/carryover_merge.go + the
// finalizeCycle call site — to green them):
//
//	internal/core : TestRunCycle_MergesMemoCarryoverTodosIntoState
//	                TestMergeWorkspaceCarryover_DedupesById
//	                TestMergeWorkspaceCarryover_CapsActionRunes
//	                TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails
//	                TestMergeWorkspaceCarryover_StampsExpiryForPrune
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/wiring : C667_001 the real terminal path (finalizeCycle) persists
//     the memo todos — the load-bearing anti-no-op signal (a defined-but-unwired
//     helper leaves the orphan open and fails here).
//   - Semantic        : C667_002 re-entry is idempotent (dedup by id).
//   - Edge            : C667_003 an oversized action is capped.
//   - Negative/OOD    : C667_004 malformed JSON + id/action-less entries tolerated
//     (WARN, no panic, no fatal).
//   - Semantic        : C667_005 every merged todo carries a future ExpiresAt so
//     the loop-start prune converges.
//   - Integrity       : C667_006 internal/core builds against the new surface and
//     the merge tests pass under -race (no DATA RACE).
package cycle667

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "./internal/core/..."

// goDir returns the go module directory for `go test -C <dir>` subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runNamedTest runs `go test -race -v -run ^<name>$ <pkg>` and reports whether
// the named test actually RAN and PASSED. The `--- PASS: <name>` marker guards
// the "-run matches nothing -> exit 0" false-green.
func runNamedTest(t *testing.T, pkg, name string) (passed bool, out string) {
	t.Helper()
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-race", "-count=1", "-v",
		"-run", "^"+name+"$", pkg,
	)
	out = stdout + "\n" + stderr
	return strings.Contains(out, "--- PASS: "+name), out
}

// TestC667_001_TerminalHookMergesMemoIntoPersistedState — AC1 (wiring). The
// cycle-terminal path (finalizeCycle) must persist the workspace's memo
// carryover-todos into state.CarryoverTodos.
func TestC667_001_TerminalHookMergesMemoIntoPersistedState(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestRunCycle_MergesMemoCarryoverTodosIntoState")
	if !passed {
		t.Fatalf("RED: TestRunCycle_MergesMemoCarryoverTodosIntoState did not run+PASS.\n"+
			"Builder must call MergeWorkspaceCarryover(state, cs.WorkspacePath, cycle, now) in\n"+
			"finalizeCycle (beside persistCycleEndState) so a workspace carryover-todos.json\n"+
			"lands in the PERSISTED state — closing the orphaned PASS/retro learning channel.\nOutput:\n%s", out)
	}
}

// TestC667_002_MergeDedupesById — AC2 (semantic). Re-entry over the same file is
// idempotent; no id is duplicated.
func TestC667_002_MergeDedupesById(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_DedupesById")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_DedupesById did not run+PASS.\n"+
			"MergeWorkspaceCarryover must dedup by id via the existing mergeCarryoverTodos so a\n"+
			"crash-resume / double-invocation never duplicates a todo.\nOutput:\n%s", out)
	}
}

// TestC667_003_MergeCapsActionRunes — AC3 (edge). An oversized action is bounded
// to the capRunes ceiling.
func TestC667_003_MergeCapsActionRunes(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_CapsActionRunes")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_CapsActionRunes did not run+PASS.\n"+
			"MergeWorkspaceCarryover must cap the decoded action via capRunes(action,\n"+
			"maxAdoptedDefectRunes) so a memo todo cannot bloat every future router prompt.\nOutput:\n%s", out)
	}
}

// TestC667_004_MergeMalformedTolerated — AC4 (negative/OOD). Corrupt JSON and
// id/action-less entries are tolerated (WARN, no panic, no fatal).
func TestC667_004_MergeMalformedTolerated(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails did not run+PASS.\n"+
			"MergeWorkspaceCarryover must tolerant-decode: a malformed file WARNs (never fatals),\n"+
			"and entries missing id or action are skipped. The cycle-terminal hook must never\n"+
			"abort the cycle over a bad memo file.\nOutput:\n%s", out)
	}
}

// TestC667_005_MergeStampsExpiryForPrune — AC5 (semantic). Every merged todo
// carries a future RFC3339 ExpiresAt (+ FirstSeenCycle) so the loop-start prune
// converges.
func TestC667_005_MergeStampsExpiryForPrune(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_StampsExpiryForPrune")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_StampsExpiryForPrune did not run+PASS.\n"+
			"MergeWorkspaceCarryover must stamp FirstSeenCycle + a future ExpiresAt (same TTL\n"+
			"discipline as the loop-start backfill, e.g. failurelog.ComputeExpiresAt) so\n"+
			"failurelog.PruneExpiredCarryoverTodos can age the array out.\nOutput:\n%s", out)
	}
}

// TestC667_006_CoreBuildsAndMergeTestsRaceClean — AC6a (integrity). internal/core
// builds against the new merge surface, and the merge tests pass under -race with
// no DATA RACE. The go-build guard makes a magic-string-only "fix" impossible
// (core must actually compile against MergeWorkspaceCarryover).
func TestC667_006_CoreBuildsAndMergeTestsRaceClean(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/core/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: internal/core does not build against the MergeWorkspaceCarryover surface (exit=%d): %v\n%s",
			code, err, errOut)
	}
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-race", "-count=1", "-v",
		"-run", "^TestMergeWorkspaceCarryover_", corePkg,
	)
	out := stdout + "\n" + stderr
	if strings.Contains(out, "DATA RACE") {
		t.Fatalf("RED: DATA RACE detected in MergeWorkspaceCarryover tests:\n%s", out)
	}
	if !strings.Contains(out, "--- PASS: TestMergeWorkspaceCarryover_DedupesById") {
		t.Fatalf("RED: MergeWorkspaceCarryover tests did not pass under -race.\nOutput:\n%s", out)
	}
}
