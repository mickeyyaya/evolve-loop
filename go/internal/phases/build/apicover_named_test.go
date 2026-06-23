package build

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestPhase_NamedConcreteRunner names the concrete build.Phase type (New returns
// *Phase but the type is never named in a test) and pins the public-API promise
// that New hands back a concrete *Phase which still satisfies core.PhaseRunner
// and reports the "build" phase identity.
func TestPhase_NamedConcreteRunner(t *testing.T) {
	var p *Phase = New(Config{Bridge: &fakeBridge{}, Prompts: fakePromptsFS("body")})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	if got := p.Name(); got != string(core.PhaseBuild) {
		t.Errorf("Name() = %q, want %q", got, core.PhaseBuild)
	}
	// The concrete *Phase must still satisfy the core.PhaseRunner contract it embeds.
	var _ core.PhaseRunner = p
}
