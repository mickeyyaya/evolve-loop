//go:build acs

package cycle1340

import (
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

func TestC1340_001_worktree_evidence_closes_defect(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_WorktreeResidentEvidenceClosesADefect")
	if !ok {
		t.Errorf("AC1 unmet — worktree-resident closure evidence is still rejected (missing PASS receipts: %v). This is the cycles 1320/1323/1325/1330 deadlock: evidenceResolves must retry os.Lstat under req.Worktree when the project-root lookup misses.\n%s", missing, tail(out, 25))
	}
}

func TestC1340_002_evidence_absent_from_both_roots_blocks(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_EvidenceAbsentFromBothRootsStillBlocks")
	if !ok {
		t.Errorf("AC2 unmet — a citation resolving under NEITHER root must still block PASS (missing: %v)\n%s", missing, tail(out, 25))
	}
}

func TestC1340_003_worktree_root_is_not_a_bypass(t *testing.T) {
	ok, missing, out := runNamed(t,
		"TestClassify_WorktreeSelfCitationStillRejected",
		"TestClassify_WorktreeEvidenceCannotEscapeRoot")
	if !ok {
		t.Errorf("AC3 unmet — the self-citation and path-escape rejections must survive the worktree fallback (missing: %v)\n%s", missing, tail(out, 25))
	}
}

func TestC1340_004_project_root_path_unchanged(t *testing.T) {
	ok, missing, out := runNamed(t, "TestClassify_ProjectRootEvidencePathUnchanged")
	if !ok {
		t.Errorf("AC4 unmet — the pre-existing project-root resolution and the empty-worktree case must be untouched (missing: %v)\n%s", missing, tail(out, 25))
	}
}

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
