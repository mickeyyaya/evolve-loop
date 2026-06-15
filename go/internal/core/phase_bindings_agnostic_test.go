//go:build integration

// Phase 3.9 — phase-agnostic ledger binding. These tests pin (a) that the new
// recordPhaseBinding dispatcher routes audit/build to their specialized,
// byte-unchanged recorders with a faithful CycleState→bindingInputs mapping
// (Risk #1: ship's auditor/builder binding bytes must not drift), and (b) the
// NEW phase-agnostic capability — any non-audit/non-build phase binds under its
// identity role, gated to EVOLVE_PHASE_IO=enforce so the default-off loop is
// byte-identical (no new ledger lines). They reuse initBindingRepo from
// resume_audit_binding_test.go (same package, same integration tag).
package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func fixedNowFn() func() time.Time {
	return func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
}

// TestEmitPhaseBindings_AuditBuild_CorrectMapping proves the new path
// (emitPhaseBindings → recordPhaseBinding → recordAuditBinding/recordBuildBinding)
// maps CycleState into the specialized recorders correctly: the audit binding
// reads the WORKTREE (ActiveWorktree) for its worktree-tree SHA and the WORKSPACE
// (WorkspacePath) for its audit-report.md artifact, with verdict-derived exit
// code; the build binding uses the workspace for build-report.md and never
// computes a worktree-tree SHA.
func TestEmitPhaseBindings_AuditBuild_CorrectMapping(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-9")
	cs := CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: repo}

	// --- audit, PASS ---
	ledA := &fakeLedger{}
	oA := NewOrchestrator(nil, ledA, nil)
	oA.now = fixedNowFn()
	oA.emitPhaseBindings(context.Background(), 9, repo, cs, PhaseAudit, VerdictPASS)
	if len(ledA.entries) != 1 {
		t.Fatalf("audit: want 1 binding entry, got %d (%+v)", len(ledA.entries), ledA.entries)
	}
	a := ledA.entries[0]
	if a.Role != "auditor" || a.Kind != "agent_subprocess" {
		t.Errorf("audit: role/kind = %q/%q, want auditor/agent_subprocess", a.Role, a.Kind)
	}
	if a.ExitCode != 0 {
		t.Errorf("audit PASS: exit_code = %d, want 0", a.ExitCode)
	}
	if a.ArtifactPath != filepath.Join(ws, "audit-report.md") {
		t.Errorf("audit: artifact_path = %q, want %q", a.ArtifactPath, filepath.Join(ws, "audit-report.md"))
	}
	if len(a.GitHEAD) != 40 {
		t.Errorf("audit: git_head = %q, want a 40-char SHA", a.GitHEAD)
	}
	if a.WorktreeTreeSHA == "" {
		t.Errorf("audit: worktree_tree_sha empty — ActiveWorktree was not mapped to the worktree recorder")
	}

	// --- audit, WARN → exit_code 1 ---
	ledW := &fakeLedger{}
	oW := NewOrchestrator(nil, ledW, nil)
	oW.now = fixedNowFn()
	oW.emitPhaseBindings(context.Background(), 9, repo, cs, PhaseAudit, VerdictWARN)
	if len(ledW.entries) != 1 || ledW.entries[0].ExitCode != 1 {
		t.Errorf("audit WARN: want 1 entry with exit_code 1, got %+v", ledW.entries)
	}

	// --- build, PASS ---
	ledB := &fakeLedger{}
	oB := NewOrchestrator(nil, ledB, nil)
	oB.now = fixedNowFn()
	oB.emitPhaseBindings(context.Background(), 9, repo, cs, PhaseBuild, VerdictPASS)
	if len(ledB.entries) != 1 {
		t.Fatalf("build: want 1 binding entry, got %d (%+v)", len(ledB.entries), ledB.entries)
	}
	b := ledB.entries[0]
	if b.Role != "builder" || b.Kind != "agent_subprocess" {
		t.Errorf("build: role/kind = %q/%q, want builder/agent_subprocess", b.Role, b.Kind)
	}
	if b.ArtifactPath != filepath.Join(ws, "build-report.md") {
		t.Errorf("build: artifact_path = %q, want %q", b.ArtifactPath, filepath.Join(ws, "build-report.md"))
	}
	if b.WorktreeTreeSHA != "" {
		t.Errorf("build: worktree_tree_sha = %q, want empty (build never computes it)", b.WorktreeTreeSHA)
	}
	if len(b.GitHEAD) != 40 {
		t.Errorf("build: git_head = %q, want a 40-char SHA", b.GitHEAD)
	}
}

// TestEmitPhaseBindings_UserPhase_OnlyAtEnforce is the master gate test: a
// non-audit/non-build phase produces NO binding below enforce (default-off is
// byte-identical to pre-3.9 — no new ledger lines in the live loop), while
// audit/build keep binding as before; at EVOLVE_PHASE_IO=enforce the user phase
// binds under its identity role with a builder-shaped generic entry.
func TestEmitPhaseBindings_UserPhase_OnlyAtEnforce(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-11")
	cs := CycleState{CycleID: 11, WorkspacePath: ws, ActiveWorktree: repo}

	// default (StageOff): user phase scout does NOT bind; audit still binds.
	ledOff := &fakeLedger{}
	oOff := NewOrchestrator(nil, ledOff, nil)
	oOff.now = fixedNowFn()
	oOff.emitPhaseBindings(context.Background(), 11, repo, cs, PhaseScout, VerdictPASS)
	if len(ledOff.entries) != 0 {
		t.Errorf("PhaseIO=off: user phase scout must not bind, got %d entries (%+v)", len(ledOff.entries), ledOff.entries)
	}
	oOff.emitPhaseBindings(context.Background(), 11, repo, cs, PhaseAudit, VerdictPASS)
	if len(ledOff.entries) != 1 || ledOff.entries[0].Role != "auditor" {
		t.Errorf("PhaseIO=off: audit must still bind as auditor, got %+v", ledOff.entries)
	}

	// enforce: scout now binds under its identity role.
	ledEnf := &fakeLedger{}
	oEnf := NewOrchestrator(nil, ledEnf, nil)
	oEnf.now = fixedNowFn()
	oEnf.cfg.PhaseIO = config.StageEnforce
	oEnf.emitPhaseBindings(context.Background(), 11, repo, cs, PhaseScout, VerdictPASS)
	if len(ledEnf.entries) != 1 {
		t.Fatalf("PhaseIO=enforce: user phase scout must bind, got %d entries", len(ledEnf.entries))
	}
	s := ledEnf.entries[0]
	if s.Role != "scout" || s.Kind != "agent_subprocess" {
		t.Errorf("PhaseIO=enforce: scout binding role/kind = %q/%q, want scout/agent_subprocess", s.Role, s.Kind)
	}
	if !strings.HasSuffix(s.ArtifactPath, "scout-report.md") {
		t.Errorf("PhaseIO=enforce: scout artifact_path = %q, want suffix scout-report.md", s.ArtifactPath)
	}
}
