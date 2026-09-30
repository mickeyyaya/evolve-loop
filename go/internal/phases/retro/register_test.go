package retro

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func TestRetroSelfRegisters(t *testing.T) {
	factory, ok := registry.For(string(core.PhaseRetro))
	if !ok {
		t.Fatalf("retro not self-registered: registry.For(%q) returned ok=false", core.PhaseRetro)
	}
	runner := factory(core.PhaseRequest{ProjectRoot: t.TempDir()})
	if runner == nil {
		t.Fatal("retro factory returned a nil runner")
	}
	if got := runner.Name(); got != string(core.PhaseRetro) {
		t.Errorf("retro factory runner Name() = %q, want %q", got, core.PhaseRetro)
	}
}
