//go:build acs

package cycle2113

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var evolveBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "evolve-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := func() int {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		root, err := acsassert.ModuleDir(wd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		evolveBinary = filepath.Join(dir, "evolve")
		build := exec.Command("go", "build", "-o", evolveBinary, "./cmd/evolve")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build evolve: %v\n%s", err, out)
			return 1
		}
		return m.Run()
	}()
	os.RemoveAll(dir)
	os.Exit(code)
}

type inboxItem struct {
	ID     string   `json:"id"`
	Weight float64  `json:"weight"`
	Files  []string `json:"files"`
	Deps   []string `json:"deps,omitempty"`
}

type fixture struct {
	fleetCount int
	items      []inboxItem
	staleClaim *inboxItem
	budget     map[string]any
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	fleetBlock := map[string]any{"count": f.fleetCount, "concurrency": f.fleetCount, "min_lanes": 1, "plan_source": "triage"}
	if f.budget != nil {
		fleetBlock["budget"] = f.budget
	}
	writeJSON(t, filepath.Join(evolveDir, "policy.json"), map[string]any{"fleet": fleetBlock})
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, it := range f.items {
		writeJSON(t, filepath.Join(evolveDir, "inbox", "2026-10-01T00-00-00Z-"+it.ID+".json"), it)
	}
	if f.staleClaim != nil {
		name := "2026-10-01T00-00-00Z-" + f.staleClaim.ID + ".json"
		writeJSON(t, filepath.Join(evolveDir, "inbox", "processing", "cycle-7", name), f.staleClaim)
	}
	return root
}

var fourItems = fixture{fleetCount: 2, items: []inboxItem{
	{ID: "a1", Weight: 0.9, Files: []string{"pkg/a/a.go"}},
	{ID: "a2", Weight: 0.7, Files: []string{"pkg/a/a.go"}},
	{ID: "b1", Weight: 0.8, Files: []string{"pkg/b/b.go"}},
	{ID: "c1", Weight: 0.6, Files: []string{"pkg/c/c.go"}},
}}

var sixItemsThreeLanes = fixture{fleetCount: 3, items: []inboxItem{
	{ID: "d1", Weight: 0.95, Files: []string{"pkg/d/d.go"}},
	{ID: "e1", Weight: 0.85, Files: []string{"pkg/e/e.go"}},
	{ID: "e2", Weight: 0.75, Files: []string{"pkg/e/e.go"}},
	{ID: "f1", Weight: 0.65, Files: []string{"pkg/f/f.go"}},
	{ID: "g1", Weight: 0.55, Files: []string{"pkg/g/g.go"}},
	{ID: "h1", Weight: 0.45, Files: []string{"pkg/h/h.go"}},
}}

type recordingLauncher struct{ specs []fleet.CycleSpec }

func (r *recordingLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	r.specs = specs
	results := make([]fleet.Result, len(specs))
	for i := range specs {
		results[i] = fleet.Result{Index: i}
	}
	return results
}

func engineDispatchedLanes(t *testing.T, f fixture) [][]string {
	t.Helper()
	root := f.project(t)
	evolveDir := filepath.Join(root, ".evolve")
	ports := loopwave.Ports{
		LastCycle: func(context.Context) (int, error) { return 0, nil },
		Workspace: func(cycle int) string { return filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle)) },
		Protected: func(string) bool { return false },
		Shrink:    fleet.QuotaAwareCount,
	}
	engine := loopwave.New(loopwave.RootsOf(root), ports, io.Discard)
	launcher := &recordingLauncher{}
	out, err := engine.Dispatch(context.Background(), loopwave.DispatchRequest{
		Config:    policy.FleetConfig{Count: f.fleetCount, Concurrency: f.fleetCount, MinLanes: 1, PlanSource: "triage"},
		Wave:      1,
		Preflight: func() error { return nil },
		Plan:      engine.PlanFn(f.fleetCount),
		Launcher:  launcher,
		Routed:    engine.RoutedResolver(),
	})
	if err != nil || !out.Ran {
		t.Fatalf("fixture invalid: the wave engine did not dispatch (ran=%v err=%v)", out.Ran, err)
	}
	lanes := make([][]string, len(launcher.specs))
	for i, s := range launcher.specs {
		lanes[i] = slices.Clone(s.Scope)
	}
	return lanes
}

