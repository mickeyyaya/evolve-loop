package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestRun_CleanExitIdle_DeliverableSettlesLate_WidenedWindowCatchesIt(t *testing.T) {
	genuine := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	// The pane carries echoed example sentinels, a FAIL among them, which would synthesize a false FAIL.
	noisyStdout := "Deliverable Contract example (FAIL):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n" +
		"(prompt-echoed example, not the agent's real report)\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: genuine, stdout: noisyStdout}

	const settleOnCall = 10 // beyond a 3-retry window, within the current one
	calls := 0
	settlesLate := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < settleOnCall {
			return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
		}
		return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: settlesLate,
		SleepFn:  func(time.Duration) {}, // deterministic: no real settle delay in tests
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuine {
		t.Errorf("late-settling deliverable dropped to the pane (false FAIL) — settle window too short;\n got  %q\n want %q", hooks.gotArtifact, genuine)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (the valid report settled at call %d, within the widened window)", resp.Verdict, settleOnCall)
	}
	if calls < settleOnCall {
		t.Errorf("settle-retry gave up early at %d calls; must retry until the deliverable settles (call %d)", calls, settleOnCall)
	}
}
