package loopwave

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func focusBacklog(t *testing.T, h *harness) {
	t.Helper()
	for _, it := range []struct {
		id     string
		weight float64
		files  []string
	}{
		{"alpha-ready", 0.9, []string{"pkg/alpha/x.go"}},
		{"bravo-ready", 0.8, []string{"pkg/bravo/x.go"}},
		{"charlie-ready", 0.7, []string{"pkg/charlie/x.go"}},
		{"focus-one", 0.2, []string{"pkg/f1/x.go"}},
		{"focus-two", 0.1, []string{"pkg/f2/x.go"}},
	} {
		writeJSON(t, filepath.Join(h.evolveDir, "inbox", it.id+".json"),
			map[string]any{"id": it.id, "weight": it.weight, "priority_class": "feature", "files": it.files})
	}
}

func backlogOrder(e *Engine) []string {
	var ids []string
	for _, c := range e.backlog() {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestFocus_BacklogRanksFocusIdsFirstInTheGivenOrder(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)

	got := backlogOrder(selectorEngine(h, Selector{Focus: []string{"focus-two", "focus-one"}}))

	want := []string{"focus-two", "focus-one", "alpha-ready", "bravo-ready", "charlie-ready"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %v, want %v", got, want)
	}
}

func TestFocus_FocusIdsTakeTheFirstLanesAndOthersFillTheRest(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)

	got := plannedIDs(t, selectorEngine(h, Selector{Focus: []string{"focus-one", "focus-two"}}), 5)

	want := []string{"alpha-ready", "bravo-ready", "charlie-ready", "focus-one", "focus-two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %v, want %v", got, want)
	}
}

func TestFocus_AFocusIdThatAdmissionRefusesWaitsAndItsLaneGoesOnToTheNextItem(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)
	refill := selectorEngine(h, Selector{Focus: []string{"focus-one", "focus-two"}}).refill()

	waiting, ok := refill(map[string]bool{"focus-one": true})
	if !ok || waiting.Scope[0] != "focus-two" {
		t.Fatalf("refill with focus-one refused = %+v ok=%v, want focus-two, the next focus id", waiting, ok)
	}
	next, ok := refill(map[string]bool{"focus-one": true, "focus-two": true})
	if !ok || next.Scope[0] != "alpha-ready" {
		t.Fatalf("refill with both focus ids refused = %+v ok=%v, want alpha-ready, the next admissible item", next, ok)
	}
	again, ok := refill(map[string]bool{})
	if !ok || again.Scope[0] != "focus-one" {
		t.Fatalf("refill once focus-one is admissible = %+v ok=%v, want focus-one back at the head", again, ok)
	}
}

func TestFocus_ExcludeRemovesAFocusId(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)

	got := backlogOrder(selectorEngine(h, Selector{Focus: []string{"focus-one", "focus-two"}, Exclude: []string{"focus-one"}}))

	if got[0] != "focus-two" {
		t.Fatalf("backlog = %v, want focus-two first and focus-one absent", got)
	}
	for _, id := range got {
		if id == "focus-one" {
			t.Fatalf("backlog = %v, want focus-one removed", got)
		}
	}
}

func TestFocus_APlanKeepsTheFullPoolWidth(t *testing.T) {
	cases := []struct {
		name string
		sel  Selector
		want int
	}{
		{"zero", Selector{}, 2},
		{"only", Selector{Only: []string{"a"}}, 1},
		{"exclude", Selector{Exclude: []string{"a"}}, 1},
		{"focus", Selector{Focus: []string{"a"}}, 2},
		{"focus with exclude", Selector{Focus: []string{"a"}, Exclude: []string{"b"}}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sel.minLanes(); got != c.want {
				t.Fatalf("minLanes = %d, want %d", got, c.want)
			}
		})
	}
}

func TestFocus_APriorDecisionWidensFromTheFocusRankedBacklog(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)
	withPriorDecision(t, h, "alpha-ready")

	data, _, err := selectorEngine(h, Selector{Focus: []string{"focus-two"}}).PlanFn(2)(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for id := range topNIDs(t, data) {
		got[id] = true
	}
	if !got["alpha-ready"] || !got["focus-two"] {
		t.Fatalf("plan = %v, want alpha-ready kept and focus-two widened in", got)
	}
}

