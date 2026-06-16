package tdd

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestPhase_NamesTDDAndSatisfiesRunner names the concrete tdd.Phase type (New
// returns *Phase but the bare type is never named in a test) and pins that New
// yields a *Phase reporting the "tdd" phase name and satisfying the
// core.PhaseRunner contract the registry dispatches against.
func TestPhase_NamesTDDAndSatisfiesRunner(t *testing.T) {
	var p *Phase = New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")})
	var _ core.PhaseRunner = p // compile-time: *Phase implements the phase contract
	if got := p.Name(); got != "tdd" {
		t.Errorf("Name() = %q, want tdd", got)
	}
}
