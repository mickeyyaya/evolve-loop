package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func brGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func brCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	brGit(t, dir, "add", name)
	brGit(t, dir, "commit", "-q", "-m", msg)
}

// brFixture builds a remote-less repo with one branch that main already
// contains (superseded) and one that diverges cleanly.
func brFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	brGit(t, dir, "init", "-q", "-b", "main")
	brGit(t, dir, "config", "user.email", "acs@evolve.local")
	brGit(t, dir, "config", "user.name", "acs")
	brGit(t, dir, "config", "commit.gpgsign", "false")
	brCommit(t, dir, "base.txt", "line1\n", "base commit")
	brGit(t, dir, "branch", "cycle-100")
	brGit(t, dir, "checkout", "-q", "-b", "cycle-200")
	brCommit(t, dir, "feature.txt", "new work\n", "cycle-200 unique commit")
	brGit(t, dir, "checkout", "-q", "main")
	brCommit(t, dir, "base.txt", "line1\nline2\n", "advance main")
	return dir
}

func brBranchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	return exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

func brRun(t *testing.T, dir string, args ...string) (stdout string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runBranches(append(args, "--project-root", dir, "--base", "main"), nil, &out, &errb)
	if errb.Len() > 0 {
		t.Logf("stderr: %s", errb.String())
	}
	return out.String(), code
}

func brLine(stdout, ref string, needles ...string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		if !strings.Contains(line, ref) {
			continue
		}
		ok := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestBranchesAudit_ReportsSupersededAndLandable(t *testing.T) {
	dir := brFixture(t)
	stdout, code := brRun(t, dir, "audit")
	if code != 0 {
		t.Fatalf("audit exit=%d (want 0)\n%s", code, stdout)
	}
	if !brLine(stdout, "cycle-100", "superseded=true", "landable=false") {
		t.Errorf("cycle-100 must report superseded=true landable=false\n%s", stdout)
	}
	if !brLine(stdout, "cycle-200", "superseded=false", "landable=true") {
		t.Errorf("cycle-200 must report superseded=false landable=true\n%s", stdout)
	}
	if !brBranchExists(t, dir, "cycle-100") {
		t.Errorf("audit must be read-only: cycle-100 was deleted")
	}
}

func TestBranchesPruneDryRunDefault_KeepsSuperseded(t *testing.T) {
	dir := brFixture(t)
	stdout, code := brRun(t, dir, "prune")
	if code != 0 {
		t.Fatalf("prune (default) exit=%d (want 0)\n%s", code, stdout)
	}
	if !brBranchExists(t, dir, "cycle-100") {
		t.Errorf("default prune must be dry-run: cycle-100 was deleted")
	}
	if !brLine(stdout, "cycle-100", "would-prune") {
		t.Errorf("default prune must flag cycle-100 as would-prune\n%s", stdout)
	}
}

func TestBranchesPruneForce_DeletesSupersededKeepsDivergent(t *testing.T) {
	dir := brFixture(t)
	stdout, code := brRun(t, dir, "prune", "--dry-run=false")
	if code != 0 {
		t.Fatalf("prune --dry-run=false exit=%d (want 0)\n%s", code, stdout)
	}
	if brBranchExists(t, dir, "cycle-100") {
		t.Errorf("prune --dry-run=false must delete superseded cycle-100\n%s", stdout)
	}
	if !brBranchExists(t, dir, "cycle-200") {
		t.Errorf("prune must NOT delete divergent cycle-200")
	}
}

func TestBranchesUnknownSubcommand_Errors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runBranches([]string{"bogus"}, nil, &out, &errb); code == 0 {
		t.Errorf("unknown subcommand must exit non-zero, got 0")
	}
}

