//go:build integration

// Cycle-1571 H3 producer half: a FAIL audit verdict emitted NO auditor ledger
// binding (phase_bindings.go guarded to PASS|WARN), so ship's findLatestAudit
// had nothing for this run and fell back to a FOREIGN run's entry — the FAIL
// verdict was the very thing that removed the gate's ability to see it. These
// pins flip the producer: FAIL records the same rich auditor binding (so ship
// reads THIS cycle's report and returns the terminal AUDIT_BINDING_VERDICT_FAIL),
// while the verdict-cache projection stays PASS|WARN-only (the cache exists to
// skip re-audits of known-good trees; caching FAIL would change its consumers'
// contract, and the WARN control below proves the guard is what's observed,
// not an environmental skip).
package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEmitPhaseBindings_AuditFAIL_RecordsBinding_NoCachePut(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-13")
	// Dirty the worktree so the content tree differs from the base tree —
	// ProbeEligible would be TRUE, so only the verdict guard can skip the Put.
	// The delta MUST be a tracked modification: since cycle-1594's declared-
	// content contract, worktreeContentSHA stages `git add -u`, so an untracked
	// file is residue that keeps base identity — it would make this pin pass
	// vacuously via the fresh-base guard instead of the verdict guard.
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("delta"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, repo, "rev-parse", "HEAD")
	cs := CycleState{CycleID: 13, WorkspacePath: ws, ActiveWorktree: repo, WorktreeBaseSHA: base}

	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = fixedNowFn()
	o.emitPhaseBindings(context.Background(), 13, repo, cs, PhaseAudit, VerdictFAIL)

	if len(led.entries) != 1 {
		t.Fatalf("audit FAIL: want 1 auditor binding entry, got %d (%+v)", len(led.entries), led.entries)
	}
	e := led.entries[0]
	if e.Role != "auditor" || e.Kind != "agent_subprocess" {
		t.Errorf("audit FAIL: role/kind = %q/%q, want auditor/agent_subprocess", e.Role, e.Kind)
	}
	// Findings exist and the auditor process did not crash: Unix findings
	// convention (1), same as WARN — ship's 0|1 gate passes and the verdict
	// parse of THIS run's artifact returns the honest VERDICT_FAIL terminal.
	if e.ExitCode != 1 {
		t.Errorf("audit FAIL: exit_code = %d, want 1", e.ExitCode)
	}
	if e.ArtifactSHA256 == "" || e.ArtifactPath != filepath.Join(ws, "audit-report.md") {
		t.Errorf("audit FAIL: artifact binding incomplete: path=%q sha=%q", e.ArtifactPath, e.ArtifactSHA256)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "verdict-cache.json")); !os.IsNotExist(err) {
		t.Errorf("audit FAIL must not be projected into the verdict cache (stat err=%v)", err)
	}
}

// TestEmitPhaseBindings_AuditWARN_CachePut_Control is the positive control for
// the pin above: identical repo shape, WARN verdict → the cache file IS
// written. If this control ever reds, the FAIL pin's no-cache assertion is
// vacuous and must not be trusted.
func TestEmitPhaseBindings_AuditWARN_CachePut_Control(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-14")
	// Tracked modification, not an untracked file — see the FAIL pin above.
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("delta"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := gitOut(t, repo, "rev-parse", "HEAD")
	cs := CycleState{CycleID: 14, WorkspacePath: ws, ActiveWorktree: repo, WorktreeBaseSHA: base}

	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = fixedNowFn()
	o.emitPhaseBindings(context.Background(), 14, repo, cs, PhaseAudit, VerdictWARN)

	if len(led.entries) != 1 {
		t.Fatalf("audit WARN control: want 1 binding entry, got %d", len(led.entries))
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "verdict-cache.json")); err != nil {
		t.Errorf("audit WARN control: verdict cache file expected (guard-vacuity control): %v", err)
	}
}
