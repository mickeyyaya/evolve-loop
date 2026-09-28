package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

func TestWorktreeContentSHA_BindsTheTreeTheAuditSealed(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	doc := "docs/explain/builds/cycle-1735-01m3k2r1z13znw2sze4j96xx4k.md"
	writeFile(t, filepath.Join(repo, "base.txt"), "base\ntracked edit\n")
	writeFile(t, filepath.Join(repo, filepath.FromSlash(doc)), "# Why\n")
	ws := t.TempDir()
	writeFile(t, filepath.Join(ws, "build-report.md"), "1. **What files?** `"+doc+"` only.\n")
	ctx := context.Background()
	sealed, err := treefence.Take(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	tree := worktreeContentSHA(ctx, "", repo, ws)
	if tree != sealed.Tree {
		t.Fatalf("the binding %s is not the tree the audit sealed %s (contents: %v): Ship checks the receipt and its staged tree against the binding", tree, sealed.Tree, treeNames(t, repo, tree))
	}
	shipped, err := shipmanifest.TakeShipTree(ctx, shipmanifest.GitIn(ctx, repo), repo, ws)
	if err != nil || shipped.Tree != tree {
		t.Fatalf("the binding is the one ship tree (%v): %s vs %s", err, tree, shipped.Tree)
	}
}

func TestWorktreeContentSHA_WithoutAWorkspaceBindsNothing(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "base.txt"), "base\nedit\n")
	if tree := worktreeContentSHA(context.Background(), "", repo, ""); tree != "" {
		t.Fatalf("without the cycle's reports the ship tree is unknown, so the binding degrades to empty: %s", tree)
	}
}

func TestEmitPhaseBindings_AuditBindingHoldsTheDeclaredUntrackedDeliverable(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	base := gitOut(t, repo, "rev-parse", "HEAD")
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-1735")
	doc := "docs/explain/builds/cycle-1735.md"
	writeFile(t, filepath.Join(ws, "audit-report.md"), "## Verdict\n**PASS**\n")
	writeFile(t, filepath.Join(ws, "build-report.md"), "Wrote `"+doc+"`.\n")
	wt := filepath.Join(t.TempDir(), "cycle-1735")
	gitOut(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	writeFile(t, filepath.Join(wt, filepath.FromSlash(doc)), "# Why\n")
	writeFile(t, filepath.Join(wt, "foreign-residue.txt"), "must not enter the audit binding\n")

	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }
	cs := CycleState{CycleID: 1735, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}
	o.emitPhaseBindings(context.Background(), 1735, repo, cs, PhaseAudit, VerdictPASS)

	if len(led.entries) != 1 {
		t.Fatalf("want exactly 1 auditor binding entry, got %d", len(led.entries))
	}
	tree := led.entries[0].WorktreeTreeSHA
	if !treeHas(t, repo, tree, doc) || treeHas(t, repo, tree, "foreign-residue.txt") {
		t.Fatalf("the production binding holds the declared document and no residue: %v", treeNames(t, repo, tree))
	}
}
