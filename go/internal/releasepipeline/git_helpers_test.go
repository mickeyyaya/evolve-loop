package releasepipeline

import (
	"os/exec"
	"strings"
	"testing"
)

// initTempRepoWithTag creates an isolated git repository in a fresh t.TempDir(),
// makes one commit, and tags it. It returns the repo path. Using an isolated
// repo keeps the test independent of the live repository's tag set — the
// cautionary failure this replaces was a *_ValidGitRepo test that `git describe`'d
// the live worktree and broke when a non-semver tag (pre-consolidation-*) shadowed
// the expected v* tag. Tests must never depend on live-repo or runtime state.
func initTempRepoWithTag(t *testing.T, tag string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		// Quiet, deterministic identity so the commit succeeds in any environment.
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("commit", "--allow-empty", "-m", "initial")
	run("tag", tag)
	return dir
}

// === resolvePrevTag — error branch returns ("", err) =======================

// TestResolvePrevTag_NonGitDir exercises the error path in resolvePrevTag:
// when git -C <dir> describe fails (not a git repo), the function propagates
// the error and the caller falls through to resolveInitCommit.
func TestResolvePrevTag_NonGitDir(t *testing.T) {
	dir := t.TempDir() // plain directory, not a git repo
	tag, err := resolvePrevTag(dir)
	if err == nil {
		t.Errorf("resolvePrevTag in non-git dir: want error, got tag=%q err=nil", tag)
	}
}

// TestResolvePrevTag_ValidGitRepo exercises the success path against an ISOLATED
// temp repo with exactly one known tag, so the assertion is deterministic and
// does not depend on the live repository's tag set.
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

// === resolveInitCommit — error and empty-result branches ===================

// TestResolveInitCommit_NonGitDir exercises the error branch: git rev-list
// fails on a directory that is not a git repository.
func TestResolveInitCommit_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	commit, err := resolveInitCommit(dir)
	if err == nil {
		t.Errorf("resolveInitCommit in non-git dir: want error, got commit=%q err=nil", commit)
	}
}

// TestResolveInitCommit_ValidGitRepo exercises the happy path: a real repo
// returns a non-empty SHA on the first line.
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

// === currentBranch — error branch returns "unknown" ========================

// TestCurrentBranch_NonGitDir exercises the error branch: git symbolic-ref
// fails on a non-git directory, so the function returns "unknown" with nil error.
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

// TestCurrentBranch_ValidGitRepo exercises the success branch: a real repo
// returns a non-empty branch name.
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

// findRepoRoot resolves the enclosing git repository's top level from the
// test's working directory (the package dir). git handles both normal clones
// and linked worktrees. The *_ValidGitRepo / *_RealRepo tests need real git
// history + a buildable module, so the test is skipped when not run inside a
// git checkout (e.g. a source tarball) rather than failing on a path that only
// exists on one machine.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not inside a git checkout (git rev-parse --show-toplevel: %v) — skipping real-repo test", err)
	}
	return strings.TrimSpace(string(out))
}
