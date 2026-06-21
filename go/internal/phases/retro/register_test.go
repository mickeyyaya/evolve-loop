package retro

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

// TestRetroSelfRegisters asserts the retro phase publishes its own factory to
// the phase registry in package init() — like every other built-in phase. The
// dispatcher must not hardcode retro construction (phase-agnostic flow,
// ADR-0035/0038); it resolves the factory by name.
//
// Registration previously lived in the dispatcher (internal/cli/phasecmd);
// retro now self-registers in its own init(). This is the permanent regression
// guard for that invariant — the test does not import the dispatcher.
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
