package loopwave

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func writeInbox(t *testing.T, h *harness, id string, weight float64, files []string, deps ...string) {
	t.Helper()
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "2026-10-01T00-00-00Z-"+id+".json"),
		map[string]any{"id": id, "weight": weight, "files": files, "deps": deps})
}

func waveFleet(count int) policy.FleetConfig {
	return policy.FleetConfig{Count: count, Concurrency: count, MinLanes: 1, PlanSource: "triage"}
}

func laneIDs(p WavePlan) [][]string {
	out := [][]string{}
	for _, lane := range p.Lanes {
		ids := []string{}
		for _, it := range lane.Items {
			ids = append(ids, it.ID)
		}
		out = append(out, ids)
	}
	return out
}

func TestPreview_LanesAndWeightsEqualWhatDispatchLaunches(t *testing.T) {
	h := newHarness(t)
	writeInbox(t, h, "a1", 0.9, []string{"pkg/a/a.go"})
	writeInbox(t, h, "a2", 0.7, []string{"pkg/a/a.go"})
	writeInbox(t, h, "b1", 0.8, []string{"pkg/b/b.go"})
	fc := waveFleet(2)

	plan := h.e.Preview(context.Background(), fc, nil, nil, budgethistory.Throughput{}, time.Time{})

	launcher := &recorder{}
	if _, err := h.e.Dispatch(context.Background(), DispatchRequest{
		Config: fc, Wave: 1, Preflight: func() error { return nil },
		Plan: h.e.PlanFn(2), Launcher: launcher, Routed: h.e.RoutedResolver(),
	}); err != nil {
		t.Fatal(err)
	}
	var want [][]string
	for _, s := range launcher.calls[0] {
		want = append(want, slices.Clone(s.Scope))
	}
	if got := laneIDs(plan); !slices.EqualFunc(got, want, slices.Equal[[]string]) || len(want) != 2 {
		t.Errorf("preview lanes %v, dispatch lanes %v", got, want)
	}
	if plan.Width != 2 || plan.Reason == "" {
		t.Errorf("width=%d reason=%q", plan.Width, plan.Reason)
	}
	weights := map[string]float64{}
	for _, lane := range plan.Lanes {
		for _, it := range lane.Items {
			weights[it.ID] = it.Weight
		}
	}
	if weights["a1"] != 0.9 || weights["b1"] != 0.8 {
		t.Errorf("weights = %v", weights)
	}
}

func TestPreview_ExcludesUnmetDependencyAndConsoleRoutedItems(t *testing.T) {
	h := newHarness(t)
	writeInbox(t, h, "a1", 0.9, []string{"pkg/a/a.go"})
	writeInbox(t, h, "b1", 0.8, []string{"pkg/b/b.go"})
	writeInbox(t, h, "z9", 0.99, []string{"pkg/z/z.go"}, "b1")
	writeInbox(t, h, "p1", 0.95, []string{"protected/p.go"})

	plan := h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})

	reasons := map[string]string{}
	for _, ex := range plan.Excluded {
		reasons[ex.ID] = ex.Reason
	}
	if reasons["z9"] == "" || !slices.ContainsFunc(laneIDs(plan), func(l []string) bool { return slices.Contains(l, "a1") }) {
		t.Errorf("excluded=%v lanes=%v", plan.Excluded, laneIDs(plan))
	}
	if reasons["p1"] == "" {
		t.Errorf("console-routed p1 missing from excluded: %v", plan.Excluded)
	}
	for _, lane := range laneIDs(plan) {
		if slices.Contains(lane, "z9") || slices.Contains(lane, "p1") {
			t.Errorf("excluded item printed in lane %v", lane)
		}
	}
}

func TestPreview_ReleasesNoStaleClaim(t *testing.T) {
	h := newHarness(t)
	writeInbox(t, h, "a1", 0.9, []string{"pkg/a/a.go"})
	writeInbox(t, h, "b1", 0.8, []string{"pkg/b/b.go"})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "processing", "cycle-7", "2026-10-01T00-00-00Z-s1.json"),
		map[string]any{"id": "s1", "weight": 0.99, "files": []string{"pkg/s/s.go"}})
	claim := filepath.Join(h.evolveDir, "inbox", "processing", "cycle-7", "2026-10-01T00-00-00Z-s1.json")

	h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})

	if _, err := os.Stat(claim); err != nil {
		t.Errorf("Preview released the stale claim")
	}
}

func TestPreview_NoWaveFleetsPrintEmptyPlanWithReason(t *testing.T) {
	h := newHarness(t)
	for name, fc := range map[string]policy.FleetConfig{
		"sequential": waveFleet(1),
		"plan":       {Count: 2, PlanSource: "inbox"},
		"pool":       {Count: 2, PlanSource: "triage", Scheduling: "pool"},
	} {
		plan := h.e.Preview(context.Background(), fc, nil, nil, budgethistory.Throughput{}, time.Time{})
		if len(plan.Lanes) != 0 || plan.Reason == "" || plan.Width != fc.Count {
			t.Errorf("%s: %+v", name, plan)
		}
	}
}

func TestPreview_EmptyInboxReportsWhyTheWaveCannotFill(t *testing.T) {
	h := newHarness(t)
	plan := h.e.Preview(context.Background(), waveFleet(2), nil, nil, budgethistory.Throughput{}, time.Time{})
	if len(plan.Lanes) != 0 || plan.Reason == "" || len(plan.Excluded) != 0 {
		t.Errorf("%+v", plan)
	}
}

func TestWidthReason_NamesTheShrinkWhenBenched(t *testing.T) {
	if got := widthReason(policy.FleetConfig{Count: 3}, 3, nil); got == "" {
		t.Error("unshrunk reason empty")
	}
	if got := widthReason(policy.FleetConfig{Count: 3}, 2, []string{"codex"}); !strings.Contains(got, "codex") || !strings.Contains(got, "fleet.count=3") || !strings.Contains(got, "2 lane") {
		t.Errorf("shrunk reason = %q", got)
	}
}

func TestPreview_PlanTypesCarryLaneItemsAndExclusions(t *testing.T) {
	plan := WavePlan{
		Width:    1,
		Lanes:    []PlanLane{{Items: []PlanItem{{ID: "x1", Weight: 0.5}}}},
		Excluded: []PlanExclusion{{ID: "x2", Reason: "waiting on x1"}},
	}
	if got := laneIDs(plan); len(got) != 1 || !slices.Equal(got[0], []string{"x1"}) {
		t.Fatalf("lane ids = %v, want [[x1]]", got)
	}
	if plan.Lanes[0].Items[0].Weight != 0.5 {
		t.Fatalf("weight = %v, want 0.5", plan.Lanes[0].Items[0].Weight)
	}
	if plan.Excluded[0].ID != "x2" || plan.Excluded[0].Reason != "waiting on x1" {
		t.Fatalf("exclusion = %+v", plan.Excluded[0])
	}
}

func TestWidthReason_NamesTheBudgetWhenNothingIsBenched(t *testing.T) {
	fc := policy.FleetConfig{Count: 4, Budget: &policy.FleetBudgetConfig{Stage: "enforce"}}
	got := widthReason(fc, 3, nil)
	if !strings.Contains(got, "fleet budget") || !strings.Contains(got, "enforce") || !strings.Contains(got, "3 lane") {
		t.Errorf("budget reason = %q", got)
	}
}
