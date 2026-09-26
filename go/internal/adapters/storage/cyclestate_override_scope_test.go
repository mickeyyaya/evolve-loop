package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// A go test run inside a fleet lane inherits the lane's override. Its fixtures write through this
// adapter, and none of them may land in the lane's live cycle state.
func TestWriteCycleState_AFixtureNeverWritesAnotherTreesLiveLaneFile(t *testing.T) {
	live := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1700", core.CycleStateFile)
	mkParents(t, live)
	const liveState = `{"cycle_id":1700,"phase":"audit"}`
	if err := os.WriteFile(live, []byte(liveState), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, live)

	fixture := New(filepath.Join(t.TempDir(), ".evolve"))
	if err := fixture.WriteCycleState(context.Background(), core.CycleState{CycleID: 42, Phase: "build"}); err != nil {
		t.Fatalf("fixture write: %v", err)
	}
	if back, err := fixture.ReadCycleState(context.Background()); err != nil || back.CycleID != 42 {
		t.Errorf("the fixture reads back cycle %d (err %v), want its own 42", back.CycleID, err)
	}

	got, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != liveState {
		t.Errorf("the lane's live cycle state was overwritten by a fixture: %s", got)
	}
}
