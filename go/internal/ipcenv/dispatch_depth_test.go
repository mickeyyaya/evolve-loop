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

func TestInCell_IsTrueForThePhaseMarkerOrAPositiveDepth(t *testing.T) {
	cases := []struct {
		phase, depth string
		want         bool
	}{{"", "", false}, {"build", "", true}, {"", "1", true}, {"", "0", false}, {"", "x", false}}
	for _, c := range cases {
		getenv := func(k string) string {
			switch k {
			case ipcenv.PhaseKey:
				return c.phase
			case ipcenv.DispatchDepthKey:
				return c.depth
			}
			return ""
		}
		if got := ipcenv.InCell(getenv); got != c.want {
			t.Errorf("InCell(phase=%q depth=%q) = %v, want %v", c.phase, c.depth, got, c.want)
		}
	}
}
