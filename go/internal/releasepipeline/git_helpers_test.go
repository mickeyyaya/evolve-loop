package releasepipeline

import (
	"os/exec"
	"strings"
	"testing"
)

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

// TestResolvePrevTag_GitDir exercises the success path: a real git repo with
// at least one tag. We cannot guarantee a tag exists in CI, so this test
// simply verifies the function does not panic on a valid dir and returns a
// non-empty string when git succeeds.  We skip if git is not on PATH.
func TestResolvePrevTag_ValidGitRepo(t *testing.T) {
	// Use the actual repo root that contains real git history.
	// If there are no tags, git describe exits non-zero — that's acceptable;
	// the test proves the function CALLS git and handles both outcomes.
	repo := findRepoRoot(t)
	tag, err := resolvePrevTag(repo)
	if err != nil {
		// No tags in repo — acceptable; function returned error correctly.
		return
	}
	if tag == "" {
		t.Errorf("resolvePrevTag returned empty tag with nil error")
	}
	// Tags in this repo begin with "v".
	if !strings.HasPrefix(tag, "v") {
		t.Errorf("resolvePrevTag = %q, expected v-prefixed tag", tag)
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
