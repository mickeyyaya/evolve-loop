package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// The settle retry sits on the clean-exit path, so the test binary sleeps for free; retry-count tests inject SleepFn.
func init() { settleSleep = func(time.Duration) {} }

func TestRun_NonTimeout_CleanExitIdle_DeliverableSettlesOnRetry_PrefersFile(t *testing.T) {
	genuine := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	noisyStdout := "Deliverable Contract example (PASS):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n" +
		"Deliverable Contract example (FAIL):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n" +
		"(prompt-echoed examples, not the agent's real report)\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: genuine, stdout: noisyStdout}

	// The probe misses twice while the agent idles; the third check, inside the settle window, sees the PASS.
	calls := 0
	settling := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < 3 {
			return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
		}
		return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: settling,
		SleepFn:  func(time.Duration) {}, // deterministic: no real settle delay
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuine {
		t.Errorf("clean-exit-idle: Classify received contaminated scrollback instead of the settled on-disk deliverable;\n got  %q\n want %q", hooks.gotArtifact, genuine)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (on-disk deliverable settled on retry, must not synthesize FAIL from scrollback)", resp.Verdict)
	}
	if calls < 3 {
		t.Errorf("clean-exit path must settle-retry the presence probe (mirroring the reconcile path); got only %d verify call(s)", calls)
	}
}

func TestRun_NonTimeout_CleanExitIdle_GenuineFAILOnDisk_RecordsFromFileNotScrollback(t *testing.T) {
	genuineFail := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n"
	noisyPassStdout := "prompt example (PASS): <!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	nb := &noisyStdoutBridge{fileContent: genuineFail, stdout: noisyPassStdout}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil), // a well-formed FAIL verifies OK
		SleepFn:  func(time.Duration) {},
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuineFail {
		t.Errorf("on-disk FAIL must be Classified from the FILE, not laundered from PASS-noisy scrollback;\n got  %q\n want %q", hooks.gotArtifact, genuineFail)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL — the file is authoritative in BOTH directions", resp.Verdict)
	}
}

func TestRun_NonTimeout_CleanExitIdle_DeliverableNeverSettles_CoherentFailNotPane(t *testing.T) {
	stdout := "raw scrollback with no clean deliverable\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	nb := &noisyStdoutBridge{fileContent: "", stdout: stdout}
	calls := 0
	neverSettles := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		calls++
		return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: neverSettles,
		SleepFn:  func(time.Duration) {},
	})
	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != "" {
		t.Errorf("a never-settling CONTRACTED deliverable must yield a coherent FAIL (empty artifact to Classify), never the lossy pane; got %q", hooks.gotArtifact)
	}
	if calls <= 1 {
		t.Errorf("clean-exit path must RE-VERIFY (settle-wait), not single-shot; got %d verify call(s)", calls)
	}
	if calls > reconcileSettleRetries+1 {
		t.Errorf("settle-retry must be BOUNDED at %d calls (1 + %d retries) so the loop cannot spin; got %d", reconcileSettleRetries+1, reconcileSettleRetries, calls)
	}
	_ = resp
}
