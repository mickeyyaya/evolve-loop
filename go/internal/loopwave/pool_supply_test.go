package loopwave

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

type dirCells struct{}

func (dirCells) Packages(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, path.Dir(f))
	}
	return out
}

func (dirCells) Deps(string) []string { return nil }

func (dirCells) Hubs() map[string]bool { return map[string]bool{} }

type laneMeter struct {
	mu       sync.Mutex
	live     int
	peak     int
	total    int
	launched chan string
	release  chan struct{}
}

func newLaneMeter() *laneMeter {
	return &laneMeter{launched: make(chan string, 64), release: make(chan struct{})}
}

func (m *laneMeter) launch(ctx context.Context, spec fleet.CycleSpec) (int, error) {
	m.mu.Lock()
	m.live++
	m.total++
	m.peak = max(m.peak, m.live)
	m.mu.Unlock()
	select {
	case m.launched <- spec.Scope[0]:
	case <-ctx.Done():
	}
	select {
	case <-m.release:
	case <-ctx.Done():
	}
	m.mu.Lock()
	m.live--
	m.mu.Unlock()
	return 0, nil
}

func (m *laneMeter) counts() (peak, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.peak, m.total
}

func enforceFC(count, scanK, maxLanes int) policy.FleetConfig {
	fc := waveFC(count)
	fc.Orchestrator = policy.OrchestratorConfig{Stage: "enforce", ScanK: scanK, HubMinImporters: 120, RunMaxLanes: maxLanes}
	return fc
}

func inboxItem(t *testing.T, evolveDir, id string, weight float64, file string) {
	t.Helper()
	writeJSON(t, filepath.Join(evolveDir, "inbox", id+".json"),
		map[string]any{"id": id, "weight": weight, "priority_class": "feature", "files": []string{file}})
}

func hotCellBacklog(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.ports.LastCycle = func(context.Context) (int, error) { return 3, nil }
	h.e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	inboxItem(t, h.evolveDir, "rank0", 0.99, "go/internal/hot/rank0.go")
	for i := 1; i <= 4; i++ {
		inboxItem(t, h.evolveDir, fmt.Sprintf("hot%d", i), 0.95-float64(i)/100, fmt.Sprintf("go/internal/hot/f%d.go", i))
	}
	for i := 1; i <= 3; i++ {
		inboxItem(t, h.evolveDir, fmt.Sprintf("cold%d", i), 0.5-float64(i)/100, fmt.Sprintf("go/internal/cold%d/c.go", i))
	}
	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()),
		map[string]any{"top_n": []map[string]any{{"id": "rank0", "files": []string{"go/internal/hot/rank0.go"}}}})
	return h
}

func poolBacklog(t *testing.T, h *harness, fc policy.FleetConfig) []fleet.Todo {
	t.Helper()
	data, _, err := h.e.PlanFn(PoolSupply(fc))(context.Background(), 129)
	if err != nil {
		t.Fatal(err)
	}
	todos, _, err := fleet.TodosFromTriage(data, nil, h.e.RoutedResolver())
	if err != nil {
		t.Fatal(err)
	}
	return todos
}

func runMeteredPool(fc policy.FleetConfig, admit orchestrator.Admit, backlog []fleet.Todo, m *laneMeter) <-chan []fleet.Result {
	done := make(chan []fleet.Result, 1)
	pool := orchestrator.Pool{Target: fc.Count, Concurrency: fc.Concurrency, MaxLanes: fc.Orchestrator.RunMaxLanes, Admit: admit, Launch: m.launch}
	go func() { done <- pool.Run(context.Background(), backlog) }()
	return done
}

const poolWait = 30 * time.Second

func awaitLaunches(t *testing.T, m *laneMeter, n int, done <-chan []fleet.Result) []string {
	t.Helper()
	var got []string
	timeout := time.After(poolWait)
	for len(got) < n {
		select {
		case id := <-m.launched:
			got = append(got, id)
		case <-done:
			t.Fatalf("the pool ended after %d of %d concurrent launches: %v", len(got), n, got)
		case <-timeout:
			close(m.release)
			t.Fatalf("timed out after %d of %d concurrent launches: %v", len(got), n, got)
		}
	}
	return got
}

