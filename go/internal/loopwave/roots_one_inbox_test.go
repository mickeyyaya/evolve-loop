package loopwave

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestEveryPlannerReaderReadsTheConfiguredEvolveDir(t *testing.T) {
	h := newHarness(t)
	moved := filepath.Join(t.TempDir(), ".evolve")
	e := New(Roots{ProjectRoot: h.root, EvolveDir: moved}, h.ports, h.stderr)
	lifecycleItem(t, moved, inboxmover.StateProcessed, "done")
	lifecycleItem(t, moved, inboxmover.StatePending, "ready")
	writeJSON(t, filepath.Join(moved, "inbox", "console-item.json"), map[string]any{"id": "console-item", "route": "console", "files": []string{"pkg/c.go"}})
	if f := e.probe()("done"); f.Fresh {
		t.Error("the launch gate must see the processed item in the configured evolve dir")
	}
	if kept := topNIDs(t, e.pruneUndispatchable(mustJSON(t, map[string]any{"top_n": []map[string]any{{"id": "done"}, {"id": "ready"}}}))); kept["done"] || !kept["ready"] {
		t.Errorf("the plan prune must read the configured evolve dir: %v", kept)
	}
	if spec, ok := e.refill()(map[string]bool{}); !ok || spec.Scope[0] != "ready" {
		t.Errorf("the refill must read the configured evolve dir: %+v %v", spec, ok)
	}
	if routed, _ := e.routedBase()("console-item"); !routed {
		t.Error("the routing authority must read the configured evolve dir")
	}
}
