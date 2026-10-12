//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

func auditBindingOrchestratorE4(ledger Ledger) *Orchestrator {
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil))
	o.now = func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC) }
	return o
}

func editedLaneWorktreeE4(t *testing.T) (projectRoot, worktree, base string) {
	t.Helper()
	projectRoot = putBaseRepo(t)
	base = gitOut(t, projectRoot, "rev-parse", "HEAD")
	worktree = putBaseClone(t, projectRoot)
	writeFile(t, filepath.Join(worktree, "base.txt"), "lane edit")
	return projectRoot, worktree, base
}

func TestRecordAuditBinding_AnUnreadableProjectHeadBindsNothing(t *testing.T) {
	ledger := &fakeLedger{}
	o := auditBindingOrchestratorE4(ledger)
	missing := filepath.Join(t.TempDir(), "gone")

	log := captureStderr(t, func() {
		o.recordAuditBinding(context.Background(), 7, missing, putBaseWorkspace(t), "", "", VerdictPASS)
	})

	if !strings.Contains(log, "WARN audit-binding: git rev-parse HEAD failed: ") {
		t.Fatalf("stderr must name the HEAD failure:\n%s", log)
	}
	if len(ledger.entries) != 0 {
		t.Fatalf("no auditor row may be bound without a HEAD: %+v", ledger.entries)
	}
}

func TestRecordAuditBinding_ALostLedgerRowStillCachesTheReusableVerdict(t *testing.T) {
	projectRoot, worktree, base := editedLaneWorktreeE4(t)
	ws := putBaseWorkspace(t)
	o := auditBindingOrchestratorE4(&fakeLedger{failOnAppend: true})

	log := captureStderr(t, func() {
		o.recordAuditBinding(context.Background(), 8, projectRoot, ws, worktree, base, VerdictPASS)
	})

	if !strings.Contains(log, "WARN audit-binding ledger append: forced ledger append fail") {
		t.Fatalf("stderr must surface the lost binding row:\n%s", log)
	}
	entries, err := verdictcache.NewStore(projectRoot, o.now).Load()
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache = %+v (%v), want the PASS verdict cached despite the ledger failure", entries, err)
	}
	for _, e := range entries {
		if e.Cycle != 8 || e.Verdict != VerdictPASS {
			t.Fatalf("cached entry = %+v", e)
		}
	}
}

func TestRecordAuditBinding_AnUnresolvableBaseSkipsTheCache(t *testing.T) {
	projectRoot, worktree, _ := editedLaneWorktreeE4(t)
	ledger := &fakeLedger{}
	o := auditBindingOrchestratorE4(ledger)

	log := captureStderr(t, func() {
		o.recordAuditBinding(context.Background(), 9, projectRoot, putBaseWorkspace(t), worktree, strings.Repeat("d", 40), VerdictPASS)
	})

	if !strings.Contains(log, "WARN verdict-cache put skipped (cycle 9): worktree base "+strings.Repeat("d", 40)+" did not resolve to a tree") {
		t.Fatalf("stderr must name the unresolved base:\n%s", log)
	}
	if len(ledger.entries) != 1 || ledger.entries[0].WorktreeBaseSHA != strings.Repeat("d", 40) {
		t.Fatalf("the binding row must still be written: %+v", ledger.entries)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".evolve", "verdict-cache.json")); !os.IsNotExist(err) {
		t.Fatalf("no cache entry may be written for an unresolved base: %v", err)
	}
}

