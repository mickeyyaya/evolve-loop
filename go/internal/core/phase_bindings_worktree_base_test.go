package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestEmitPhaseBindings_TheAuditRowNamesTheBaseTheAuditedWorktreeStoodOn(t *testing.T) {
	t.Parallel()
	repo := stagingScopeRepo(t)
	base := gitOut(t, repo, "rev-parse", "HEAD")
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-1729")
	writeFile(t, filepath.Join(ws, "audit-report.md"), "## Verdict\n**PASS**\n")
	wt := filepath.Join(t.TempDir(), "cycle-1729")
	gitOut(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	gitOut(t, repo, "commit", "-q", "--allow-empty", "-m", "a sibling lane shipped")
	mainHead := gitOut(t, repo, "rev-parse", "HEAD")

	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }
	cs := CycleState{CycleID: 1729, WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: base}
	o.emitPhaseBindings(context.Background(), 1729, repo, cs, PhaseAudit, VerdictPASS)

	if len(led.entries) != 1 {
		t.Fatalf("want exactly 1 auditor binding entry, got %d (%+v)", len(led.entries), led.entries)
	}
	row := led.entries[0]
	if row.WorktreeBaseSHA != base || row.GitHEAD != mainHead {
		t.Errorf("auditor row base=%q head=%q, want the worktree's base %q beside main's head %q", row.WorktreeBaseSHA, row.GitHEAD, base, mainHead)
	}
}