func TestFocus_ShieldProtectsFocusIdsAndKeepsTheStartedSet(t *testing.T) {
	started := map[string]bool{"live-one": true}
	got := Selector{Focus: []string{"focus-one"}}.shield(started)
	if !reflect.DeepEqual(got, map[string]bool{"live-one": true, "focus-one": true}) {
		t.Fatalf("shield = %v, want the started id and the focus id", got)
	}
	if len(started) != 1 {
		t.Fatalf("shield changed its input: %v", started)
	}
	if plain := (Selector{}).shield(started); !reflect.DeepEqual(plain, started) {
		t.Fatalf("zero selector shield = %v, want the input", plain)
	}
}

func TestFocus_AFocusIdAbsentFromTheBacklogIsSkipped(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)

	got := backlogOrder(selectorEngine(h, Selector{Focus: []string{"ghost-id", "focus-one"}}))

	if got[0] != "focus-one" || len(got) != 5 {
		t.Fatalf("backlog = %v, want focus-one first and the ghost id skipped", got)
	}
}

func topNOrderedIDs(t *testing.T, data []byte) []string {
	t.Helper()
	var d struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("unmarshal top_n: %v", err)
	}
	ids := make([]string, len(d.TopN))
	for i, c := range d.TopN {
		ids[i] = c.ID
	}
	return ids
}

