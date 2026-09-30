package retro

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_NamedConcreteRunner(t *testing.T) {
	var p *Phase = New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	if got := p.Name(); got != "retro" {
		t.Errorf("Name() = %q, want retro", got)
	}
	var _ core.PhaseRunner = p
}
