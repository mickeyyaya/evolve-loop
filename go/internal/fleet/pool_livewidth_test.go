package fleet

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

type liveRecord struct {
	PID   int `json:"pid"`
	Width int `json:"width"`
}

func readLive(t *testing.T, path string) liveRecord {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the live width file is unreadable: %v", err)
	}
	var rec liveRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("the live width file is not JSON: %v", err)
	}
	return rec
}

func TestPool_WritesTheLiveWidthAtLaneStartAndEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), lanebudget.LiveWidthFile)
	type seen struct {
		kind  PoolTransitionKind
		live  int
		width int
		pid   int
	}
	var got []seen
	onTransition := func(tr PoolTransition) {
		if tr.Kind != PoolLaunched && tr.Kind != PoolBackfilled && tr.Kind != PoolExited {
			return
		}
		rec := readLive(t, path)
		got = append(got, seen{tr.Kind, tr.Live, rec.Width, rec.PID})
	}
	var log launchLog
	RunPool(context.Background(), PoolConfig{Target: 2, LiveWidthFile: path}, []Todo{{ID: "A"}, {ID: "B"}, {ID: "C"}}, log.launch(0), onTransition)
	if len(got) < 6 {
		t.Fatalf("observed %d lane transitions, want a start and an end for each of 3 lanes", len(got))
	}
	for _, s := range got {
		if want := max(1, s.live); s.width != want || s.pid != os.Getpid() {
			t.Errorf("at %s with %d live lanes the file holds width %d pid %d, want width %d pid %d", s.kind, s.live, s.width, s.pid, want, os.Getpid())
		}
	}
	if end := readLive(t, path); end.Width != 1 {
		t.Errorf("after the last lane the file holds width %d, want the floor of 1", end.Width)
	}
}

func TestPool_ALaneIsToldWhereTheLiveWidthLives(t *testing.T) {
	path := filepath.Join(t.TempDir(), lanebudget.LiveWidthFile)
	var mu sync.Mutex
	var specs []CycleSpec
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		mu.Lock()
		specs = append(specs, spec)
		mu.Unlock()
		return 0, nil
	}
	RunPool(context.Background(), PoolConfig{Target: 2, LiveWidthFile: path}, []Todo{{ID: "A"}}, launch, nil)
	RunPool(context.Background(), PoolConfig{Target: 2}, []Todo{{ID: "B"}}, launch, nil)
	if len(specs) != 2 {
		t.Fatalf("launched %d lanes, want 2", len(specs))
	}
	if got := specs[0].Env[ipcenv.FleetLiveWidthKey]; got != path {
		t.Errorf("lane env %s = %q, want %q", ipcenv.FleetLiveWidthKey, got, path)
	}
	if _, set := specs[1].Env[ipcenv.FleetLiveWidthKey]; set {
		t.Errorf("a pool with no live width file set %s", ipcenv.FleetLiveWidthKey)
	}
}

func TestPool_AFailedLiveWidthWriteDoesNotStopTheLane(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var log launchLog
	res := RunPool(context.Background(), PoolConfig{Target: 1, LiveWidthFile: filepath.Join(blocker, lanebudget.LiveWidthFile)}, []Todo{{ID: "A"}}, log.launch(0), nil)
	if len(res) != 1 || res[0].Err != nil || len(log.got()) != 1 {
		t.Errorf("results = %+v launched = %v, want the lane to run with the spawn width as the fallback", res, log.got())
	}
}
