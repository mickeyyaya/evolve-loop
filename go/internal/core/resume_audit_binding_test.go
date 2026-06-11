// Regression test for the cycle-294 resume incident (2026-06-12): ship's
// verifyAuditBinding reads the latest role=auditor kind=agent_subprocess
// ledger entry. RunCycle emits it after a shippable audit, but the resume
// path (RunCycleFromPhase) did not — so a resumed audit→ship always bound to
// a stale entry from an earlier cycle and failed AUDIT_BINDING_HEAD_MOVED.
package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initBindingRepo creates an ephemeral git repo with one commit plus a
// cycle workspace holding an audit-report.md, mirroring what a resumed
// audit phase leaves on disk.
func initBindingRepo(t *testing.T, cycle string) (repo, ws string) {
	t.Helper()
	repo = t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "f.txt")
	runGit("commit", "-q", "-m", "init")

	ws = filepath.Join(repo, ".evolve", "runs", cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("## Verdict\n**PASS**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, ws
}

// TestRunCycleFromPhase_EmitsAuditBinding — resuming from PhaseAudit must
// append the same rich auditor binding entry RunCycle does, bound to the
// CURRENT git HEAD, or ship cannot verify the resumed audit.
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

// TestRunCycleFromPhase_EmitsBuildBinding — resuming from PhaseBuild must
// append the builder provenance entry (role=builder, kind=agent_subprocess)
// that rt-001-ledger-role-completeness + the auditor's Ledger-Verification
// require, same as RunCycle.
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
