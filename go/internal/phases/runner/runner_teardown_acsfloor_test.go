package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

const acsFloorToken = "chal-tok-acsfloor-0001"

// writeACSFloorWorkspace stages the challenge token and the acssuite verdict; the fake bridge writes the report.
func writeACSFloorWorkspace(t *testing.T, ws, acsVerdict string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte(acsFloorToken+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	acs := `{"schema_version":"1.0","verdict":"` + acsVerdict + `","red_count":0,"ship_eligible":true}`
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte(acs), 0o644); err != nil {
		t.Fatal(err)
	}
}

const reportWithToken = "<!-- challenge-token: " + acsFloorToken + " -->\n" +
	"# Audit Report\n**Verdict:** PASS\n" +
	"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"

func TestRun_Teardown_VerifyNotOK_ACSShipEligible_ReconcilesToPass(t *testing.T) {
	ws := t.TempDir()
	writeACSFloorWorkspace(t, ws, "PASS")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: reportWithToken}
	// Verify reports not-OK at teardown although the report on disk is a genuine PASS.
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeStrayInWorktree, Message: "stray in worktree"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(notOK, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err != nil {
		t.Fatalf("ship-eligible ACS + token-valid PASS report on a teardown Verify-not-OK must reconcile to a NIL error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (ACS deterministic floor reconciled the false-FAIL)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true when the ACS floor rescues a teardown false-FAIL")
	}
	if hooks.classifyCalls != 1 {
		t.Errorf("Classify must run on the reconcile fall-through so the agent's own verdict is honored; got %d calls", hooks.classifyCalls)
	}
	if !diagMentions(resp.Diagnostics, "stray_in_worktree") {
		t.Errorf("ACS-floor rescue must surface the overridden Verify code on resp.Diagnostics; got %+v", resp.Diagnostics)
	}
}

func TestRun_Teardown_VerifyNotOK_NonAuditPhase_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSFloorWorkspace(t, ws, "PASS")
	// Also stage an audit-shaped report, to prove even audit artifacts don't rescue a non-audit phase.
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(reportWithToken), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: reportWithToken}
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeStrayInWorktree, Message: "stray"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-builder", "x"),
		VerifyFn: verifyReturns(notOK, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err == nil {
		t.Fatal("a NON-audit phase must NOT be rescued by the audit-scoped ACS floor — it must hard-FAIL")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (phase gate: ACS floor is audit-only)", resp.Verdict)
	}
	if resp.Reconciled {
		t.Error("resp.Reconciled must be false for a non-audit phase")
	}
}

func diagMentions(diags []core.Diagnostic, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestRun_Teardown_VerifyNotOK_ACSNotShipEligible_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSFloorWorkspace(t, ws, "FAIL")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: reportWithToken}
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeStrayInWorktree, Message: "stray"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(notOK, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err == nil {
		t.Fatal("a non-ship-eligible acssuite verdict must NOT be rescued by the ACS floor — the phase must hard-FAIL")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (ACS not ship-eligible)", resp.Verdict)
	}
	if hooks.classifyCalls != 0 {
		t.Errorf("Classify must NOT run when the ACS floor declines; got %d calls", hooks.classifyCalls)
	}
	if resp.Reconciled {
		t.Error("resp.Reconciled must be false when nothing was rescued")
	}
}

func TestRun_Teardown_VerifyNotOK_ReportMissingChallengeToken_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSFloorWorkspace(t, ws, "PASS")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	// A PASS sentinel without this cycle's challenge token.
	forged := "# Audit Report\n**Verdict:** PASS\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: forged}
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeStrayInWorktree, Message: "stray"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(notOK, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err == nil {
		t.Fatal("a report missing this cycle's challenge token must NOT be rescued by the ACS floor (anti-gaming)")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (report failed the challenge-token anchor)", resp.Verdict)
	}
	if resp.Reconciled {
		t.Error("resp.Reconciled must be false for a token-less report")
	}
}
