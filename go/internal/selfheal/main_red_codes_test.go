package selfheal

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestMainRedStopCodes_AreRegisteredUnderTheRepoGate(t *testing.T) {
	t.Parallel()
	for _, code := range []signalcenter.Code{CodeMainRedStopWouldRefuse, CodeMainRedStopReleased, CodeMainRedStopEscalated} {
		if module, ok := signalcenter.IsRegistered(code); !ok || module != signalcenter.ModuleGateRepo {
			t.Errorf("IsRegistered(%s) = %q, %v, want %q, true", code, module, ok, signalcenter.ModuleGateRepo)
		}
	}
}
