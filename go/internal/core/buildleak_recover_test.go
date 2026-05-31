package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildleak_recover_test.go — Option A for the cycle-160 incident
// (docs/operations/multicli-validation-run-2026-05-31.md §"Implementation plan for A").
//
// A non-Claude builder (agy/codex in tmux) is not bound by the Claude-only role-gate,
// and the OS sandbox is off on nested-macOS, so it can write its build output to the
// MAIN tree instead of its worktree. recoverBuildLeak relocates that leaked output into
// the worktree (staging ONLY the relocated paths, so the auditor's `git diff HEAD` sees
// it without pollution) and restores the main tree.
//
// These tests use a REAL `git worktree add` (not two independent repos) so the worktree
// shares the main repo's tracked directory structure — the production topology where an
// earlier independent-repo test masked a directory-rename bug.

// realWorktree provisions repo (one base commit + a nested tracked dir) and a linked
// worktree off it, returning (repo, worktree).
func realWorktree(t *testing.T) (string, string) {
	t.Helper()
	repo, _ := newRepoWithBaseCommit(t)
	// A tracked nested dir so the worktree contains it too (mirrors a real checkout).
	if err := os.MkdirAll(filepath.Join(repo, "go/internal/phases"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go/internal/phases/registry.go"), []byte("package phases\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, repo, "add", "-A")
	gitInRepo(t, repo, "commit", "-q", "-m", "phases")
	wt := filepath.Join(t.TempDir(), "wt")
	gitInRepo(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	return repo, wt
}

// Relocate a leaked NEW file written into an EXISTING tracked directory in main —
// the real cycle-160 shape (agy wrote go/internal/phases/backfill/* into main).
func TestRecoverBuildLeak_RelocatesIntoRealWorktree(t *testing.T) {
	repo, wt := realWorktree(t)
	baseline := porcelainDirtySet(context.Background(), repo) // clean

	leakDir := filepath.Join(repo, "go/internal/phases/backfill")
	if err := os.MkdirAll(leakDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leakDir, "backfill.go"), []byte("package backfill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	if st := gitInRepo(t, repo, "status", "--porcelain", "-uall"); st != "" {
		t.Fatalf("main tree should be clean after recovery; got %q", st)
	}
	if _, err := os.Stat(filepath.Join(wt, "go/internal/phases/backfill/backfill.go")); err != nil {
		t.Fatalf("leaked file should be relocated into the worktree: %v", err)
	}
	if diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only"); !strings.Contains(diff, "go/internal/phases/backfill/backfill.go") {
		t.Fatalf("relocated file must be staged/visible to git diff HEAD; got %q", diff)
	}
}

// Staging must be SCOPED to the relocated paths — pre-existing untracked worktree
// content must NOT be swept into the audit's `git diff HEAD` (CRITICAL: not `git add -A`).
func TestRecoverBuildLeak_StagesOnlyRelocatedPaths(t *testing.T) {
	repo, wt := realWorktree(t)
	// Pre-existing untracked leftover already in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "leftover.txt"), []byte("not part of this build\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline := porcelainDirtySet(context.Background(), repo)
	if err := os.WriteFile(filepath.Join(repo, "leaked.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only")
	if !strings.Contains(diff, "leaked.go") {
		t.Fatalf("relocated leaked.go must be staged; git diff HEAD=%q", diff)
	}
	if strings.Contains(diff, "leftover.txt") {
		t.Fatalf("pre-existing untracked worktree file must NOT be staged into the audit view; git diff HEAD=%q", diff)
	}
}

// A modified TRACKED file leaked into main (e.g. a rebuilt go/evolve) is discarded,
// including the staged-only ("M ") case that `git checkout -- p` would no-op.
func TestRecoverBuildLeak_DiscardsModifiedTracked(t *testing.T) {
	repo, wt := realWorktree(t)
	baseline := porcelainDirtySet(context.Background(), repo)

	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("LEAKED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, repo, "add", "base.txt") // staged-only ("M ") — the case checkout -- would miss

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	if st := gitInRepo(t, repo, "status", "--porcelain", "-uall"); st != "" {
		t.Fatalf("main tree should be clean after discard; got %q", st)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "base.txt")); string(got) != "base\n" {
		t.Fatalf("base.txt should be restored to committed content; got %q", got)
	}
}

// Pre-existing operator dirt (in the baseline) is left untouched; only build-introduced leaks move.
func TestRecoverBuildLeak_LeavesBaselineDirtUntouched(t *testing.T) {
	repo, wt := realWorktree(t)
	if err := os.WriteFile(filepath.Join(repo, "preexisting.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline := porcelainDirtySet(context.Background(), repo) // includes preexisting.txt

	if err := os.WriteFile(filepath.Join(repo, "new_leak.txt"), []byte("leaked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	if _, err := os.Stat(filepath.Join(repo, "preexisting.txt")); err != nil {
		t.Fatalf("pre-existing operator file must NOT be touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "new_leak.txt")); !os.IsNotExist(err) {
		t.Fatalf("new leak should have been relocated out of main; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "new_leak.txt")); err != nil {
		t.Fatalf("new leak should be in the worktree: %v", err)
	}
}
