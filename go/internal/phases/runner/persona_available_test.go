package runner

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBaseRunner_PersonaAvailable(t *testing.T) {
	hooks := &fakeHooks{phase: "amplify-tests", agent: "evolve-amplify-tests", model: "auto", prompt: "x", verdict: core.VerdictPASS}
	missing := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-other", "x")})
	if err := missing.PersonaAvailable(); !errors.Is(err, core.ErrAgentDocMissing) {
		t.Fatalf("absent persona doc must report core.ErrAgentDocMissing; got %v", err)
	}
	present := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-amplify-tests", "x")})
	if err := present.PersonaAvailable(); err != nil {
		t.Fatalf("present persona doc must be available; got %v", err)
	}
	var _ core.PersonaProber = present
}
