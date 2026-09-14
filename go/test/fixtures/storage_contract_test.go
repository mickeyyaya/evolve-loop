package fixtures_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// The fake must preserve the value semantics of the real persistence boundary.
func TestStorage_StateSnapshotsDoNotAliasCallers(t *testing.T) {
	t.Parallel()
	for _, backend := range []string{"fake", "filesystem"} {
		for _, direction := range []string{"write_input", "read_result"} {
			t.Run(backend+"/"+direction, func(t *testing.T) {
				var store core.Storage = &fixtures.FakeStorage{}
				if backend == "filesystem" {
					store = storage.New(fixtures.NewWorkspace(t).Build().EvolveDir)
				}
				ctx := context.Background()
				state := core.State{
					FailedAt:         []core.FailedRecord{{Cycle: 7, Defects: []string{"original"}}},
					CarryoverTodos:   []core.CarryoverTodo{{ID: "original"}},
					TriageThroughput: []core.TriageThroughputEntry{{Cycle: 7, Floors: 2}},
				}
				cycle := core.CycleState{
					CycleID: 7, CompletedPhases: []string{"tdd"},
					AuditFailReasons: []string{"audit"}, ShipFailReasons: []string{"ship"},
					FailedAt: []core.FailedRecord{{Cycle: 7, Defects: []string{"original"}}},
				}
				fixtures.RequireNoErr(t, store.WriteState(ctx, state), "write state")
				fixtures.RequireNoErr(t, store.WriteCycleState(ctx, cycle), "write cycle state")
				if direction == "read_result" {
					var err error
					state, err = store.ReadState(ctx)
					fixtures.RequireNoErr(t, err, "first read state")
					cycle, err = store.ReadCycleState(ctx)
					fixtures.RequireNoErr(t, err, "first read cycle state")
				}

				state.FailedAt[0].Cycle = 99
				state.FailedAt[0].Defects[0] = "changed"
				state.CarryoverTodos[0].ID = "changed"
				state.TriageThroughput[0].Floors = 99
				cycle.CompletedPhases[0] = "changed"
				cycle.AuditFailReasons[0] = "changed"
				cycle.ShipFailReasons[0] = "changed"
				cycle.FailedAt[0].Cycle = 99
				cycle.FailedAt[0].Defects[0] = "changed"

				gotState, err := store.ReadState(ctx)
				fixtures.RequireNoErr(t, err, "reread state")
				wantState := core.State{
					FailedAt:         []core.FailedRecord{{Cycle: 7, Defects: []string{"original"}}},
					CarryoverTodos:   []core.CarryoverTodo{{ID: "original"}},
					TriageThroughput: []core.TriageThroughputEntry{{Cycle: 7, Floors: 2}},
				}
				if !reflect.DeepEqual(gotState, wantState) {
					t.Errorf("stored state changed through %s: got %+v, want %+v", direction, gotState, wantState)
				}
				gotCycle, err := store.ReadCycleState(ctx)
				fixtures.RequireNoErr(t, err, "reread cycle state")
				wantCycle := core.CycleState{
					CycleID: 7, CompletedPhases: []string{"tdd"},
					AuditFailReasons: []string{"audit"}, ShipFailReasons: []string{"ship"},
					FailedAt: []core.FailedRecord{{Cycle: 7, Defects: []string{"original"}}},
				}
				if !reflect.DeepEqual(gotCycle, wantCycle) {
					t.Errorf("stored cycle changed through %s: got %+v, want %+v", direction, gotCycle, wantCycle)
				}
			})
		}
	}
}

func TestFakeStorage_WriteHistoryIsIndependent(t *testing.T) {
	t.Parallel()
	store := &fixtures.FakeStorage{}
	ctx := context.Background()
	fixtures.RequireNoErr(t, store.WriteState(ctx, core.State{
		FailedAt: []core.FailedRecord{{Defects: []string{"original"}}},
	}), "write state")
	fixtures.RequireNoErr(t, store.WriteCycleState(ctx, core.CycleState{
		FailedAt: []core.FailedRecord{{Defects: []string{"original"}}},
	}), "write cycle state")

	store.State.FailedAt[0].Defects[0] = "changed"
	store.CycleState.FailedAt[0].Defects[0] = "changed"

	if got := store.StateLog[0].FailedAt[0].Defects[0]; got != "original" {
		t.Errorf("state write history = %q, want original", got)
	}
	if got := store.CycleStateLog[0].FailedAt[0].Defects[0]; got != "original" {
		t.Errorf("cycle write history = %q, want original", got)
	}
}
