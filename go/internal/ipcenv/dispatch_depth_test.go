package ipcenv_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestDispatchDepth_ReadsTheDepthMarkerAndTreatsJunkAsTopLevel(t *testing.T) {
	for raw, want := range map[string]int{"": 0, "2": 2, " 1 ": 1, "-1": 0, "x": 0} {
		getenv := func(k string) string {
			if k == ipcenv.DispatchDepthKey {
				return raw
			}
			return "9"
		}
		if got := ipcenv.DispatchDepth(getenv); got != want {
			t.Errorf("DispatchDepth(%s=%q) = %d, want %d", ipcenv.DispatchDepthKey, raw, got, want)
		}
	}
}

func TestPhaseMarkers_AreProtocolKeysWithTheirWireNames(t *testing.T) {
	if ipcenv.DispatchDepthKey != "EVOLVE_DISPATCH_DEPTH" || ipcenv.PhaseKey != "EVOLVE_PHASE" {
		t.Fatalf("DispatchDepthKey = %q, PhaseKey = %q: the refusals of evolve agent run and the boundary writes read these names", ipcenv.DispatchDepthKey, ipcenv.PhaseKey)
	}
	for _, key := range []string{ipcenv.DispatchDepthKey, ipcenv.PhaseKey} {
		if !slices.Contains(ipcenv.ProtocolKeys(), key) {
			t.Errorf("ProtocolKeys() misses %s", key)
		}
	}
}
