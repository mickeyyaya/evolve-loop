package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

func seedStaleReport(t *testing.T, ws, phase string) string {
	t.Helper()
	path := filepath.Join(ws, phase+"-report.md")
	body := "# " + phase + "\n<!-- evolve-verdict: {\"phase\":\"" + phase + "\",\"verdict\":\"PASS\"} -->\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_Timeout_StalePreDispatchLeftoverIsNotReconciled(t *testing.T) {
	ws := t.TempDir()
	seedStaleReport(t, ws, "audit")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr()} // writes nothing: the leftover stands alone
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err == nil {
		t.Fatalf("a byte-identical pre-dispatch leftover must NOT reconcile to a nil error (resp=%+v)", resp)
	}
	if resp.Reconciled {
		t.Fatal("resp.Reconciled=true on a stale leftover — the prior attempt's verdict was resurrected")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (no trustworthy deliverable)", resp.Verdict)
	}
	all := ""
	for _, d := range resp.Diagnostics {
		all += d.Message + "\n"
	}
	if !strings.Contains(all, "pre-dispatch") {
		t.Errorf("diagnostics must name the pre-dispatch refusal for forensics; got %q", all)
	}
}

func TestRun_Timeout_RewrittenLeftoverStillReconciles(t *testing.T) {
	ws := t.TempDir()
	seedStaleReport(t, ws, "audit")
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(),
		writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err != nil {
		t.Fatalf("a rewritten deliverable must still reconcile (cycle-254/255): %v", err)
	}
	if !resp.Reconciled || resp.Verdict != core.VerdictPASS {
		t.Errorf("got (reconciled=%v, verdict=%q), want (true, PASS)", resp.Reconciled, resp.Verdict)
	}
}

func TestRun_Timeout_OptionalPhaseStaleLeftoverDegradesToWarn(t *testing.T) {
	ws := t.TempDir()
	seedStaleReport(t, ws, "smell-scan")
	hooks := &fakeHooks{phase: "smell-scan", agent: "evolve-smell-scan", model: "", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-smell-scan", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
		Optional: true,
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err != nil {
		t.Fatalf("optional phase must soft-fail, not error: %v", err)
	}
	if resp.Verdict != core.VerdictWARN || resp.Reconciled {
		t.Errorf("got (verdict=%q, reconciled=%v), want (WARN, false)", resp.Verdict, resp.Reconciled)
	}
}

func TestRun_Timeout_AcsFloorRefusesStaleLeftover(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "audit-report.md")
	body := "# audit\n<!-- challenge-token: tok-123 -->\nverdict: PASS\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"),
		[]byte(`{"verdict":"PASS","red_count":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte("tok-123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr()} // writes nothing
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
		// Verify not-OK, the shape that routes into the ACS floor.
		VerifyFn: verifyReturns(deliverable.Result{OK: false}, nil),
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err == nil || resp.Reconciled {
		t.Fatalf("the ACS floor resurrected a stale pre-dispatch leftover: (reconciled=%v, err=%v)", resp.Reconciled, err)
	}
}
