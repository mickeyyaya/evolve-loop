//go:build acs

// Package cycle1340 materializes the acceptance criteria of this lane's sole
// top_n task, `defect-ledger-worktree-evidence-fallback` (triage-report.md
// "## top_n"; scout-report.md Task 1 + Task 2, folded into one card by triage
// because the fix and its regression proof share one worktree/build/audit).
//
// The defect: `evidenceResolves` (go/internal/phases/audit/defect_ledger.go:253)
// resolves every closure citation with ONE os.Lstat under req.ProjectRoot. A
// continuation lane's own fix lives in its own still-open worktree and reaches
// the project root only on merge — which is exactly what this gate blocks.
// Cycles 1320 → 1323 → 1325 → 1330 each cited the same two real files and were
// rejected four times with the identical "resolves to no file under the project
// root". P0, cycles_unpicked=5+.
//
// EXECUTION SHAPE (why these predicates shell one narrowed `go test`):
// the subject, `evidenceResolves`, is unexported, and its only production
// caller is `reconcileAgainstAncestor` ← `reconcileContinuationDefects` ←
// `hooks.Classify` — all unexported inside package `audit`. An external acs
// package cannot reach that seam, and re-implementing the gate here would
// assert on a copy. So the behavioral contract lives IN the package
// (internal/phases/audit/defect_ledger_worktree_evidence_test.go), driven
// through the REAL production seam hooks{}.Classify, and each predicate below
// runs ONE named test in ONE named package with `-run` narrowing —
// the sanctioned shape (cycle1300/cycle1310/cycle1323 precedent), not a
// `./...` sweep and not one of the known 40s+ suites.
//
// A zero exit is NOT sufficient: each predicate demands the `--- PASS: <name>`
// receipt, so a deleted or skipped test reads as a miss rather than a pass —
// the frozen contract cannot be satisfied by removing it (rule 4).
//
// No new go/internal package is created by this task, so ADR-0069's repo-wide
// apicover enrollment does not apply (nothing to append to go/.apicover-enforce,
// no apicover_named_test.go to author).
//
// Adversarial diversity:
//
//	C1340_001 positive  — worktree-resident evidence closes an inherited defect
//	                      (the 1320→1330 deadlock, broken).
//	C1340_002 negative  — evidence under NEITHER root still blocks PASS
//	                      (deleting the Lstat passes 001 and fails this).
//	C1340_003 negative  — the self-citation guard (rule 4) rejects the gate's own
//	                      bookkeeping under the WORKTREE root too — the graded
//	                      agent writes that tree, so this is the fix's cheapest
//	                      bypass; plus the '..' escape rejection with a worktree
//	                      root present.
//	C1340_004 edge/regr — project-root evidence still closes when a worktree is
//	                      also set (fallback, not replacement), and an empty
//	                      req.Worktree (provisioning failed) still blocks.
//	C1340_005 no-regr   — the whole Classify verdict family in the audit package
//	                      stays green; the gate is shared by every cycle.
package cycle1340

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// auditPkg is the single named package under test. Narrowed further by -run on
// every invocation; never a /... sweep (flaky-predicate-shape rule 1).
const auditPkg = "./internal/phases/audit"

// goDir is the CYCLE's go module root — acsassert.RepoRoot resolves the
// worktree, so predicates read this lane's source, not main's.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runNamed runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./internal/phases/audit` and reports whether EVERY named test both RAN and
// PASSED.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal; a compile failure
//     in the target package — the expected RED signal before Builder implements
//     the fallback — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test was
//     deleted or skipped, not that it passed. Reported as a miss.
func runNamed(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", auditPkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", auditPkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

// tail returns the last n lines so verdict diagnostics stay readable.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// TestC1340_001_worktree_evidence_closes_defect — POSITIVE. AC1: a closure
// claim whose cited file exists in this lane's OWN worktree (and nowhere under
// the project root) is accepted, the continuation reaches PASS, and the
// written-back ledger records the entry FIXED with its evidence. This is the
// single assertion that is RED today.
func TestC1340_001_worktree_evidence_closes_defect(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_WorktreeResidentEvidenceClosesADefect")
	if !ok {
		t.Errorf("AC1 unmet — worktree-resident closure evidence is still rejected (missing PASS receipts: %v). This is the cycles 1320/1323/1325/1330 deadlock: evidenceResolves must retry os.Lstat under req.Worktree when the project-root lookup misses.\n%s", missing, tail(out, 25))
	}
}

// TestC1340_002_evidence_absent_from_both_roots_blocks — NEGATIVE, the
// anti-no-op. AC2: the fallback widens WHERE a real file may live, never
// whether one must exist. Deleting the Lstat satisfies 001 and fails here.
func TestC1340_002_evidence_absent_from_both_roots_blocks(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_EvidenceAbsentFromBothRootsStillBlocks")
	if !ok {
		t.Errorf("AC2 unmet — a citation resolving under NEITHER root must still block PASS (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1340_003_worktree_root_is_not_a_bypass — NEGATIVE. AC3: rule 4 (the
// self-citation guard, cycle-1285 F3) and the '..'-escape rejection apply to
// the worktree root exactly as to the project root. The graded agent writes
// its own worktree, so a guard that checks only ProjectRoot would hand the fix
// its cheapest bypass.
func TestC1340_003_worktree_root_is_not_a_bypass(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_WorktreeSelfCitationStillRejected",
		"TestClassify_WorktreeEvidenceCannotEscapeRoot")
	if !ok {
		t.Errorf("AC3 unmet — the self-citation and path-escape rejections must survive the worktree fallback (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1340_004_project_root_path_unchanged — EDGE / regression. AC4: evidence
// under the project root still closes a defect when a worktree is ALSO set
// (fallback, not replacement), and an empty req.Worktree — provisioning
// failed — still blocks an unresolvable citation instead of panicking or
// degrading open on the empty root.
func TestC1340_004_project_root_path_unchanged(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_ProjectRootEvidencePathUnchanged")
	if !ok {
		t.Errorf("AC4 unmet — the pre-existing project-root resolution and the empty-worktree case must be untouched (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1340_005_classify_family_no_regression — AC5. This gate runs on EVERY
// cycle's audit, continuation or not; the whole Classify verdict family in the
// audit package must stay green. Narrowed by -run to the TestClassify_ prefix
// in one named package — not a sweep.
func TestC1340_005_classify_family_no_regression(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-run", "^TestClassify_", auditPkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", auditPkg, code, err, tail(out, 30))
	}
	if code != 0 {
		t.Errorf("AC5 unmet — the Classify verdict family regressed (exit %d). This gate grades every cycle's audit; a fallback that fixes continuations by loosening the shared path is not the fix.\n%s", code, tail(out, 30))
	}
}
