package runner

// contract_escalation_dispatch_test.go — the RUNNER-side half of contract-block
// CLI escalation (inbox contract-block-cli-escalation, P1 0.95).
//
// core's correction ladder escalates by setting PhaseRequest.ModelRoutingCLI on
// the re-dispatch that already failed the contract. That only helps if the
// runner (a) actually dispatches the escalated CLI as chain primary, and (b)
// still carries the correction directive telling the agent what to fix. This
// pins BOTH on one dispatch — the combination the core fix depends on, which
// neither the pre-existing overlay tests nor the correction tests covered alone.

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestRunner_ContractEscalation_RedispatchesOnEscalatedCLIWithDirective drives a
// correction re-dispatch shaped exactly like the escalated one core emits:
// profile primary agy-tmux (the CLI that failed the contract twice live),
// ModelRoutingCLI=claude-tmux (the escalation target) and a non-empty
// CorrectionDirective.
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

// TestRunner_ContractEscalation_KeepsOriginalPrimaryInChain pins the SOFT
// semantics: escalation promotes the fallback CLI to primary but must NOT
// discard the profile's own chain. If the escalated CLI is missing/benched on
// this host the dispatch has to be able to walk back — an escalation that
// collapses the chain to one unavailable candidate trades a format failure for
// an availability failure.
func TestRunner_ContractEscalation_KeepsOriginalPrimaryInChain(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "agy-tmux", []string{"codex-tmux"})
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "deep", prompt: "x", verdict: core.VerdictPASS}
	// The escalated primary hits a REPL boot timeout (exit 80, a fallback
	// trigger); the walk must continue into the profile's original candidates.
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