func TestRecordAuditBinding_AnUnwritableCacheWarnsAfterBinding(t *testing.T) {
	projectRoot, worktree, base := editedLaneWorktreeE4(t)
	if err := os.MkdirAll(filepath.Join(projectRoot, ".evolve", "verdict-cache.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := &fakeLedger{}
	o := auditBindingOrchestratorE4(ledger)

	log := captureStderr(t, func() {
		o.recordAuditBinding(context.Background(), 10, projectRoot, putBaseWorkspace(t), worktree, base, VerdictWARN)
	})

	if !strings.Contains(log, "WARN verdict-cache put: ") {
		t.Fatalf("stderr must surface the failed cache write:\n%s", log)
	}
	if len(ledger.entries) != 1 || ledger.entries[0].Role != "auditor" || ledger.entries[0].WorktreeTreeSHA == "" {
		t.Fatalf("the binding row must precede the cache write: %+v", ledger.entries)
	}
}

func TestNormalizeWorktreeToBase_AnUnreadableHeadLeavesTheWorktreeAsIs(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")

	log := captureStderr(t, func() { normalizeWorktreeToBase(context.Background(), missing, strings.Repeat("a", 40)) })

	if !strings.Contains(log, "WARN worktree-normalize: rev-parse HEAD failed: ") {
		t.Fatalf("stderr must name the HEAD failure:\n%s", log)
	}
}

func TestNormalizeWorktreeToBase_ALockedBranchKeepsTheBuilderCommit(t *testing.T) {
	root := putBaseRepo(t)
	base := gitOut(t, root, "rev-parse", "HEAD")
	writeFile(t, filepath.Join(root, "built.txt"), "builder output")
	gitOut(t, root, "add", "built.txt")
	gitOut(t, root, "commit", "-q", "-m", "builder commit")
	head := gitOut(t, root, "rev-parse", "HEAD")
	branch := gitOut(t, root, "symbolic-ref", "--short", "HEAD")
	lock := filepath.Join(root, ".git", "refs", "heads", branch+".lock")
	writeFile(t, lock, "")

	log := captureStderr(t, func() { normalizeWorktreeToBase(context.Background(), root, base) })

	if !strings.Contains(log, "WARN worktree-normalize: git reset --soft "+base+" failed (rc=1)") {
		t.Fatalf("stderr must name the failed soft reset:\n%s", log)
	}
	if got := gitOut(t, root, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD = %s, want the builder commit %s kept as-is", got, head)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() { normalizeWorktreeToBase(context.Background(), root, base) })
	if got := gitOut(t, root, "rev-parse", "HEAD"); got != base {
		t.Fatalf("control: an unlocked branch soft-resets to the base, HEAD = %s", got)
	}
}

func TestPruneSupersededOrphans_EachGitFailureAbortsWithItsStage(t *testing.T) {
	t.Parallel()

	noPR := func(string) (bool, error) { return false, nil }
	t.Run("git cannot start", func(t *testing.T) {
		v, err := PruneSupersededOrphans(context.Background(), filepath.Join(t.TempDir(), "gone"), "main", noPR)
		if err == nil || !strings.HasPrefix(err.Error(), "prune orphans: list cycle-* refs: ") || v != nil {
			t.Fatalf("prune = (%v, %v)", v, err)
		}
	})
	t.Run("not a repository", func(t *testing.T) {
		v, err := PruneSupersededOrphans(context.Background(), t.TempDir(), "main", noPR)
		if err == nil || err.Error() != "prune orphans: for-each-ref exited 128" || v != nil {
			t.Fatalf("prune = (%v, %v)", v, err)
		}
	})
	t.Run("an unknown base", func(t *testing.T) {
		root := putBaseRepo(t)
		gitOut(t, root, "branch", "cycle-5")
		v, err := PruneSupersededOrphans(context.Background(), root, "no-such-base", noPR)
		if err == nil || !strings.Contains(err.Error(), "git cherry no-such-base cycle-5 exited") || v != nil {
			t.Fatalf("prune = (%v, %v)", v, err)
		}
		if got := gitOut(t, root, "branch", "--list", "cycle-5"); !strings.Contains(got, "cycle-5") {
			t.Fatal("an aborted prune must delete nothing")
		}
	})
}

func TestKeepOrPrune_ADeleteThatCannotStartIsAnError(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gone")

	v, err := keepOrPrune(context.Background(), missing, OrphanVerdict{Ref: "cycle-3", Superseded: true}, false, false, func(string) (bool, error) { return false, nil })

	if err == nil || !strings.HasPrefix(err.Error(), "prune orphans: delete cycle-3: ") {
		t.Fatalf("err = %v, want the delete failure named", err)
	}
	if v.Pruned || v.KeptDeleteFailed || v.Ref != "cycle-3" {
		t.Fatalf("verdict = %+v, want neither pruned nor kept-on-exit", v)
	}
}

func TestCheckedOutBranches_AGitThatCannotStartIsAnError(t *testing.T) {
	t.Parallel()

	got, err := checkedOutBranches(context.Background(), filepath.Join(t.TempDir(), "gone"))

	if err == nil || !strings.HasPrefix(err.Error(), "prune orphans: list worktrees: ") || got != nil {
		t.Fatalf("checkedOutBranches = (%v, %v)", got, err)
	}
}
