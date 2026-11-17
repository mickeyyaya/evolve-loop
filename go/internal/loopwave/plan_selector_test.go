package loopwave

import (
	"context"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func selectorBacklog(t *testing.T, h *harness) {
	t.Helper()
	for _, it := range []struct {
		id     string
		weight float64
	}{{"alpha-ready", 0.9}, {"bravo-ready", 0.8}, {"charlie-ready", 0.7}, {"delta-ready", 0.6}} {
		writeJSON(t, filepath.Join(h.evolveDir, "inbox", it.id+".json"),
			map[string]any{"id": it.id, "weight": it.weight, "priority_class": "feature", "files": []string{"pkg/" + it.id + "/x.go"}})
	}
}

func selectorEngine(h *harness, s Selector) *Engine {
	return New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr, WithSelector(s))
}

func plannedIDs(t *testing.T, e *Engine, count int) []string {
	t.Helper()
	data, _, err := e.PlanFn(count)(context.Background(), 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var ids []string
	for id := range topNIDs(t, data) {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func withPriorDecision(t *testing.T, h *harness, ids ...string) {
	t.Helper()
	h.ports.LastCycle = func(context.Context) (int, error) { return 3, nil }
	var cards []map[string]any
	for _, id := range ids {
		cards = append(cards, map[string]any{"id": id, "files": []string{"pkg/" + id + "/x.go"}})
	}
	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()), map[string]any{"top_n": cards})
}

func TestPlanSelector_ExcludeKeepsTheItemOffEveryLane(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)

	got := plannedIDs(t, selectorEngine(h, Selector{Exclude: []string{"alpha-ready"}}), 2)

	if want := []string{"bravo-ready", "charlie-ready"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seed plan = %v, want %v — the highest-ranked item is excluded, so the next two take the lanes", got, want)
	}
}

func TestPlanSelector_OnlyAssignsNothingButTheListedItems(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)

	got := plannedIDs(t, selectorEngine(h, Selector{Only: []string{"charlie-ready", "delta-ready"}}), 2)

	if want := []string{"charlie-ready", "delta-ready"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seed plan = %v, want %v — only the listed ids may take a lane, however low they rank", got, want)
	}
}

func TestPlanSelector_ExcludeAlsoPrunesAPriorTriageDecisionAndTheWiden(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)
	withPriorDecision(t, h, "alpha-ready", "bravo-ready")

	got := plannedIDs(t, selectorEngine(h, Selector{Exclude: []string{"alpha-ready"}}), 2)

	if want := []string{"bravo-ready", "charlie-ready"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("companion-decision plan = %v, want %v — the prior decision names alpha-ready; the selector must prune it and the widen must not pull it back", got, want)
	}
}

func TestPlanSelector_OnlyAlsoFiltersAPriorTriageDecisionAndTheWiden(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)
	withPriorDecision(t, h, "alpha-ready", "bravo-ready")

	got := plannedIDs(t, selectorEngine(h, Selector{Only: []string{"charlie-ready", "delta-ready"}}), 2)

	if want := []string{"charlie-ready", "delta-ready"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("companion-decision plan = %v, want %v — a decision item outside --only must be pruned and the widen must draw only from the listed ids", got, want)
	}
}

func TestPlanSelector_ExcludeAlsoKeepsTheItemOutOfTheBacklogRefill(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)
	refill := selectorEngine(h, Selector{Exclude: []string{"alpha-ready"}}).refill()

	spec, ok := refill(map[string]bool{})

	if !ok || spec.Scope[0] != "bravo-ready" || spec.Env[ipcenv.FleetScopeKey] != "bravo-ready" {
		t.Fatalf("refill = %+v ok=%v, want bravo-ready — a stale lane must not be refilled with an excluded item", spec, ok)
	}
}

func TestPlanSelector_OnlyAlsoLimitsTheBacklogRefill(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)
	refill := selectorEngine(h, Selector{Only: []string{"delta-ready"}}).refill()

	spec, ok := refill(map[string]bool{})
	if !ok || spec.Scope[0] != "delta-ready" {
		t.Fatalf("refill = %+v ok=%v, want delta-ready — the only listed item", spec, ok)
	}
	if again, ok := refill(map[string]bool{"delta-ready": true}); ok {
		t.Fatalf("refill after excluding the only listed item = %+v, want none", again)
	}
}

func TestPlanSelector_TheZeroSelectorPlansLikeNoSelector(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)

	selected := plannedIDs(t, selectorEngine(h, Selector{}), 2)
	plain := plannedIDs(t, New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr), 2)

	if !reflect.DeepEqual(selected, plain) || !reflect.DeepEqual(plain, []string{"alpha-ready", "bravo-ready"}) {
		t.Fatalf("zero selector plan = %v, plain plan = %v, want both [alpha-ready bravo-ready]", selected, plain)
	}
}

func inboxFingerprint(t *testing.T, root string) map[string][sha256.Size]byte {
	t.Helper()
	out := map[string][sha256.Size]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			out[rel+"/"] = [sha256.Size]byte{}
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out[rel+"@"+info.ModTime().String()] = sha256.Sum256(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPlanSelector_PlanningChangesNoInboxFile(t *testing.T) {
	h := newHarness(t)
	selectorBacklog(t, h)
	withPriorDecision(t, h, "alpha-ready", "bravo-ready")
	inbox := filepath.Join(h.evolveDir, "inbox")
	before := inboxFingerprint(t, inbox)

	e := selectorEngine(h, Selector{Only: []string{"charlie-ready", "delta-ready"}, Exclude: []string{"alpha-ready"}})
	if got := plannedIDs(t, e, 2); !reflect.DeepEqual(got, []string{"charlie-ready", "delta-ready"}) {
		t.Fatalf("plan = %v, want [charlie-ready delta-ready]", got)
	}
	e.refill()(map[string]bool{})

	if after := inboxFingerprint(t, inbox); !reflect.DeepEqual(before, after) {
		t.Fatalf("the inbox changed under a selector: before=%d entries, after=%d entries — a selector scopes one launch and never edits an item", len(before), len(after))
	}
}
