//go:build acs

// Package cycle1342 materializes the acceptance criteria of this lane's
// three top_n tasks (scout-report.md, fleet_scope
// `defect-ledger-worktree-evidence-fallback`):
//
//   - Task 1 auditor-disposition-schema-producer: agents/evolve-auditor.md
//     documents the defect-dispositions.json schema (F3, 0.97 — the actual
//     unblock for THIS cycle; a prompt gap, not a code gap).
//   - Task 2 defect-ledger-worktree-fallback-land: evidenceResolves
//     (go/internal/phases/audit/defect_ledger.go) resolves a closure
//     citation under req.Worktree when req.ProjectRoot misses, and treats a
//     ":line-line" range suffix as one locator. SELF-GRADING — this ports
//     the cycle-1340 fix so it grades SUCCESSOR cycles; it cannot change
//     THIS cycle's verdict (see build-report.md).
//   - Task 3 disposition-completeness-preflight: a continuation whose
//     defect-dispositions.json is entirely absent, or covers fewer ids than
//     the ancestor ledger enumerates, must fail with a diagnostic named
//     distinctly from the existing per-id "(no disposition)" switch branch
//     — a structural pre-flight, not only a per-id fallthrough.
//
// EXECUTION SHAPE (why these predicates shell one narrowed `go test`):
// the Task 2/3 subjects (evidenceResolves, the disposition pre-flight) are
// unexported, and their only production caller is hooks.Classify — all
// inside package `audit`. An external acs package cannot reach that seam,
// and re-implementing the gate here would assert on a copy. So the
// behavioral contract lives IN the package
// (internal/phases/audit/defect_ledger_worktree_evidence_test.go and
// internal/phases/audit/disposition_preflight_test.go), driven through the
// REAL production seam hooks{}.Classify, and each predicate below runs ONE
// named test in ONE named package with `-run` narrowing — never a `./...`
// sweep and not one of the known 40s+ suites (flaky-predicate-shape rule 1).
//
// A zero exit is NOT sufficient: each predicate demands the `--- PASS:
// <name>` receipt, so a deleted or skipped test reads as a miss rather than
// a pass — the frozen contract cannot be satisfied by removing it.
//
// No new go/internal package is created by any of the three tasks, so
// ADR-0069's repo-wide apicover enrollment does not apply.
//
// Adversarial diversity:
//
//	C1342_001 positive  — agents/evolve-auditor.md names the
//	                      defect-dispositions.json schema + re-author rule.
//	C1342_002 positive  — worktree-resident evidence closes an inherited
//	                      defect (the 1320→1330 deadlock, broken).
//	C1342_003 negative  — evidence under NEITHER root still blocks PASS.
//	C1342_004 negative  — the self-citation guard + '..'-escape rejection
//	                      survive the worktree fallback.
//	C1342_005 edge/regr — project-root evidence unchanged; empty
//	                      req.Worktree still blocks; ":line-line" ranges
//	                      resolve; a non-numeric ':' suffix is NOT stripped.
//	C1342_006 no-regr   — the whole Classify verdict family stays green.
//	C1342_007 negative  — a continuation with NO defect-dispositions.json at
//	                      all fails with a NAMED pre-flight diagnostic.
//	C1342_008 negative  — a continuation whose defect-dispositions.json
//	                      covers only SOME inherited ids fails with a NAMED
//	                      pre-flight diagnostic naming what's missing.
//	C1342_009 edge       — a fully-dispositioned continuation trips NEITHER
//	                      new pre-flight message (anti-no-op: a pre-flight
//	                      that always fires proves nothing); an ordinary
//	                      (non-continuation) cycle trips neither either.
package cycle1342

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// auditPkg is the single named package under test. Narrowed further by -run
// on every invocation; never a /... sweep (flaky-predicate-shape rule 1).
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
//   - code < 0 is a genuine "could not launch" and is fatal; a compile
//     failure in the target package — the expected RED signal before Builder
//     implements the fix — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test
//     was deleted or skipped, not that it passed. Reported as a miss.
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

// -- Task 1: agents/evolve-auditor.md documents the disposition schema -----

// TestC1342_001_auditor_prompt_documents_disposition_schema — POSITIVE. The
// actual unblock for THIS cycle (Finding 3, 0.97): the auditor is graded
// against defect-dispositions.json but its own prompt never names the
// schema, so continuations coin-flip between inferring it from a Go error
// string and never writing the file at all. This predicate reads the
// PROMPT TEXT ITSELF — the deliverable of Task 1 is prose, not behavior a
// unit test could otherwise exercise, so asserting file content here is the
// full criterion, not a degenerate stand-in for it (predicate-quality
// waiver: the AC is textual).
//
// acs-predicate: config-check — the criterion IS "this section of prose
// exists in this file"; there is no behavior to drive around it.
func TestC1342_001_auditor_prompt_documents_disposition_schema(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-auditor.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(raw)

	if !strings.Contains(body, "defect-dispositions.json") {
		t.Fatalf("AC1 unmet — agents/evolve-auditor.md does not mention defect-dispositions.json at all. The auditor must be told, in its own prompt, to write this file before emitting a verdict on a continuation cycle (Finding 3, cycle-1340 lesson `cycle-1340-defect-dispositions-are-per-workspace-and-never-inherit`, confidence 0.97).")
	}
	if !strings.Contains(body, `"dispositions"`) {
		t.Fatalf("AC1 unmet — the section mentions defect-dispositions.json but never quotes the {\"dispositions\":[...]} wire shape (id/status/evidence/reason), so an auditor reading only its own prompt cannot author a well-formed file.")
	}
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "every cycle") && !strings.Contains(lower, "each cycle") {
		t.Errorf("AC1 unmet — the section must state the re-author-every-cycle rule explicitly (an ancestor's defect-dispositions.json is never inherited/read); found no 'every cycle'/'each cycle' phrasing near the schema.")
	}
}

