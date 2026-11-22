package fleet

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestPoolSpec_CarriesWidthAndLiveWidthFile(t *testing.T) {
	t.Parallel()

	spec := poolSpec(Todo{ID: "t"}, 3, "/tmp/live-width")

	if got := spec.Env[ipcenv.FleetWidthKey]; got != "3" {
		t.Errorf("FleetWidthKey = %q, want %q", got, "3")
	}
	if got := spec.Env[ipcenv.FleetLiveWidthKey]; got != "/tmp/live-width" {
		t.Errorf("FleetLiveWidthKey = %q, want %q", got, "/tmp/live-width")
	}
}
