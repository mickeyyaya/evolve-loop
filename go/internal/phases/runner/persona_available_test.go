package runner

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestBaseRunner_PersonaAvailable — 2026-09-09 token-waste root cause #2: an
// optional phase whose persona doc does not exist must never enter a
// selectable plan. The runner already knows its persona name and loader; it
// answers the availability question before any dispatch, with the same
// sentinel the dispatch path raises (core.ErrAgentDocMissing) so the planner
// and the skip classifier agree on the cause.
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