// -- Task 2: worktree-fallback evidence resolution (self-grading) ----------

// TestC1342_002_worktree_evidence_closes_defect — POSITIVE. AC1: a closure
// claim whose cited file exists in this lane's OWN worktree (and nowhere
// under the project root) is accepted, the continuation reaches PASS, and
// the written-back ledger records the entry FIXED with its evidence.
func TestC1342_002_worktree_evidence_closes_defect(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_WorktreeResidentEvidenceClosesADefect")
	if !ok {
		t.Errorf("AC1 unmet — worktree-resident closure evidence is still rejected (missing PASS receipts: %v). evidenceResolves must retry os.Lstat under req.Worktree when the project-root lookup misses.\n%s", missing, tail(out, 25))
	}
}

// TestC1342_003_evidence_absent_from_both_roots_blocks — NEGATIVE, the
// anti-no-op. AC2: the fallback widens WHERE a real file may live, never
// whether one must exist.
func TestC1342_003_evidence_absent_from_both_roots_blocks(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_EvidenceAbsentFromBothRootsStillBlocks")
	if !ok {
		t.Errorf("AC2 unmet — a citation resolving under NEITHER root must still block PASS (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1342_004_worktree_root_is_not_a_bypass — NEGATIVE. AC3: the
// self-citation guard and the '..'-escape rejection apply to the worktree
// root exactly as to the project root.
func TestC1342_004_worktree_root_is_not_a_bypass(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_WorktreeSelfCitationStillRejected",
		"TestClassify_WorktreeEvidenceCannotEscapeRoot")
	if !ok {
		t.Errorf("AC3 unmet — the self-citation and path-escape rejections must survive the worktree fallback (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1342_005_project_root_and_line_range_unchanged — EDGE / regression.
// AC4: evidence under the project root still closes a defect when a
// worktree is ALSO set; a ":line-line" range locator resolves; a
// non-numeric ':' suffix is NOT stripped (over-strip guard); an empty
// req.Worktree still blocks an unresolvable citation.
func TestC1342_005_project_root_and_line_range_unchanged(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_ProjectRootEvidencePathUnchanged",
		"TestClassify_LineRangeCitationResolves",
		"TestClassify_NonLocatorSuffixIsPartOfThePath")
	if !ok {
		t.Errorf("AC4 unmet — project-root resolution, line-range locators, and the over-strip guard must all hold (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1342_006_classify_family_no_regression — AC5. This gate runs on
// EVERY cycle's audit, continuation or not; the whole Classify verdict
// family in the audit package must stay green.
func TestC1342_006_classify_family_no_regression(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-run", "^TestClassify_", auditPkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", auditPkg, code, err, tail(out, 30))
	}
	if code != 0 {
		t.Errorf("AC5 unmet — the Classify verdict family regressed (exit %d). This gate grades every cycle's audit.\n%s", code, tail(out, 30))
	}
}

// -- Task 3: disposition-completeness pre-flight ---------------------------

// TestC1342_007_missing_disposition_file_is_named — NEGATIVE. A continuation
// with an ancestor ledger and NO defect-dispositions.json at all must fail
// with a diagnostic naming the pre-flight distinctly — not merely the
// per-id "(no disposition)" fallthrough already in
// reconcileAgainstAncestor's switch.
func TestC1342_007_missing_disposition_file_is_named(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_DispositionPreflightMissingFileIsNamed")
	if !ok {
		t.Errorf("AC1 (Task 3) unmet — an entirely absent defect-dispositions.json on a continuation must fail with a NAMED structural pre-flight diagnostic, not only the per-id switch (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1342_008_incomplete_disposition_file_is_named — NEGATIVE. A
// defect-dispositions.json that covers only SOME of the ancestor's ids must
// fail with a pre-flight diagnostic that names the coverage gap (count +
// which ids), distinct from the per-id switch text.
func TestC1342_008_incomplete_disposition_file_is_named(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_DispositionPreflightIncompleteFileIsNamed")
	if !ok {
		t.Errorf("AC2 (Task 3) unmet — a partially-covered defect-dispositions.json must fail with a NAMED pre-flight diagnostic naming the missing ids (missing: %v)\n%s", missing, tail(out, 25))
	}
}

// TestC1342_009_complete_or_non_continuation_no_false_positive — EDGE, the
// anti-no-op for Task 3. A pre-flight that fires unconditionally proves
// nothing: a fully-dispositioned continuation, and an ordinary
// (non-continuation) cycle, must trip NEITHER new pre-flight message.
func TestC1342_009_complete_or_non_continuation_no_false_positive(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_DispositionPreflightCompleteFileNoFalsePositive",
		"TestClassify_DispositionPreflightNoAncestorNoOp")
	if !ok {
		t.Errorf("AC3 (Task 3) unmet — the pre-flight must stay silent on a complete disposition file and on a non-continuation cycle (missing: %v)\n%s", missing, tail(out, 25))
	}
}
