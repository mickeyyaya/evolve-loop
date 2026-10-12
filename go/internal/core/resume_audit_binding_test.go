//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

// initBindingRepo creates an ephemeral git repo with one commit plus a
// cycle workspace holding an audit-report.md, mirroring what a resumed
// audit phase leaves on disk.
func buildBindingRepo(r *gittest.Repo) error {
	r.Write("f.txt", "x")
	r.Git("add", "f.txt")
	r.Git("commit", "-q", "-m", "init")
	return nil
}

func seedBindingWorkspace(t *testing.T, repo, cycle string) string {
	t.Helper()
	ws := filepath.Join(repo, ".evolve", "runs", cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("## Verdict\n**PASS**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func initBindingRepo(t *testing.T, cycle string) (repo, ws string) {
	t.Helper()
	repo = gittest.FromTemplate(t, "core-binding-repo", buildBindingRepo).Dir
	return repo, seedBindingWorkspace(t, repo, cycle)
}

func TestRunCycleFromPhase_EmitsAuditBinding(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-7")
	st := &fakeStorage{
		state:      State{LastCycleNumber: 7},
		cycleState: CycleState{CycleID: 7, WorkspacePath: ws},
	}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: repo,
	}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 7}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	var bind *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Role == "auditor" && led.entries[i].Kind == "agent_subprocess" {
			bind = &led.entries[i]
		}
	}
	if bind == nil {
		t.Fatalf("resume path wrote no role=auditor kind=agent_subprocess binding entry; got %+v", led.entries)
	}
	if len(bind.GitHEAD) != 40 {
		t.Errorf("git_head=%q, want a 40-char SHA bound to the resumed-audit HEAD", bind.GitHEAD)
	}
	if bind.Cycle != 7 {
		t.Errorf("cycle=%d, want 7", bind.Cycle)
	}
}

func TestRunCycleFromPhase_EmitsBuildBinding(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-8")
	st := &fakeStorage{
		state:      State{LastCycleNumber: 8},
		cycleState: CycleState{CycleID: 8, WorkspacePath: ws},
	}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: repo,
	}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 8}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	found := false
	for i := range led.entries {
		if led.entries[i].Role == "builder" && led.entries[i].Kind == "agent_subprocess" {
			found = true
		}
	}
	if !found {
		t.Errorf("resume path wrote no role=builder kind=agent_subprocess provenance entry; got %+v", led.entries)
	}
}
