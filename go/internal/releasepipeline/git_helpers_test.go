package releasepipeline

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func initTempRepoWithTag(t *testing.T, tag string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := gittest.Fixture(t).Dir
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("commit", "--allow-empty", "-m", "initial")
	run("tag", tag)
	return dir
}

func TestResolvePrevTag_NonGitDir(t *testing.T) {
	nonGitDir := t.TempDir()
	tag, err := resolvePrevTag(nonGitDir)
	if err == nil {
		t.Errorf("resolvePrevTag in non-git dir: want error, got tag=%q err=nil", tag)
	}
}

func TestResolvePrevTag_ValidGitRepo(t *testing.T) {
	repo := initTempRepoWithTag(t, "v1.2.3")
	tag, err := resolvePrevTag(repo)
	if err != nil {
		t.Fatalf("resolvePrevTag on tagged repo: unexpected error %v", err)
	}
	if tag != "v1.2.3" {
		t.Errorf("resolvePrevTag = %q, want %q", tag, "v1.2.3")
	}
}

func TestResolveInitCommit_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	commit, err := resolveInitCommit(dir)
	if err == nil {
		t.Errorf("resolveInitCommit in non-git dir: want error, got commit=%q err=nil", commit)
	}
}

func TestResolveInitCommit_ValidGitRepo(t *testing.T) {
	repo := findRepoRoot(t)
	commit, err := resolveInitCommit(repo)
	if err != nil {
		t.Fatalf("resolveInitCommit: %v", err)
	}
	if len(commit) < 7 {
		t.Errorf("resolveInitCommit = %q, want a full or abbreviated SHA (>=7 chars)", commit)
	}
}

func TestCurrentBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	branch, err := currentBranch(dir)
	if err != nil {
		t.Errorf("currentBranch error branch should return nil, got %v", err)
	}
	if branch != "unknown" {
		t.Errorf("currentBranch error branch = %q, want %q", branch, "unknown")
	}
}

func TestCurrentBranch_ValidGitRepo(t *testing.T) {
	repo := findRepoRoot(t)
	branch, err := currentBranch(repo)
	if err != nil {
		t.Errorf("currentBranch should swallow errors, got %v", err)
	}
	if branch == "" {
		t.Error("currentBranch = empty, want a branch name or 'unknown'")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not inside a git checkout (git rev-parse --show-toplevel: %v) — skipping real-repo test", err)
	}
	return strings.TrimSpace(string(out))
}
