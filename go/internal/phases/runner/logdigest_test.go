package runner

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestRun_WithLogAccessInlinesD0DigestOnly(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &promptRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-scout", "body")})
	req := core.PhaseRequest{
		Cycle:       7,
		ProjectRoot: t.TempDir(),
		Workspace:   ws,
		Spec:        &phasespec.PhaseSpec{LogAccess: &phasespec.LogAccess{Layers: []string{"D0"}}},
	}
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(br.gotPrompt, "## Logs (D0)") || !strings.Contains(br.gotPrompt, "Use evolve logs") {
		t.Errorf("prompt missing D0 digest block: %q", br.gotPrompt)
	}
}

func TestRun_WithoutLogAccessLeavesPromptUnchanged(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	br := &promptRecordingBridge{}
	r := New(Options{Hooks: hooks, Bridge: br, Prompts: fakePromptsFS("evolve-scout", "body")})
	if _, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: t.TempDir(), Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if br.gotPrompt != "body" {
		t.Errorf("prompt = %q, want %q", br.gotPrompt, "body")
	}
}

func TestPreparePhaseExecution_ErrorPaths(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}

	rMissingRef := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-builder", "body")})
	if _, _, err := rMissingRef.preparePhaseExecution(core.PhaseRequest{
		ProjectRoot:                     t.TempDir(),
		Workspace:                       t.TempDir(),
		ExplanationDocumentationVersion: 1,
	}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing explanation contract ref err = %v, want fs.ErrNotExist", err)
	}

	rMalformedDigest := New(Options{
		Hooks:   hooks,
		Bridge:  &fakeBridge{},
		Prompts: fakePromptsFS("evolve-builder", "<!-- digest:role=build -->\nunclosed"),
	})
	if _, _, err := rMalformedDigest.preparePhaseExecution(core.PhaseRequest{
		ProjectRoot: t.TempDir(),
		Workspace:   t.TempDir(),
	}); err == nil {
		t.Error("malformed digest err = nil, want error")
	}
}
