package build

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_NamedConcreteRunner(t *testing.T) {
	var p *Phase = New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	if got := p.Name(); got != string(core.PhaseBuild) {
		t.Errorf("Name() = %q, want %q", got, core.PhaseBuild)
	}
	var _ core.PhaseRunner = p
}
