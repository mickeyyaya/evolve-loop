package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stagingScopeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitOut(t, repo, "init", "-q")
	gitOut(t, repo, "config", "user.email", "test@test")
	gitOut(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, repo, "add", "base.txt", ".gitignore")
	gitOut(t, repo, "commit", "-q", "-m", "base")
	return repo
}

func treeNames(t *testing.T, repo, tree string) []string {
	t.Helper()
	out := gitOut(t, repo, "ls-tree", "-r", "--name-only", tree)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func treeHas(t *testing.T, repo, tree, name string) bool {
	t.Helper()
	for _, got := range treeNames(t, repo, tree) {
		if got == name {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreeContentSHA_ExcludesUnrelatedUntrackedResidue(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "base.txt"), "base\ndeclared edit\n")
	writeFile(t, filepath.Join(repo, "foreign-residue.txt"), "must not enter the audit binding\n")

	tree := worktreeContentSHA(context.Background(), "", repo, t.TempDir())
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree — the identity must still resolve")
	}
	if treeHas(t, repo, tree, "foreign-residue.txt") {
		t.Errorf("unrelated untracked residue entered binding tree %s (contents: %v) — "+
			"the audit binding and the verdict-cache key would attest content the auditor never reviewed",
			tree, treeNames(t, repo, tree))
	}
}

func TestWorktreeContentSHA_StagesDeclaredNewFile(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	declared := filepath.Join("go", "internal", "newpkg", "newpkg.go")
	writeFile(t, filepath.Join(repo, declared), "package newpkg\n")
	gitOut(t, repo, "add", declared)

	tree := worktreeContentSHA(context.Background(), "", repo, t.TempDir())
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree")
	}
	if !treeHas(t, repo, tree, filepath.ToSlash(declared)) {
		t.Errorf("declared new file %s missing from binding tree %s (contents: %v) — "+
			"scoping staging must not drop the builder's own output",
			declared, tree, treeNames(t, repo, tree))
	}
}

func TestWorktreeContentSHA_CapturesUnstagedTrackedModification(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "base.txt"), "base\nunstaged tracked edit\n")

	tree := worktreeContentSHA(context.Background(), "", repo, t.TempDir())
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree")
	}
	baseTree := gitOut(t, repo, "rev-parse", "HEAD^{tree}")
	if tree == baseTree {
		t.Fatalf("binding tree %s equals the base tree — an unstaged tracked modification was dropped, "+
			"so every dirty lane would bind a base-identical (and cross-lane shared) identity", tree)
	}
	if got := gitOut(t, repo, "show", tree+":base.txt"); !strings.Contains(got, "unstaged tracked edit") {
		t.Errorf("binding tree %s carries stale base.txt content %q — tracked worktree edits must be captured", tree, got)
	}
}

func TestWorktreeContentSHA_ResidueOnlyWorktreeKeepsBaseIdentity(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "coverage.core476.func.txt"), "regenerated build output\n")
	writeFile(t, filepath.Join(repo, ".evolve-scratch-residue.txt"), "another lane's scratch\n")

	tree := worktreeContentSHA(context.Background(), "", repo, t.TempDir())
	if tree == "" {
		t.Fatal("worktreeContentSHA returned an empty tree — residue-only must still resolve an identity")
	}
	if baseTree := gitOut(t, repo, "rev-parse", "HEAD^{tree}"); tree != baseTree {
		t.Errorf("residue-only worktree resolved identity %s, want the base tree %s (contents: %v) — "+
			"an empty declared selection must not silently adopt residue",
			tree, baseTree, treeNames(t, repo, tree))
	}
}

func TestEmitPhaseBindings_AuditBindingTreeExcludesUnrelatedResidue(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	base := gitOut(t, repo, "rev-parse", "HEAD")
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-1594")
	writeFile(t, filepath.Join(ws, "audit-report.md"), "## Verdict\n**PASS**\n")
	wt := filepath.Join(t.TempDir(), "cycle-1594")
	gitOut(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")

	declared := filepath.Join("go", "internal", "newpkg", "newpkg.go")
	writeFile(t, filepath.Join(wt, declared), "package newpkg\n")
	gitOut(t, wt, "add", declared)
	writeFile(t, filepath.Join(wt, "foreign-residue.txt"), "must not enter the audit binding\n")

	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }
	cs := CycleState{CycleID: 1594, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}
	o.emitPhaseBindings(context.Background(), 1594, repo, cs, PhaseAudit, VerdictPASS)

	if len(led.entries) != 1 {
		t.Fatalf("want exactly 1 auditor binding entry, got %d (%+v)", len(led.entries), led.entries)
	}
	tree := led.entries[0].WorktreeTreeSHA
	if tree == "" {
		t.Fatal("audit binding recorded an empty worktree_tree_sha — ship cannot bind the audited content")
	}
	if treeHas(t, repo, tree, "foreign-residue.txt") {
		t.Errorf("audit binding %s attests unrelated untracked residue (contents: %v) — "+
			"the scoped selection is not wired into the production binding path",
			tree, treeNames(t, repo, tree))
	}
	if !treeHas(t, repo, tree, filepath.ToSlash(declared)) {
		t.Errorf("audit binding %s dropped declared builder output %s (contents: %v)",
			tree, declared, treeNames(t, repo, tree))
	}
}