func TestFocus_PriorDecisionAtCapacityStillLeadsWithFocusIDs(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)
	withPriorDecision(t, h, "alpha-ready", "bravo-ready")

	narrow, _, err := selectorEngine(h, Selector{Focus: []string{"focus-one", "focus-two"}}).PlanFn(2)(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := topNOrderedIDs(t, narrow), []string{"focus-one", "focus-two"}; !slices.Equal(got, want) {
		t.Fatalf("narrow plan = %v, want %v", got, want)
	}
	wide, _, err := selectorEngine(h, Selector{Focus: []string{"focus-one", "focus-two"}}).PlanFn(5)(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := topNOrderedIDs(t, wide), []string{"focus-one", "focus-two", "alpha-ready", "bravo-ready", "charlie-ready"}; !slices.Equal(got, want) {
		t.Fatalf("wide plan = %v, want %v", got, want)
	}
}

func TestFocus_PriorDecisionEdgeCasesPreserveInvariants(t *testing.T) {
	h := newHarness(t)
	focusBacklog(t, h)
	withPriorDecision(t, h, "focus-one", "alpha-ready")

	dedup, _, err := selectorEngine(h, Selector{Focus: []string{"focus-one"}}).PlanFn(2)(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := topNOrderedIDs(t, dedup), []string{"focus-one", "alpha-ready"}; !slices.Equal(got, want) {
		t.Fatalf("dedup plan = %v, want %v", got, want)
	}

	withPriorDecision(t, h, "bravo-ready", "alpha-ready")
	ghost, _, err := selectorEngine(h, Selector{Focus: []string{"ghost-id"}}).PlanFn(2)(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := topNOrderedIDs(t, ghost), []string{"bravo-ready", "alpha-ready"}; !slices.Equal(got, want) {
		t.Fatalf("ghost focus plan = %v, want %v", got, want)
	}

	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()), map[string]any{"committed_floors": []string{"alpha-ready"}})
	floors, _, err := selectorEngine(h, Selector{Focus: []string{"focus-one"}}).PlanFn(2)(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(topNOrderedIDs(t, floors), []string{}) {
		t.Fatalf("committed_floors gained top_n under focus: %s", floors)
	}
}

func focusTodos(t *testing.T, h *harness, sel Selector, count int) []fleet.Todo {
	t.Helper()
	e := selectorEngine(h, sel)
	data, _, err := e.PlanFn(count)(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	todos, _, err := fleet.TodosFromTriage(data, nil, e.RoutedResolver())
	if err != nil {
		t.Fatal(err)
	}
	return todos
}

func TestFocus_FiveLanePoolLaunchesFocusIDsFirstThenOthersInRankOrder(t *testing.T) {
	h := newHarness(t)
	inboxItem(t, h.evolveDir, "alpha-ready", 0.9, "go/internal/alpha/a.go")
	inboxItem(t, h.evolveDir, "bravo-ready", 0.8, "go/internal/bravo/b.go")
	inboxItem(t, h.evolveDir, "charlie-ready", 0.7, "go/internal/charlie/c.go")
	inboxItem(t, h.evolveDir, "first-focus", 0.2, "go/internal/f1/x.go")
	inboxItem(t, h.evolveDir, "second-focus", 0.1, "go/internal/f2/x.go")
	fc := enforceFC(5, 32, 5)
	todos := focusTodos(t, h, Selector{Focus: []string{"first-focus", "second-focus"}}, PoolSupply(fc))
	m := newLaneMeter()
	done := runMeteredPool(fc, orchestrator.AdmitByCell(admission.Admitter{Graph: dirCells{}}), todos, m)
	order := awaitLaunches(t, m, 5, done)
	close(m.release)
	awaitPool(t, done)
	if want := []string{"first-focus", "second-focus", "alpha-ready", "bravo-ready", "charlie-ready"}; !slices.Equal(order, want) {
		t.Fatalf("launch order = %v, want %v", order, want)
	}
}

func TestFocus_RefusedFocusIDWaitsThenTakesNextFreeLane(t *testing.T) {
	h := newHarness(t)
	inboxItem(t, h.evolveDir, "first-focus", 0.2, "go/internal/shared/a.go")
	inboxItem(t, h.evolveDir, "second-focus", 0.1, "go/internal/shared/b.go")
	inboxItem(t, h.evolveDir, "alpha-ready", 0.9, "go/internal/alpha/a.go")
	inboxItem(t, h.evolveDir, "bravo-ready", 0.8, "go/internal/bravo/b.go")
	fc := enforceFC(2, 32, 3)
	todos := focusTodos(t, h, Selector{Focus: []string{"first-focus", "second-focus"}}, PoolSupply(fc))

	release := map[string]chan struct{}{
		"first-focus": make(chan struct{}),
		"alpha-ready": make(chan struct{}),
	}
	launched := make(chan string, 4)
	launch := func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		id := spec.Scope[0]
		launched <- id
		if ch := release[id]; ch != nil {
			<-ch
		}
		return 0, nil
	}
	done := make(chan []fleet.Result, 1)
	pool := orchestrator.Pool{Target: fc.Count, Concurrency: fc.Concurrency, MaxLanes: fc.Orchestrator.RunMaxLanes, Admit: orchestrator.AdmitByCell(admission.Admitter{Graph: dirCells{}}), Launch: launch}
	go func() { done <- pool.Run(context.Background(), todos) }()

	first := []string{<-launched, <-launched}
	if !slices.Equal(first, []string{"first-focus", "alpha-ready"}) {
		close(release["first-focus"])
		close(release["alpha-ready"])
		t.Fatalf("initial two lanes = %v, want [first-focus alpha-ready] while second-focus waits on shared cell", first)
	}
	close(release["first-focus"])
	third := <-launched
	close(release["alpha-ready"])
	awaitPool(t, done)
	if third != "second-focus" {
		t.Fatalf("next free lane = %q, want second-focus once first-focus released the cell", third)
	}
}

func TestFocus_OverlappingFocusIDsKeepTheFullLaneWidth(t *testing.T) {
	h := newHarness(t)
	for _, it := range []struct {
		id     string
		weight float64
		file   string
	}{
		{"alpha-ready", 0.9, "pkg/alpha/x.go"},
		{"bravo-ready", 0.8, "pkg/bravo/x.go"},
		{"focus-one", 0.2, "pkg/shared/x.go"},
		{"focus-two", 0.1, "pkg/shared/x.go"},
	} {
		writeJSON(t, filepath.Join(h.evolveDir, "inbox", it.id+".json"),
			map[string]any{"id": it.id, "weight": it.weight, "priority_class": "feature", "files": []string{it.file}})
	}
	withPriorDecision(t, h, "alpha-ready", "bravo-ready")

	todos := focusTodos(t, h, Selector{Focus: []string{"focus-one", "focus-two"}}, 3)
	specs, _ := fleet.PlanCycles(todos, 3)

	if len(specs) != 3 {
		t.Fatalf("lanes = %d from %v, want 3: the second focus id waits and the next disjoint item takes its lane", len(specs), todos)
	}
	if todos[0].ID != "focus-one" {
		t.Fatalf("todos = %v, want focus-one first", todos)
	}
}