// brKeptFixture builds a remote-less repo whose cycle-1..cycle-4 are all
// superseded by main: cycle-1 is checked out in a linked worktree, cycle-2 is
// named by a continuation binding, and cycle-3 and cycle-4 are free to prune.
func brKeptFixture(t *testing.T) string {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	for _, b := range []string{"cycle-1", "cycle-2", "cycle-3", "cycle-4"} {
		r.Git("branch", b)
	}
	r.Git("commit", "-q", "--allow-empty", "-m", "advance main")
	r.Git("worktree", "add", "-q", filepath.Join(t.TempDir(), "lane"), "cycle-1")
	if err := os.MkdirAll(filepath.Join(r.Dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	bound := continuation.Continuation{Branch: "cycle-2", SnapshotSHA: r.Git("rev-parse", "cycle-2"), Cycle: 2}
	if err := continuation.WriteRegistryEntry(r.Dir, "scope-bound", bound); err != nil {
		t.Fatalf("bind cycle-2: %v", err)
	}
	return r.Dir
}

func brWantLines(t *testing.T, stdout string, want [][2]string) {
	t.Helper()
	for _, w := range want {
		if !brLine(stdout, w[0]+" ", "superseded=true", w[1]) {
			t.Errorf("%s must report superseded=true %s\n%s", w[0], w[1], stdout)
		}
	}
}

func TestBranchesPruneForce_KeepsCheckedOutAndBoundPrunesRest(t *testing.T) {
	dir := brKeptFixture(t)
	stdout, code := brRun(t, dir, "prune", "--dry-run=false")
	if code != 0 {
		t.Fatalf("prune --dry-run=false exit=%d (want 0: a kept ref never aborts the run)\n%s", code, stdout)
	}
	brWantLines(t, stdout, [][2]string{
		{"cycle-1", "kept-checked-out"},
		{"cycle-2", "kept-bound"},
		{"cycle-3", "pruned"},
		{"cycle-4", "pruned"},
	})
	if strings.Contains(stdout, "kept-open-pr") {
		t.Errorf("the repo has no remote, so no ref can be kept behind an open PR\n%s", stdout)
	}
	if !brBranchExists(t, dir, "cycle-1") {
		t.Error("deleted cycle-1, which is checked out in a worktree")
	}
	if !brBranchExists(t, dir, "cycle-2") {
		t.Error("deleted cycle-2, which a continuation binding names")
	}
	if brBranchExists(t, dir, "cycle-3") || brBranchExists(t, dir, "cycle-4") {
		t.Error("left a free superseded ref undeleted")
	}
}

func TestBranchesPruneForce_DeleteFailureReportedPerRef(t *testing.T) {
	dir := brKeptFixture(t)
	// A held ref lock makes git refuse to delete a ref no worktree holds.
	if err := os.WriteFile(filepath.Join(dir, ".git", "refs", "heads", "cycle-3.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := brRun(t, dir, "prune", "--dry-run=false")
	brWantLines(t, stdout, [][2]string{
		{"cycle-1", "kept-checked-out"},
		{"cycle-3", "kept-delete-failed"},
		{"cycle-4", "pruned"},
	})
	if strings.Contains(stdout, "kept-open-pr") {
		t.Errorf("a failed delete was labeled kept-open-pr in a repo with no remote\n%s", stdout)
	}
	if !brBranchExists(t, dir, "cycle-3") {
		t.Error("cycle-3 is gone although its delete was refused")
	}
	if brBranchExists(t, dir, "cycle-4") {
		t.Error("the walk stopped at cycle-3's failed delete instead of pruning cycle-4")
	}
}

func TestBranchesPruneDryRun_KeptReasonsNeverWouldPrune(t *testing.T) {
	dir := brKeptFixture(t)
	stdout, code := brRun(t, dir, "prune")
	if code != 0 {
		t.Fatalf("prune (default) exit=%d (want 0)\n%s", code, stdout)
	}
	brWantLines(t, stdout, [][2]string{
		{"cycle-1", "kept-checked-out"},
		{"cycle-2", "kept-bound"},
		{"cycle-3", "would-prune"},
	})
	for _, b := range []string{"cycle-1", "cycle-2", "cycle-3", "cycle-4"} {
		if !brBranchExists(t, dir, b) {
			t.Errorf("dry-run deleted %s", b)
		}
	}
}
