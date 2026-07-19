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

// Teardown ACS deterministic floor (verdict-incoherence family: cycles
// 603/921/924/931/3). The failure: a tmux auditor writes a complete PASS
// audit-report.md + a ship-eligible acs-verdict.json, then never runs the
// `evolve phase verify` completion handshake and idles until the runner
// ctx-cancels the session. The ctx-cancel maps to a transient teardown
// (engine 635510d7), so control reaches the reconcile door — but the
// teardown-time deliverable.Verify returns not-OK on a report that
// standalone-verifies OK (a lifecycle artifact, not a defect), so the mandatory
// phase hard-FAILs and a green/verify-OK/ship-eligible cycle is discarded, then
// the ADR-0072 coherence floor sees recorded-FAIL vs on-disk-PASS and HALTS.
//
// The fix: before the teardown default-FAIL, consult the NON-LLM ground truth a
// session stall cannot corrupt — the acssuite verdict. When it is PASS AND the
// report carries THIS cycle's challenge token with a PASS sentinel (anti-gaming
// preserved), reconcile to the agent's own report via the same Classify path the
// clean exit uses. This is exactly the (audit==PASS && acs==PASS) condition the
// coherence floor flags, prevented at the source.

const acsFloorToken = "chal-tok-acsfloor-0001"

// writeACSFloorWorkspace stages the deterministic-truth artifacts a teardown
// reconcile reads: the per-cycle challenge token and the acssuite verdict. The
// audit-report.md itself is written by the fakeBridge during Launch.
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

// reportWithToken is a well-formed PASS report echoing THIS cycle's challenge
// token (the anti-gaming anchor) plus the machine-readable verdict sentinel.
const reportWithToken = "<!-- challenge-token: " + acsFloorToken + " -->\n" +
	"# Audit Report\n**Verdict:** PASS\n" +
	"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"

// TestRun_Teardown_VerifyNotOK_ACSShipEligible_ReconcilesToPass — the fix. A
// mandatory audit phase, teardown error, and a teardown-time Verify that returns
// NOT-OK (the cycle-3 divergence — a stray/section/lifecycle code on a report
// that is genuinely on disk and PASS). Because the acssuite verdict is PASS and
// the report carries this cycle's token with a PASS sentinel, the runner must
// reconcile to PASS via Classify, not synthesize FAIL.
func TestRun_Teardown_VerifyNotOK_ACSShipEligible_ReconcilesToPass(t *testing.T) {
	ws := t.TempDir()
	writeACSFloorWorkspace(t, ws, "PASS")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: reportWithToken}
	// Teardown-time Verify diverges to NOT-OK — the exact cycle-3 symptom.
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
	// The rescue must SURFACE the overridden Verify violation(s) on the response — never a
	// silent bypass (a stray_in_worktree hygiene flag shipped on the ACS verdict's authority
	// must stay visible to the operator/retro).
	if !diagMentions(resp.Diagnostics, "stray_in_worktree") {
		t.Errorf("ACS-floor rescue must surface the overridden Verify code on resp.Diagnostics; got %+v", resp.Diagnostics)
	}
}

// TestRun_Teardown_VerifyNotOK_NonAuditPhase_StaysFail — the phase gate. Even
// when a NON-audit phase's teardown workspace happens to carry a PASS
// acs-verdict.json + a token-echoing PASS report, the ACS floor must decline:
// the deterministic acssuite ground truth is an audit-vs-acs cross-check and has
// no meaning for other phases, so a build/scout/tdd teardown still hard-FAILs.
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

// diagMentions reports whether any diagnostic message contains substr.
func diagMentions(diags []core.Diagnostic, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

// TestRun_Teardown_VerifyNotOK_ACSNotShipEligible_StaysFail — the deterministic
// gate. Same teardown + Verify-not-OK, but the acssuite verdict is FAIL: the ACS
// floor must NOT rescue (the tests genuinely did not pass), so the phase
// hard-FAILs and Classify is never reached.
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

// TestRun_Teardown_VerifyNotOK_ReportMissingChallengeToken_StaysFail — the
// anti-gaming gate. Ship-eligible ACS, but the report does NOT echo this cycle's
// challenge token (a stale/forged/replayed report). The ACS floor must decline —
// the deterministic verdict alone cannot launder an unauthenticated report to
// PASS.
func TestRun_Teardown_VerifyNotOK_ReportMissingChallengeToken_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSFloorWorkspace(t, ws, "PASS")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	// Report has a PASS sentinel but NO challenge token → fails the anti-gaming anchor.
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
