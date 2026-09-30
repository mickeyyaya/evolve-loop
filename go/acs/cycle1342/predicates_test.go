//go:build acs

package cycle1342

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "./internal/phases/audit"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// acs-predicate: config-check — the criterion IS "this section of prose
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

func TestC1342_002_worktree_evidence_closes_defect(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_WorktreeResidentEvidenceClosesADefect")
	if !ok {
		t.Errorf("AC1 unmet — worktree-resident closure evidence is still rejected (missing PASS receipts: %v). evidenceResolves must retry os.Lstat under req.Worktree when the project-root lookup misses.\n%s", missing, tail(out, 25))
	}
}

func TestC1342_003_evidence_absent_from_both_roots_blocks(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_EvidenceAbsentFromBothRootsStillBlocks")
	if !ok {
		t.Errorf("AC2 unmet — a citation resolving under NEITHER root must still block PASS (missing: %v)\n%s", missing, tail(out, 25))
	}
}

func TestC1342_004_worktree_root_is_not_a_bypass(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_WorktreeSelfCitationStillRejected",
		"TestClassify_WorktreeEvidenceCannotEscapeRoot")
	if !ok {
		t.Errorf("AC3 unmet — the self-citation and path-escape rejections must survive the worktree fallback (missing: %v)\n%s", missing, tail(out, 25))
	}
}

func TestC1342_005_project_root_and_line_range_unchanged(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_ProjectRootEvidencePathUnchanged",
		"TestClassify_LineRangeCitationResolves",
		"TestClassify_NonLocatorSuffixIsPartOfThePath")
	if !ok {
		t.Errorf("AC4 unmet — project-root resolution, line-range locators, and the over-strip guard must all hold (missing: %v)\n%s", missing, tail(out, 25))
	}
}

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

func TestC1342_007_missing_disposition_file_is_named(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_DispositionPreflightMissingFileIsNamed")
	if !ok {
		t.Errorf("AC1 (Task 3) unmet — an entirely absent defect-dispositions.json on a continuation must fail with a NAMED structural pre-flight diagnostic, not only the per-id switch (missing: %v)\n%s", missing, tail(out, 25))
	}
}

func TestC1342_008_incomplete_disposition_file_is_named(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_DispositionPreflightIncompleteFileIsNamed")
	if !ok {
		t.Errorf("AC2 (Task 3) unmet — a partially-covered defect-dispositions.json must fail with a NAMED pre-flight diagnostic naming the missing ids (missing: %v)\n%s", missing, tail(out, 25))
	}
}

func TestC1342_009_complete_or_non_continuation_no_false_positive(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_DispositionPreflightCompleteFileNoFalsePositive",
		"TestClassify_DispositionPreflightNoAncestorNoOp")
	if !ok {
		t.Errorf("AC3 (Task 3) unmet — the pre-flight must stay silent on a complete disposition file and on a non-continuation cycle (missing: %v)\n%s", missing, tail(out, 25))
	}
}
