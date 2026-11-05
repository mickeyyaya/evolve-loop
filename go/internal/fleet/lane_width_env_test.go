package fleet

import (
	"context"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func widthsSeen(t *testing.T, concurrency, lanes int) []string {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	s := &Supervisor{Concurrency: concurrency, Launch: func(_ context.Context, spec CycleSpec) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, spec.Env[ipcenv.FleetWidthKey])
		return 0, nil
	}}
	s.Run(context.Background(), make([]CycleSpec, lanes))
	return seen
}

func TestSupervisorRun_EachLaneGetsTheEffectiveWidthAfterTheShrink(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		concurrency, lanes int
		want               string
	}{
		{"width 4 of 4", 4, 4, "4"},
		{"width 8 shrinks to 3 lanes", 8, 3, "3"},
		{"no width runs all lanes", 0, 2, "2"},
		{"width 2 of 5", 2, 5, "2"},
	}
	for _, c := range cases {
		seen := widthsSeen(t, c.concurrency, c.lanes)
		if len(seen) != c.lanes {
			t.Fatalf("%s: %d launches, want %d", c.name, len(seen), c.lanes)
		}
		for _, got := range seen {
			if got != c.want {
				t.Errorf("%s: lane width %q, want %q", c.name, got, c.want)
			}
		}
	}
}

func TestPoolSpec_ALaneGetsThePoolWidthAndNoWidthAddsNoKey(t *testing.T) {
	t.Parallel()
	if got := poolSpec(Todo{ID: "t1"}, 4).Env[ipcenv.FleetWidthKey]; got != "4" {
		t.Errorf("poolSpec width 4: lane width %q, want %q", got, "4")
	}
	if _, ok := poolSpec(Todo{ID: "t1"}, 0).Env[ipcenv.FleetWidthKey]; ok {
		t.Errorf("poolSpec width 0 set %s, want no key", ipcenv.FleetWidthKey)
	}
}
