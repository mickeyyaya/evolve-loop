package loopwave

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func TestPlanFn_CarriesAPriorIDWithNoInboxItemUntilItIsRetired(t *testing.T) {
	h := newHarness(t)
	h.ports.LastCycle = func(context.Context) (int, error) { return 3, nil }
	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()),
		map[string]any{"top_n": []map[string]any{{"id": "alpha", "files": []string{"a.go"}}, {"id": "ghost", "files": []string{"g.go"}}}})
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "alpha")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "beta")
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)

	before, _, err := e.PlanFn(2)(context.Background(), 1)
	if err != nil || !strings.Contains(string(before), `"ghost"`) {
		t.Fatalf("an id with no lifecycle evidence is carried (the prune fails open): %s %v", before, err)
	}

	if _, err := inboxmover.RetireUnbacked(inboxmover.Options{ProjectRoot: h.root, Stderr: io.Discard}, 3, inboxmover.StateRejected, "planned no-work", "", []string{"ghost"}); err != nil {
		t.Fatal(err)
	}
	after, _, err := e.PlanFn(2)(context.Background(), 2)

	if err != nil || strings.Contains(string(after), `"ghost"`) || !strings.Contains(string(after), `"alpha"`) || !strings.Contains(string(after), `"beta"`) {
		t.Errorf("the retired id is pruned and its slot refilled: %s %v", after, err)
	}
	if !strings.Contains(h.stderr.String(), `pruned consumed top_n id "ghost"`) {
		t.Errorf("the prune says so: %q", h.stderr.String())
	}
}
