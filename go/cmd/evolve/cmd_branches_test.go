package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
