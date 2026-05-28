package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// artifactTimeoutErr mimics exactly what bridge.Engine.Launch returns on exit
// 81 — a wrapped core.ErrArtifactTimeout — so the runner's errors.Is match is
// exercised against the real wire shape, not a hand-rolled sentinel.
func artifactTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 81, core.ErrArtifactTimeout)
}

// TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn is the cycle-120 fix
// (Workstream D): an OPTIONAL phase (build-planner) whose artifact never
// appears must degrade to WARN with a NIL error so the orchestrator advances
// the cycle, instead of the unconditional FAIL+error that aborted cycle-120.
func TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn(t *testing.T) {
	hooks := &fakeHooks{phase: "build-planner", agent: "evolve-build-planner", model: "opus", prompt: "x"}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-build-planner", "x"),
		Optional: true,
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("optional+artifact-timeout must return a NIL error (so the cycle advances); got %v", err)
	}
	if resp.Verdict != core.VerdictWARN {
		t.Errorf("verdict=%q, want WARN", resp.Verdict)
	}
	if hooks.classifyCalls != 0 {
		t.Errorf("Classify must not run on the bridge-error path; got %d calls", hooks.classifyCalls)
	}
	if len(resp.Diagnostics) == 0 || resp.Diagnostics[0].Severity != "warning" {
		t.Errorf("expected a warning diagnostic, got %+v", resp.Diagnostics)
	}
}

// TestRun_OptionalPhase_OtherBridgeError_StillFails proves the soft-fail is
// SCOPED to artifact-timeout: any other bridge error on an optional phase
// still hard-fails (we don't want to silently swallow a real crash).
func TestRun_OptionalPhase_OtherBridgeError_StillFails(t *testing.T) {
	hooks := &fakeHooks{phase: "build-planner", agent: "evolve-build-planner", model: "opus", prompt: "x"}
	fb := &fakeBridge{err: errors.New("bridge: launch exit=2")} // safety-gate, not a timeout
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-build-planner", "x"),
		Optional: true,
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("optional phase with a NON-timeout bridge error must still return an error")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL", resp.Verdict)
	}
}

// TestRun_MandatoryPhase_ArtifactTimeout_StillFails proves the soft-fail is
// SCOPED to optional phases: a mandatory phase (Optional unset) timing out
// still hard-fails — unchanged behavior, so no mandatory phase silently skips.
func TestRun_MandatoryPhase_ArtifactTimeout_StillFails(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "x"}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-builder", "x"),
		// Optional: false (default)
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("a MANDATORY phase that times out must still abort the cycle (non-nil error)")
	}
	if !errors.Is(err, core.ErrArtifactTimeout) {
		t.Errorf("error should still wrap ErrArtifactTimeout for the dispatcher classifier; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL", resp.Verdict)
	}
}
