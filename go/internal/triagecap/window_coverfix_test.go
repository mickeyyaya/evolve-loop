package triagecap

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestK_AWindowOfZeroFloorEntriesClampsToOneNotZero(t *testing.T) {
	t.Parallel()
	window := []core.TriageThroughputEntry{{Cycle: 290, Floors: 0}, {Cycle: 291, Floors: 0}}

	if got := K(window); got != 1 {
		t.Fatalf("K(all-zero window) = %d, want 1 so the cap never rejects everything", got)
	}
}
