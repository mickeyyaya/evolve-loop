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
	t.Parallel()
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
	t.Parallel()
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

// A modified TRACKED file leaked into main, where the worktree has NOT independently
// touched that file (its copy is still at HEAD), is the real cycle-162 shape: a
// non-Claude builder edited an existing tracked source file (orchestrator.go) in the
// MAIN tree instead of the worktree. The builder's real work must be PRESERVED — the
// leaked content is relocated into the worktree (overwriting its HEAD copy) and the
// main tree restored. Covers the staged-only ("M ") case that `git checkout -- p`
// would no-op.
func TestRecoverBuildLeak_RelocatesTrackedEditWhenWorktreeClean(t *testing.T) {
	t.Parallel()
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
		t.Fatalf("main tree should be clean after recovery; got %q", st)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "base.txt")); string(got) != "base\n" {
		t.Fatalf("main base.txt should be restored to committed content; got %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(wt, "base.txt")); string(got) != "LEAKED\n" {
		t.Fatalf("builder's edit must be relocated into the worktree; got %q", got)
	}
	if diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only"); !strings.Contains(diff, "base.txt") {
		t.Fatalf("relocated tracked edit must be staged/visible to git diff HEAD; got %q", diff)
	}
}

// When the worktree ALSO modified the same tracked file (it diverged from HEAD), the
// main-tree leak is DISCARDED and the worktree's own version is left untouched —
// relocating would clobber legitimate in-worktree work. The worktree is authoritative.
func TestRecoverBuildLeak_DiscardsTrackedEditWhenWorktreeDiverged(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	// The worktree independently edits base.txt (legitimate in-worktree builder work).
	if err := os.WriteFile(filepath.Join(wt, "base.txt"), []byte("WORKTREE-EDIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline := porcelainDirtySet(context.Background(), repo) // main still clean here

	// A conflicting leak of the same file lands in the main tree.
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("MAIN-LEAK\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "base.txt")); string(got) != "base\n" {
		t.Fatalf("main leak should be discarded (restored to HEAD); got %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(wt, "base.txt")); string(got) != "WORKTREE-EDIT\n" {
		t.Fatalf("worktree's own edit must NOT be clobbered; got %q", got)
	}
}

// A gitignored build artifact (e.g. go/evolve) rebuilt into the main tree must NOT be
// treated as a leak: `git status --porcelain -uall` excludes ignored paths, so it never
// reaches recoverBuildLeak's loop — the gitignore IS the build-artifact-discard
// mechanism (no hardcoded path list). The artifact is left in place, untouched, and the
// tracked-only tree-diff guard ignores it too.
func TestRecoverBuildLeak_IgnoresGitignoredArtifact(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("artifact.bin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, repo, "add", ".gitignore")
	gitInRepo(t, repo, "commit", "-q", "-m", "ignore artifact")
	gitInRepo(t, repo, "worktree", "prune") // keep wt valid after the new commit on main
	baseline := porcelainDirtySet(context.Background(), repo)

	if err := os.WriteFile(filepath.Join(repo, "artifact.bin"), []byte("rebuilt binary\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	if _, err := os.Stat(filepath.Join(repo, "artifact.bin")); err != nil {
		t.Fatalf("gitignored artifact must be left untouched in main: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "artifact.bin")); !os.IsNotExist(err) {
		t.Fatalf("gitignored artifact must NOT be relocated into the worktree; stat err=%v", err)
	}
}

// A rebuilt tracked release binary (go/evolve) leaked into main must be DISCARDED even
// when the worktree's copy is at HEAD — relocating it would commit binary drift
// (cycle-153). go/evolve is re-committed only by the release pipeline, never a cycle.
func TestRecoverBuildLeak_DiscardsRebuiltArtifactEvenWhenWorktreeClean(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	if err := os.WriteFile(filepath.Join(repo, "go/evolve"), []byte("OLD BINARY\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, repo, "add", "go/evolve")
	gitInRepo(t, repo, "commit", "-q", "-m", "track go/evolve")
	gitInRepo(t, repo, "worktree", "prune")
	baseline := porcelainDirtySet(context.Background(), repo)

	// Builder rebuilds the binary into the main tree mid-cycle.
	if err := os.WriteFile(filepath.Join(repo, "go/evolve"), []byte("REBUILT BINARY\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak should return true")
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "go/evolve")); string(got) != "OLD BINARY\n" {
		t.Fatalf("rebuilt artifact must be discarded (restored to HEAD); got %q", got)
	}
	if diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only"); strings.Contains(diff, "go/evolve") {
		t.Fatalf("artifact must NOT be relocated/staged into the worktree; git diff HEAD=%q", diff)
	}
}

// recoverBuildLeak must SKIP the orchestrator's own runtime state under .evolve/ —
// it is never build output. In the live repo .evolve/ is gitignored (invisible to
// `git status`); a minimal fixture without that .gitignore exposed recoverBuildLeak
// (a) relocating .evolve/ledger.tip into the worktree (pollutes the audit diff) and
// (b) CHOKING on the nested worktree dir .evolve/worktrees/cycle-1/ — `git status
// -uall` reports a nested working tree as a bare directory, which moveFile cannot
// relocate, so it returned false and aborted the cycle (415a9a7 regression caught by
// the e2e ship-path tests). Both must be skipped; the cycle proceeds.
func TestRecoverBuildLeak_SkipsEvolveRuntimeStateAndNestedWorktreeDir(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	baseline := porcelainDirtySet(context.Background(), repo) // clean

	if err := os.MkdirAll(filepath.Join(repo, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".evolve/ledger.tip"), []byte("tip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real nested worktree — `git status -uall` reports it as a bare dir (no recurse).
	gitInRepo(t, repo, "worktree", "add", "--detach", "-q", filepath.Join(repo, ".evolve/worktrees/cycle-1"), "HEAD")

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak must skip .evolve/ runtime state + the nested-worktree dir and return true, not abort")
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve/ledger.tip")); err != nil {
		t.Fatalf(".evolve/ledger.tip must be left untouched in main: %v", err)
	}
	if diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only"); strings.Contains(diff, ".evolve") {
		t.Fatalf(".evolve/ runtime state must NOT be relocated/staged into the worktree; git diff HEAD=%q", diff)
	}
}

// Issue #11 (cycle-176): guard hooks run with cwd set to subdirectories and write
// NESTED `<subdir>/.evolve/guards.log`. The top-level-only skip missed these, so
// recoverBuildLeak relocated them and the gitignored `git add` failed → batch abort.
// Nested `.evolve/` paths (path contains `/.evolve/`) must be skipped like top-level.
func TestRecoverBuildLeak_SkipsNestedEvolveRuntimeState(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	baseline := porcelainDirtySet(context.Background(), repo) // clean

	// Nested .evolve/ runtime state under tracked subdirs (mirrors cycle-176).
	for _, d := range []string{"go", "go/internal/phases"} {
		if err := os.MkdirAll(filepath.Join(repo, d, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, d, ".evolve/guards.log"), []byte("guard\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if !recoverBuildLeak(context.Background(), repo, wt, baseline) {
		t.Fatal("recoverBuildLeak must SKIP nested .evolve/ runtime state and return true, not abort")
	}
	// Left in place (not relocated into the worktree).
	if _, err := os.Stat(filepath.Join(repo, "go/.evolve/guards.log")); err != nil {
		t.Fatalf("nested go/.evolve/guards.log must be left untouched in main: %v", err)
	}
	if diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only"); strings.Contains(diff, ".evolve") {
		t.Fatalf("nested .evolve/ runtime state must NOT be relocated/staged into the worktree; git diff HEAD=%q", diff)
	}
}

// Pre-existing operator dirt (in the baseline) is left untouched; only build-introduced leaks move.
func TestRecoverBuildLeak_LeavesBaselineDirtUntouched(t *testing.T) {
	t.Parallel()
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
