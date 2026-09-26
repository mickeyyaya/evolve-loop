package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// artifactTimeoutErr wraps core.ErrArtifactTimeout as bridge.Engine.Launch does on exit 81, so errors.Is sees the real shape.
func artifactTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=%d: %w", 81, core.ErrArtifactTimeout)
}

func TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn(t *testing.T) {
	hooks := &fakeHooks{phase: "build-planner", agent: "evolve-build-planner", model: "opus", prompt: "x"}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-build-planner", "x"),
		Optional: true,
		SleepFn:  func(time.Duration) {}, // skip the real settle-retry delay on the miss path
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

func TestRun_MandatoryPhase_ArtifactTimeout_StillFails(t *testing.T) {
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "x"}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-builder", "x"),
		SleepFn: func(time.Duration) {}, // skip the real settle-retry delay on the miss path
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