func awaitPool(t *testing.T, done <-chan []fleet.Result) []fleet.Result {
	t.Helper()
	select {
	case results := <-done:
		return results
	case <-time.After(poolWait):
		t.Fatal("the pool did not end after its lanes were released")
		return nil
	}
}

func TestPoolSupply_AtEnforceAHotRankZeroCellStillFillsTheTarget(t *testing.T) {
	h := hotCellBacklog(t)
	fc := enforceFC(4, 32, 16)
	backlog := poolBacklog(t, h, fc)
	m := newLaneMeter()
	done := runMeteredPool(fc, orchestrator.AdmitByCell(admission.Admitter{Graph: dirCells{}}), backlog, m)

	live := awaitLaunches(t, m, 4, done)
	close(m.release)
	awaitPool(t, done)
	sort.Strings(live)
	if want := []string{"cold1", "cold2", "cold3", "rank0"}; !slices.Equal(live, want) {
		t.Fatalf("live lanes before any exit = %v, want rank0 plus three cold lanes %v: the pool was supplied %d todos %v", live, want, len(backlog), todoIDs(backlog))
	}
}

func TestPoolSupply_TheShadowAndOffStagesKeepTheFleetCount(t *testing.T) {
	for _, stage := range []string{"shadow", ""} {
		fc := enforceFC(4, 32, 16)
		fc.Orchestrator.Stage = stage
		if got := PoolSupply(fc); got != 4 {
			t.Errorf("PoolSupply(stage %q) = %d, want the fleet count 4", stage, got)
		}
		h := hotCellBacklog(t)
		if backlog := poolBacklog(t, h, fc); len(backlog) != 4 || backlog[0].ID != "rank0" {
			t.Errorf("stage %q: the plan holds %v, want the count-wide widen of today (rank0 and three hot todos)", stage, todoIDs(backlog))
		}
	}
}

func TestPoolSupply_AtEnforceTheDepthIsTheLargerOfCountAndScanK(t *testing.T) {
	for _, tc := range []struct{ count, scanK, want int }{{4, 32, 32}, {8, 6, 8}, {4, 4, 4}} {
		if got := PoolSupply(enforceFC(tc.count, tc.scanK, 4*tc.count)); got != tc.want {
			t.Errorf("PoolSupply(count %d, scan_k %d) = %d, want %d", tc.count, tc.scanK, got, tc.want)
		}
	}
}

func TestPoolSupply_ADeepSupplyStaysInsideTheTargetAndTheRunCap(t *testing.T) {
	h := newHarness(t)
	for i := range 10 {
		inboxItem(t, h.evolveDir, fmt.Sprintf("item%02d", i), 0.9-float64(i)/100, fmt.Sprintf("go/internal/p%02d/x.go", i))
	}
	fc := enforceFC(2, 32, 5)
	backlog := poolBacklog(t, h, fc)
	if len(backlog) != 10 {
		t.Fatalf("the deep supply holds %v, want all 10 ranked todos", todoIDs(backlog))
	}
	m := newLaneMeter()
	done := runMeteredPool(fc, orchestrator.AdmitByCell(admission.Admitter{Graph: dirCells{}}), backlog, m)
	awaitLaunches(t, m, fc.Count, done)
	close(m.release)
	results := awaitPool(t, done)
	peak, total := m.counts()
	if peak != fc.Count {
		t.Errorf("peak live lanes = %d, want exactly the target %d", peak, fc.Count)
	}
	if total != fc.Orchestrator.RunMaxLanes {
		t.Errorf("launched %d lanes, want exactly the run cap %d", total, fc.Orchestrator.RunMaxLanes)
	}
	if len(results) != len(backlog) {
		t.Errorf("results = %d, want one per supplied todo (%d)", len(results), len(backlog))
	}
}
