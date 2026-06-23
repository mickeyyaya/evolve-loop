package retro

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestPhase_NamedConcreteRunner names the concrete retro.Phase type (New returns
// *Phase but the bare type is never named in a test) and pins that New yields a
// non-nil *Phase which satisfies core.PhaseRunner and reports the "retro" identity.
func TestPhase_NamedConcreteRunner(t *testing.T) {
	var p *Phase = New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	if got := p.Name(); got != "retro" {
		t.Errorf("Name() = %q, want retro", got)
	}
	var _ core.PhaseRunner = p // concrete *Phase must satisfy the runner contract
}
