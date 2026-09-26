package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunner_ContractEscalation_RedispatchesOnEscalatedCLIWithDirective(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-triage", "agy-tmux", nil)
	hooks := &fakeHooks{phase: "triage", agent: "evolve-triage", model: "balanced", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "x"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-triage", "x")})

	directive := "## Correction\nYour previous deliverable was REJECTED: missing the schema_version 2 failure block."
	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot:         root,
		Workspace:           t.TempDir(),
		ModelRoutingCLI:     "claude-tmux",
		CorrectionDirective: directive,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.gotReq.CLI != "claude-tmux" {
		t.Errorf("dispatched CLI=%q, want claude-tmux — the escalated re-dispatch must leave the contract-violating primary (agy-tmux)", fb.gotReq.CLI)
	}
	if fb.gotReq.CorrectionDirective != directive {
		t.Errorf("CorrectionDirective=%q, want the injected violation — an escalated re-dispatch that drops the directive re-runs blind", fb.gotReq.CorrectionDirective)
	}
}

func TestRunner_ContractEscalation_KeepsOriginalPrimaryInChain(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "agy-tmux", []string{"codex-tmux"})
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "deep", prompt: "x", verdict: core.VerdictPASS}
	// The escalated primary hits a boot timeout (exit 80), so the walk must reach the profile's own candidates.
	sb := &scriptedBridge{responses: map[string]scriptedResp{
		"claude-tmux": {
			resp: core.BridgeResponse{ExitCode: 80, Stderr: "REPL boot timeout"},
			err:  errors.New("bridge: launch exit=80"),
		},
	}}
	r := New(Options{Hooks: hooks, Bridge: sb, Prompts: fakePromptsFS("evolve-auditor", "x")})

	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot:         root,
		Workspace:           t.TempDir(),
		ModelRoutingCLI:     "claude-tmux",
		CorrectionDirective: "## Correction\nmissing '## Findings'",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sb.calls) < 2 {
		t.Fatalf("dispatch attempted %v, want the escalated CLI then a profile candidate — the profile chain must survive escalation", sb.calls)
	}
	if sb.calls[0] != "claude-tmux" {
		t.Errorf("first attempt=%q, want claude-tmux (the escalation target as chain primary)", sb.calls[0])
	}
	if sb.calls[1] == "claude-tmux" {
		t.Errorf("second attempt=%q, want a DIFFERENT candidate from the profile chain (%v)", sb.calls[1], sb.calls)
	}
}
