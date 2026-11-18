package tdd

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_NamesTDDAndSatisfiesRunner(t *testing.T) {
	var p *Phase = New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")})
	var pr core.PhaseRunner = p
	if got := pr.Name(); got != "tdd" {
		t.Errorf("Name() = %q, want tdd", got)
	}
}
