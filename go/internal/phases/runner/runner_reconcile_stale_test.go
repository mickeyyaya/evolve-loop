package runner

// runner_reconcile_stale_test.go — the SECOND door of cycle-1550 (go-review
// CRITICAL on the bridge baseline fix): after a bridge artifact-timeout the
// runner's reconcile-on-teardown re-reads the canonical artifact and trusts a
// well-formed deliverable. A byte-identical leftover from a PRIOR attempt is
// well-formed, carries the cycle-scoped challenge token, and pre-fix would be
// resurrected as this session's verdict — same wrong outcome as the bridge
// door, five minutes slower. The runner now snapshots the artifact
// PRE-DISPATCH and refuses to reconcile anything still byte-identical to it.
// The cycle-254/255 contract is untouched: a deliverable the agent wrote
// DURING the session (fakeBridge.writeArtifact — new mtime) still reconciles,
// pinned by TestRun_Timeout_WellFormedPASS_ReconcilesToPass.

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

// The replay: stale well-formed PASS report on disk pre-dispatch, bridge times
// out, agent wrote NOTHING — the runner must NOT resurrect the leftover.
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

// The contract-preserved complement: same stale leftover, but the agent
// REWROTE the report during the session (fakeBridge.writeArtifact — even with
// identical bytes the mtime advances) — reconcile behaves exactly as the
// cycle-254/255 contract requires.
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

// An OPTIONAL phase with only the stale leftover degrades to WARN (the
// Workstream-D soft-fail), never resurrects.
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

// The SECOND reconcile door — the ACS deterministic floor — must refuse the
// stale leftover too: a prior attempt's audit report carries THIS cycle's
// challenge token (minted once per cycle) and the workspace's acs-verdict.json
// may equally be the prior attempt's PASS, so token+acs cannot distinguish
// stale from fresh. Only the pre-dispatch snapshot can.
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
		// Verify not-OK: the shape that historically routed into the ACS floor.
		VerifyFn: verifyReturns(deliverable.Result{OK: false}, nil),
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: ws})
	if err == nil || resp.Reconciled {
		t.Fatalf("the ACS floor resurrected a stale pre-dispatch leftover: (reconciled=%v, err=%v)", resp.Reconciled, err)
	}
}
