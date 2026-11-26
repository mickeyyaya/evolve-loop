package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func laneIdentityWorkspace(t *testing.T, ids ...string) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		return ws
	}
	pin, err := json.Marshal(core.LaneScope{TodoIDs: ids, GoalHash: "g"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.LaneScopeFile), pin, 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestLaneIdentity_ComposeRecordsThePoolTheLaneCycleAndTheScopeOfAFleetLane(t *testing.T) {
	t.Cleanup(BindLane(41))
	cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: laneIdentityWorkspace(t, "item-a", "item-b")}

	cp := Compose(cs, ReasonQuotaLikely, 0, "", fixedTime())

	if cp.PoolID != 41 || cp.LaneCycle != 7 || !reflect.DeepEqual(cp.Scope, []string{"item-a", "item-b"}) {
		t.Fatalf("checkpoint identity = pool %d, lane cycle %d, scope %v; want pool 41, lane cycle 7, scope [item-a item-b]", cp.PoolID, cp.LaneCycle, cp.Scope)
	}
}

func TestLaneIdentity_ComposeLeavesASequentialCycleWithoutAnyLaneIdentity(t *testing.T) {
	cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: laneIdentityWorkspace(t)}

	cp := Compose(cs, ReasonQuotaLikely, 0, "", fixedTime())

	if cp.PoolID != 0 || cp.LaneCycle != 0 || cp.Scope != nil {
		t.Fatalf("a sequential checkpoint carries identity: pool %d, lane cycle %d, scope %v", cp.PoolID, cp.LaneCycle, cp.Scope)
	}
	raw, err := json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"poolId", "laneCycle", "scope"} {
		if _, present := keys[key]; present {
			t.Errorf("the sequential checkpoint writes the key %q; a legacy lane file must stay detectable by its absence", key)
		}
	}
}

func TestLaneIdentity_ALanePinWithoutAPoolIdKeepsItsScopeAndNoPool(t *testing.T) {
	cs := core.CycleState{CycleID: 9, WorkspacePath: laneIdentityWorkspace(t, "item-a")}

	cp := Compose(cs, ReasonOperatorRequest, 0, "", fixedTime())

	if cp.PoolID != 0 || cp.LaneCycle != 9 || !reflect.DeepEqual(cp.Scope, []string{"item-a"}) {
		t.Fatalf("checkpoint identity = pool %d, lane cycle %d, scope %v; want pool 0, lane cycle 9, scope [item-a]", cp.PoolID, cp.LaneCycle, cp.Scope)
	}
}

func TestLaneIdentity_EveryComposeVariantCarriesTheIdentity(t *testing.T) {
	t.Cleanup(BindLane(41))
	cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: laneIdentityWorkspace(t, "item-a")}

	checked, err := ComposeChecked(cs, ReasonStallInactivity, 0, "", fixedTime())
	if err != nil {
		t.Fatal(err)
	}
	withIntegrity := ComposeWithIntegrity(cs, ReasonStallInactivity, 0, "", fixedTime(), nil)

	for name, cp := range map[string]Checkpoint{"ComposeChecked": checked, "ComposeWithIntegrity": withIntegrity} {
		if cp.PoolID != 41 || cp.LaneCycle != 7 || !reflect.DeepEqual(cp.Scope, []string{"item-a"}) {
			t.Errorf("%s identity = pool %d, lane cycle %d, scope %v", name, cp.PoolID, cp.LaneCycle, cp.Scope)
		}
	}
}

func TestLaneIdentity_TheBoundaryCheckpointersWriteTheIdentityIntoTheStateFile(t *testing.T) {
	boundaries := map[string]func(core.CycleState, string, time.Time) error{
		"resume":      core.ResumeBoundaryCheckpointer,
		"phase":       core.PhaseBoundaryCheckpointer,
		"main-red":    core.MainRedBoundaryCheckpointer,
		"quota-pause": core.QuotaBoundaryCheckpointer,
	}
	t.Cleanup(BindLane(41))
	for name, hook := range boundaries {
		root := t.TempDir()
		seedCycleState(t, root, "")
		cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: laneIdentityWorkspace(t, "item-a")}

		if err := hook(cs, root, fixedTime()); err != nil {
			t.Fatalf("%s boundary: %v", name, err)
		}

		raw, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
		if err != nil {
			t.Fatal(err)
		}
		var state struct {
			Checkpoint Checkpoint `json:"checkpoint"`
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		got := state.Checkpoint
		if got.PoolID != 41 || got.LaneCycle != 7 || !reflect.DeepEqual(got.Scope, []string{"item-a"}) {
			t.Errorf("%s boundary wrote identity pool %d, lane cycle %d, scope %v; want pool 41, lane cycle 7, scope [item-a]", name, got.PoolID, got.LaneCycle, got.Scope)
		}
	}
}

func TestLaneIdentity_BindLaneRestoresTheBindingOfItsCaller(t *testing.T) {
	outer := BindLane(41)
	inner := BindLane(42)
	cs := core.CycleState{CycleID: 7, WorkspacePath: laneIdentityWorkspace(t)}

	if got := Compose(cs, ReasonQuotaLikely, 0, "", fixedTime()).PoolID; got != 42 {
		t.Errorf("pool while bound to 42 = %d", got)
	}
	inner()
	if got := Compose(cs, ReasonQuotaLikely, 0, "", fixedTime()).PoolID; got != 41 {
		t.Errorf("pool after the inner restore = %d, want 41", got)
	}
	outer()
	if got := Compose(cs, ReasonQuotaLikely, 0, "", fixedTime()); got.PoolID != 0 || got.LaneCycle != 0 {
		t.Errorf("identity after the outer restore = pool %d, lane cycle %d, want none", got.PoolID, got.LaneCycle)
	}
}

func TestLaneIdentity_ABoundPoolMakesTheCycleALaneEvenWithoutAPin(t *testing.T) {
	t.Cleanup(BindLane(41))
	cs := core.CycleState{CycleID: 7, WorkspacePath: laneIdentityWorkspace(t)}

	cp := Compose(cs, ReasonQuotaLikely, 0, "", fixedTime())

	if cp.PoolID != 41 || cp.LaneCycle != 7 || cp.Scope != nil {
		t.Fatalf("identity = pool %d, lane cycle %d, scope %v; want pool 41, lane cycle 7, no scope", cp.PoolID, cp.LaneCycle, cp.Scope)
	}
}
