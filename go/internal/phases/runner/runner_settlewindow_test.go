package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// runner_settlewindow_test.go — the verdict-incoherence hotfix (ADR-0072, cycle-921):
// the settle-retry window must outlast a clean-exit-idle agent's worst-case flush
// latency, or a genuinely-valid-but-late-flushed deliverable is missed by Verify and
// the runner falls back to the (sentinel-lost) pane → a false FAIL that contradicts
// the green artifact.

// TestRun_CleanExitIdle_DeliverableSettlesLate_WidenedWindowCatchesIt: a deliverable
// that verifies OK only on the 10th probe — PAST the old ~600ms (3-retry) window,
// which would have given up at call 4 and dropped to the pane — must now be caught
// and PREFERRED over the pane. This is RED at reconcileSettleRetries=3 and GREEN at
// the widened window. Anti-gaming is untouched: the report still must VERIFY (a
// never-settling/malformed report still falls back to stdout — see the NeverSettles test).
func TestRun_CleanExitIdle_DeliverableSettlesLate_WidenedWindowCatchesIt(t *testing.T) {
	genuine := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	// The pane carries prompt-echoed EXAMPLE sentinels (a FAIL among them) — exactly
	// what would synthesize a false FAIL if the runner classified the pane.
	noisyStdout := "Deliverable Contract example (FAIL):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n" +
		"(prompt-echoed example, not the agent's real report)\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: genuine, stdout: noisyStdout}

	const settleOnCall = 10 // past the OLD 3-retry window (give-up at call 4), within the widened window
	calls := 0
	settlesLate := func(string, phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < settleOnCall {
			return deliverable.Result{OK: false}, nil
		}
		return deliverable.Result{OK: true}, nil
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
