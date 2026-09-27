package loopwave

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func TestPlanFn_NeverPlansALaneTheLaunchGateWouldRefuse(t *testing.T) {
	h := newHarness(t)
	h.ports.LastCycle = func(context.Context) (int, error) { return 3, nil }
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()),
		map[string]any{"top_n": []map[string]any{{"id": "shipped-a"}, {"id": "shipped-b"}}})
	lifecycleItem(t, h.evolveDir, inboxmover.StateConsumed, "shipped-a")
	lifecycleItem(t, h.evolveDir, inboxmover.StateConsumed, "shipped-b")
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "console-dep.json"),
		map[string]any{"id": "console-dep", "weight": 0.9, "route": "console", "files": []string{"pkg/console.go"}})
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "blocked-a", "console-dep")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "blocked-b", "console-dep")
	data, _, err := e.PlanFn(2)(context.Background(), 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	planned := topNIDs(t, data)
	probe := e.probe()
	for id := range planned {
		if f := probe(id); !f.Fresh {
			t.Errorf("planned %q, which the launch gate refuses (%s)", id, f.Reason)
		}
	}
	if len(planned) != 0 {
		t.Errorf("no lane is dispatchable, so the plan is empty: %v", planned)
	}
}
