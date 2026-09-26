package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func diagsContain(diags []core.Diagnostic, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestRun_ContractedPhase_UnverifiedDeliverable_NonShipVerdictPassesThrough(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-intent", "claude-tmux", nil)
	// Classify returns SKIPPED from partial content; the file is present but lacks a required section.
	hooks := &fakeHooks{phase: "intent", agent: "evolve-intent", model: "auto", prompt: "x", verdict: core.VerdictSKIPPED}
	const partial = "[intent-unchanged] goal_hash=abc12345\n"
	bridge := &divergentBridge{fileContent: partial, stdoutContent: partial}
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-intent", "x"),
		VerifyFn: verifyReturns(deliverable.Result{
			OK:         false,
			Violations: []deliverable.Violation{{Code: "MISSING_SECTION", Message: "required section \"goal\" is missing"}},
		}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictSKIPPED {
		t.Errorf("verdict=%q, want SKIPPED — a legitimate NON-SHIP verdict a phase derives from partial content must survive the ship-guard (only a clean PASS is downgraded)", resp.Verdict)
	}
	if hooks.gotArtifact != partial {
		t.Errorf("Classify received %q, want the partial file content %q — the content must reach Classify so the phase can derive its non-ship verdict", hooks.gotArtifact, partial)
	}
}

func TestRun_ContractedPhase_UnverifiedDeliverable_WarnPassesThrough(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictWARN}
	const partial = "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"WARN\"} -->\n(issues noted, but no challenge token)\n"
	bridge := &divergentBridge{fileContent: partial, stdoutContent: partial}
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{
			OK:         false,
			Violations: []deliverable.Violation{{Code: "MISSING_CHALLENGE_TOKEN", Message: "deliverable did not echo the challenge token"}},
		}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictWARN {
		t.Errorf("verdict=%q, want WARN — a failed-verify WARN must pass the ship-guard (fluent ships it; strict_audit promotes it in the orchestrator); only a clean PASS is clamped", resp.Verdict)
	}
}

func TestRun_UncontractedPhase_PaneRemainsVerdictSource(t *testing.T) {
	stdout := "# scout\n<!-- evolve-verdict: {\"phase\":\"scout\",\"verdict\":\"PASS\"} -->\n"
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: "", stdout: stdout}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-scout", "x"),
		VerifyFn: verifyReturns(deliverable.Result{}, errors.New("deliverable: no contract registered for phase \"scout\"")),
		SleepFn:  func(time.Duration) {},
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != stdout {
		t.Errorf("an UNCONTRACTED phase must keep the pane as its verdict source; Classify received %q, want %q", hooks.gotArtifact, stdout)
	}
}

func TestRun_UncontractedPhase_NoWastedSettleSleeps(t *testing.T) {
	stdout := "raw scrollback\n"
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: "", stdout: stdout}
	calls := 0
	erroring := func(string, phasecontract.Roots) (deliverable.Result, error) {
		calls++
		return deliverable.Result{}, errors.New("deliverable: no contract registered for phase \"scout\"")
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-scout", "x"),
		VerifyFn: erroring,
		SleepFn:  func(time.Duration) {},
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 1 {
		t.Errorf("an error result must not be re-probed (waiting cannot fix a missing contract); got %d verify call(s), want 1", calls)
	}
}
