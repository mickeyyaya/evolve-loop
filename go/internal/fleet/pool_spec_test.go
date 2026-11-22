package fleet

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestPoolSpec_ALaneEnvCarriesTheDepsPriorityAndContractOfItsTodo(t *testing.T) {
	t.Parallel()
	td := Todo{ID: "t", DependsOn: []string{"d1", "d2"}, Priority: 4, OutputContract: "pr"}

	spec := poolSpec(td, 0, "")

	got, err := committedset.DecodeLaneSpec(spec.Env[ipcenv.FleetSpecKey])
	want := committedset.LaneSpec{DependsOn: []string{"d1", "d2"}, Priority: 4, OutputContract: "pr"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("lane spec in the env = %+v, %v, want %+v", got, err, want)
	}
}

func TestPoolSpec_ATodoWithNoDepsPriorityOrContractSetsNoSpecKey(t *testing.T) {
	t.Parallel()

	spec := poolSpec(Todo{ID: "t"}, 0, "")

	if v, ok := spec.Env[ipcenv.FleetSpecKey]; ok {
		t.Fatalf("env carries %s = %q for a todo with no spec", ipcenv.FleetSpecKey, v)
	}
}