type dryRunPlan struct {
	WavePlan *struct {
		Width  int    `json:"width"`
		Reason string `json:"reason"`
		Lanes  []struct {
			Items []struct {
				ID     string  `json:"id"`
				Weight float64 `json:"weight"`
			} `json:"items"`
		} `json:"lanes"`
		Excluded []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"excluded"`
	} `json:"wave_plan"`
}

type dryRunResult struct {
	plan   dryRunPlan
	stdout string
	stderr string
	code   int
}

func dryRun(t *testing.T, project string) dryRunResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(evolveBinary, "loop", "--dry-run", "fix bug")
	cmd.Dir = project
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := dryRunResult{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run evolve loop --dry-run: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &res.plan); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nstdout: %s\nstderr tail: %s", err, acsassert.OutputTail(res.stdout, 20), acsassert.OutputTail(res.stderr, 10))
	}
	return res
}

func (r dryRunResult) laneIDs() [][]string {
	lanes := [][]string{}
	for _, lane := range r.plan.WavePlan.Lanes {
		ids := []string{}
		for _, it := range lane.Items {
			ids = append(ids, it.ID)
		}
		lanes = append(lanes, ids)
	}
	return lanes
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			state[rel+"/"] = "dir"
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		state[rel] = fmt.Sprintf("%x", sha256.Sum256(raw))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestC2113_001_DryRunPrintsWidthReasonAndEachLanesItemsWithWeights(t *testing.T) {
	project := fourItems.project(t)
	res := dryRun(t, project)
	if res.code != 0 {
		t.Fatalf("exit=%d, want 0; stderr tail: %s", res.code, acsassert.OutputTail(res.stderr, 10))
	}
	if res.plan.WavePlan == nil {
		t.Fatalf("RED: dry-run prints the config only; no wave_plan key in: %s", acsassert.OutputTail(res.stdout, 20))
	}
	wp := res.plan.WavePlan
	if wp.Width != 2 || strings.TrimSpace(wp.Reason) == "" {
		t.Errorf("width=%d reason=%q, want width 2 (fleet.count) and a non-empty reason", wp.Width, wp.Reason)
	}
	if len(wp.Lanes) != 2 {
		t.Fatalf("lanes=%d, want 2: %+v", len(wp.Lanes), wp.Lanes)
	}
	weights := map[string]float64{}
	for _, it := range fourItems.items {
		weights[it.ID] = it.Weight
	}
	laneOf := map[string]int{}
	for i, lane := range wp.Lanes {
		if len(lane.Items) == 0 {
			t.Errorf("lane %d is empty", i)
		}
		for _, it := range lane.Items {
			laneOf[it.ID] = i
			if want, ok := weights[it.ID]; !ok || it.Weight != want {
				t.Errorf("lane %d item %q weight=%v, want the inbox weight %v", i, it.ID, it.Weight, want)
			}
		}
	}
	laneA, a1ok := laneOf["a1"]
	if !a1ok || laneOf["a2"] != laneA {
		t.Errorf("a1 and a2 share pkg/a/a.go and must print in one lane: a1=%v a2=%d", a1ok, laneOf["a2"])
	}
	if b, ok := laneOf["b1"]; !ok || b == laneA {
		t.Errorf("b1 must own the other lane: lane of b1=%d (ok=%v), lane of a1=%d", b, ok, laneA)
	}
}

func TestC2113_002_DryRunPlanEqualsWhatTheWaveEngineDispatches(t *testing.T) {
	for name, f := range map[string]fixture{"two-lane": fourItems, "three-lane": sixItemsThreeLanes} {
		t.Run(name, func(t *testing.T) {
			want := engineDispatchedLanes(t, f)
			res := dryRun(t, f.project(t))
			if res.plan.WavePlan == nil {
				t.Fatalf("RED: dry-run prints the config only; no wave_plan key in: %s", acsassert.OutputTail(res.stdout, 20))
			}
			if got := res.laneIDs(); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
				t.Errorf("dry-run lanes %v differ from the lanes the wave engine dispatches %v", got, want)
			}
			if res.plan.WavePlan.Width != f.fleetCount {
				t.Errorf("width=%d, want %d", res.plan.WavePlan.Width, f.fleetCount)
			}
		})
	}
}

func TestC2113_003_DryRunWritesNoClaimLeaseOrCycleState(t *testing.T) {
	staleHolder := inboxItem{ID: "s1", Weight: 0.99, Files: []string{"pkg/s/s.go"}}
	f := fourItems
	f.staleClaim = &staleHolder

	probe := f.project(t)
	before := snapshot(t, probe)
	planProbe := loopwave.New(loopwave.RootsOf(probe), loopwave.Ports{
		LastCycle: func(context.Context) (int, error) { return 0, nil },
		Workspace: func(cycle int) string { return filepath.Join(probe, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle)) },
		Protected: func(string) bool { return false },
		Shrink:    fleet.QuotaAwareCount,
	}, io.Discard)
	if _, _, err := planProbe.PlanFn(2)(context.Background(), 1); err != nil {
		t.Fatalf("fixture invalid: engine PlanFn: %v", err)
	}
	if slices.Equal(mapKeys(before), mapKeys(snapshot(t, probe))) {
		t.Fatalf("fixture invalid: the engine's own PlanFn released no stale claim, so the dry-run could not be seen to write one")
	}

	project := f.project(t)
	before = snapshot(t, project)
	res := dryRun(t, project)
	if res.plan.WavePlan == nil {
		t.Fatalf("RED: dry-run prints the config only; no wave_plan key in: %s", acsassert.OutputTail(res.stdout, 20))
	}
	after := snapshot(t, project)
	for path, sum := range before {
		if after[path] != sum {
			t.Errorf("dry-run changed %s", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("dry-run created %s", path)
		}
	}
	claim := filepath.Join(project, ".evolve", "inbox", "processing", "cycle-7", "2026-10-01T00-00-00Z-s1.json")
	if !acsassert.FileExists(t, claim) {
		t.Errorf("the stale claim was released by a dry run")
	}
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestC2113_004_SequentialFleetPrintsAnEmptyPlanWithItsReason(t *testing.T) {
	f := fourItems
	f.fleetCount = 1
	project := f.project(t)
	before := snapshot(t, project)
	res := dryRun(t, project)
	if res.code != 0 || res.plan.WavePlan == nil {
		t.Fatalf("RED: exit=%d wave_plan present=%v; stdout: %s", res.code, res.plan.WavePlan != nil, acsassert.OutputTail(res.stdout, 20))
	}
	wp := res.plan.WavePlan
	if len(wp.Lanes) != 0 || strings.TrimSpace(wp.Reason) == "" {
		t.Errorf("a sequential fleet dispatches no wave: lanes=%d reason=%q, want 0 lanes and a reason", len(wp.Lanes), wp.Reason)
	}
	if !sameSnapshot(before, snapshot(t, project)) {
		t.Errorf("dry-run changed the project tree")
	}
}

func TestC2113_005_EmptyInboxWaveFleetPrintsNoLanesAndFailsNothing(t *testing.T) {
	f := fixture{fleetCount: 2}
	project := f.project(t)
	before := snapshot(t, project)
	res := dryRun(t, project)
	if res.code != 0 || res.plan.WavePlan == nil {
		t.Fatalf("RED: exit=%d wave_plan present=%v; stdout: %s", res.code, res.plan.WavePlan != nil, acsassert.OutputTail(res.stdout, 20))
	}
	wp := res.plan.WavePlan
	if len(wp.Lanes) != 0 || strings.TrimSpace(wp.Reason) == "" {
		t.Errorf("empty inbox: lanes=%d reason=%q, want 0 lanes and the reason the wave cannot fill", len(wp.Lanes), wp.Reason)
	}
	if !sameSnapshot(before, snapshot(t, project)) {
		t.Errorf("dry-run changed the project tree")
	}
}

func TestC2113_006_ItemWaitingOnAnUnmetDependencyIsExcludedWithItsReason(t *testing.T) {
	f := fourItems
	f.items = append(slices.Clone(fourItems.items), inboxItem{ID: "z9", Weight: 0.99, Files: []string{"pkg/z/z.go"}, Deps: []string{"b1"}})
	for _, lane := range engineDispatchedLanes(t, f) {
		if slices.Contains(lane, "z9") {
			t.Fatalf("fixture invalid: the wave engine dispatches z9 although it waits on pending b1")
		}
	}
	res := dryRun(t, f.project(t))
	if res.plan.WavePlan == nil {
		t.Fatalf("RED: dry-run prints the config only; no wave_plan key in: %s", acsassert.OutputTail(res.stdout, 20))
	}
	for _, lane := range res.laneIDs() {
		if slices.Contains(lane, "z9") {
			t.Errorf("z9 is printed in a lane %v although the engine would not dispatch it", lane)
		}
	}
	found := false
	for _, ex := range res.plan.WavePlan.Excluded {
		if ex.ID == "z9" {
			found = true
			if !strings.Contains(ex.Reason, "b1") {
				t.Errorf("excluded z9 reason=%q, want it to name the unmet dependency b1", ex.Reason)
			}
		}
	}
	if !found {
		t.Errorf("z9 missing from wave_plan.excluded: %+v", res.plan.WavePlan.Excluded)
	}
}

func sameSnapshot(a, b map[string]string) bool {
	return slices.Equal(mapKeys(a), mapKeys(b)) && func() bool {
		for k, v := range a {
			if b[k] != v {
				return false
			}
		}
		return true
	}()
}

var fourDisjointItems = []inboxItem{
	{ID: "m1", Weight: 0.9, Files: []string{"pkg/m/m.go"}},
	{ID: "n1", Weight: 0.8, Files: []string{"pkg/n/n.go"}},
	{ID: "o1", Weight: 0.7, Files: []string{"pkg/o/o.go"}},
	{ID: "q1", Weight: 0.6, Files: []string{"pkg/q/q.go"}},
}

var budgetNow = time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

func budgetFleet(stage string) policy.FleetConfig {
	return policy.FleetConfig{
		Count: 4, Concurrency: 4, MinLanes: 1, PlanSource: "triage",
		Budget: &policy.FleetBudgetConfig{Stage: stage, CapacityCycles: 200, Safety: 0.6, HistoryWindow: 10},
	}
}

func halfSpentClaudeQuota() []quotastate.QuotaState {
	return []quotastate.QuotaState{{
		Family: "claude",
		Source: quotastate.SourceProbed,
		Buckets: []quotastate.Bucket{{
			Name:         "week",
			UsedFraction: 0.5,
			ResetAt:      budgetNow.Add(20 * time.Hour),
		}},
	}}
}

func hourPerCyclePace() budgethistory.Throughput {
	return budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: 3_600_000, CyclesPerHour: 1}
}

func engineFor(root string) *loopwave.Engine {
	evolveDir := filepath.Join(root, ".evolve")
	return loopwave.New(loopwave.RootsOf(root), loopwave.Ports{
		LastCycle: func(context.Context) (int, error) { return 0, nil },
		Workspace: func(cycle int) string { return filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle)) },
		Protected: func(string) bool { return false },
		Shrink:    fleet.QuotaAwareCount,
	}, io.Discard)
}

func dispatchedLanesAt(t *testing.T, engine *loopwave.Engine, sized policy.FleetConfig) [][]string {
	t.Helper()
	launcher := &recordingLauncher{}
	out, err := engine.Dispatch(context.Background(), loopwave.DispatchRequest{
		Config:    sized,
		Wave:      1,
		Preflight: func() error { return nil },
		Plan:      engine.PlanFn(sized.Count),
		Launcher:  launcher,
		Routed:    engine.RoutedResolver(),
	})
	if err != nil || !out.Ran {
		t.Fatalf("fixture invalid: the wave engine did not dispatch (ran=%v err=%v)", out.Ran, err)
	}
	lanes := make([][]string, len(launcher.specs))
	for i, s := range launcher.specs {
		lanes[i] = slices.Clone(s.Scope)
	}
	return lanes
}

func previewLaneIDs(p loopwave.WavePlan) [][]string {
	lanes := [][]string{}
	for _, lane := range p.Lanes {
		ids := []string{}
		for _, it := range lane.Items {
			ids = append(ids, it.ID)
		}
		lanes = append(lanes, ids)
	}
	return lanes
}

func TestC2113_007_EnforcedBudgetSizesThePreviewWidthAsTheWaveSizesIt(t *testing.T) {
	root := fixture{fleetCount: 4, items: fourDisjointItems}.project(t)
	engine := engineFor(root)
	fc := budgetFleet("enforce")
	states, tp := halfSpentClaudeQuota(), hourPerCyclePace()

	sized, _ := engine.Size(fc, nil, states, tp, budgetNow)
	if sized.Count != 3 {
		t.Fatalf("fixture invalid: Engine.Size must shrink 4 lanes to 3 under this quota, got %d", sized.Count)
	}
	want := dispatchedLanesAt(t, engine, sized)
	if len(want) != 3 {
		t.Fatalf("fixture invalid: the engine dispatches %d lane(s) at the sized width, want 3", len(want))
	}

	plan := engine.Preview(context.Background(), fc, nil, states, tp, budgetNow)

	if plan.Width != sized.Count {
		t.Errorf("preview width=%d, want %d: the width the wave engine sizes with fleet.budget.stage=enforce", plan.Width, sized.Count)
	}
	if got := previewLaneIDs(plan); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("preview lanes %v differ from the lanes dispatched at the sized width %v", got, want)
	}
	if !strings.Contains(strings.ToLower(plan.Reason), "budget") {
		t.Errorf("reason=%q, want it to say the width came from the fleet budget", plan.Reason)
	}
}

func TestC2113_008_ShadowBudgetHoldsThePreviewWidthAtTheConfiguredCount(t *testing.T) {
	root := fixture{fleetCount: 4, items: fourDisjointItems}.project(t)
	engine := engineFor(root)
	fc := budgetFleet("shadow")
	states, tp := halfSpentClaudeQuota(), hourPerCyclePace()

	sized, _ := engine.Size(fc, nil, states, tp, budgetNow)
	want := dispatchedLanesAt(t, engine, sized)

	plan := engine.Preview(context.Background(), fc, nil, states, tp, budgetNow)

	if plan.Width != 4 || sized.Count != 4 {
		t.Errorf("preview width=%d size=%d, want both 4: stage shadow only logs the budget size", plan.Width, sized.Count)
	}
	if got := previewLaneIDs(plan); !slices.EqualFunc(got, want, slices.Equal[[]string]) || len(want) != 4 {
		t.Errorf("preview lanes %v, dispatched lanes %v, want the same four", got, want)
	}
}

func TestC2113_009_DryRunCliSizesThroughTheBudgetLayerWithoutProbingOrWriting(t *testing.T) {
	enforce := fixture{fleetCount: 2, items: fourItems.items, budget: map[string]any{"stage": "enforce", "capacity_cycles": 200, "safety_fraction": 0.6}}
	project := enforce.project(t)
	before := snapshot(t, project)

	res := dryRun(t, project)

	if res.code != 0 || res.plan.WavePlan == nil {
		t.Fatalf("RED: exit=%d wave_plan present=%v; stderr tail: %s", res.code, res.plan.WavePlan != nil, acsassert.OutputTail(res.stderr, 10))
	}
	if !strings.Contains(res.stderr, "[budget] enforce: sizing wave to") {
		t.Errorf("the dry run did not reach Engine.Size with fleet.budget.stage=enforce; stderr tail: %s", acsassert.OutputTail(res.stderr, 10))
	}
	if strings.Contains(res.stderr, "probing") {
		t.Errorf("a dry run must not probe live quota; stderr tail: %s", acsassert.OutputTail(res.stderr, 10))
	}
	if wp := res.plan.WavePlan; wp.Width != 2 || len(wp.Lanes) != 2 {
		t.Errorf("with no quota signal the enforced budget holds the configured width: width=%d lanes=%d, want 2 and 2", wp.Width, len(wp.Lanes))
	}
	if !sameSnapshot(before, snapshot(t, project)) {
		t.Errorf("dry-run changed the project tree")
	}
}

func TestC2113_010_DryRunWithoutABudgetBlockStaysSilentAboutTheBudget(t *testing.T) {
	project := fourItems.project(t)

	res := dryRun(t, project)

	if res.code != 0 || res.plan.WavePlan == nil {
		t.Fatalf("RED: exit=%d wave_plan present=%v", res.code, res.plan.WavePlan != nil)
	}
	if strings.Contains(res.stderr, "[budget]") {
		t.Errorf("no fleet.budget block, yet the dry run wrote budget lines: %s", acsassert.OutputTail(res.stderr, 10))
	}
}
