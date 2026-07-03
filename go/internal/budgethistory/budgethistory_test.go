package budgethistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// writeRun materializes one cycle's run workspace under
// projectRoot/.evolve/runs/cycle-<n>: a phase-timing.json built from the given
// per-phase durations, plus (when costUSD > 0) a *-events.ndjson result
// envelope carrying that cost — the exact on-disk layout Collect walks.
func writeRun(t *testing.T, root string, cycle int, durationsMS []int64, costUSD float64) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir run ws: %v", err)
	}
	entries := make([]phasetiming.Entry, 0, len(durationsMS))
	for i, d := range durationsMS {
		entries = append(entries, phasetiming.Entry{
			Phase:      fmt.Sprintf("phase%d", i),
			DurationMS: d,
			Verdict:    "PASS",
			CostUSD:    0, // per-phase cost stays $0 (subscription); batch cost comes from the event log
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal timing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatalf("write timing: %v", err)
	}
	if costUSD > 0 {
		writeEventLog(t, root, cycle, costUSD)
	}
}

// writeEventLog drops a single-result *-events.ndjson (the cyclecost source)
// into an existing cycle workspace, carrying the given cost — including $0, to
// exercise the "genuine subscription $0" vs "no event log at all" distinction.
func writeEventLog(t *testing.T, root string, cycle int, costUSD float64) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	line := fmt.Sprintf(`{"kind":"result","data":{"cost_usd":%g,"tokens":{"in":10,"out":20}}}`+"\n", costUSD)
	if err := os.WriteFile(filepath.Join(ws, "build-events.ndjson"), []byte(line), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
}

// oneHour is a tiny fixture helper: a single-phase cycle of exactly one hour.
func oneHour() []int64 { return []int64{3_600_000} }

func TestCollect_MedianDurationAndThroughput(t *testing.T) {
	root := t.TempDir()
	// Three cycles of 1h, 30m, 2h → sorted [30m, 1h, 2h] → median = 1h.
	writeRun(t, root, 10, []int64{3_600_000}, 0)
	writeRun(t, root, 11, []int64{1_800_000}, 0)
	writeRun(t, root, 12, []int64{7_200_000}, 0)

	got := Collect(root, []int{10, 11, 12})

	if got.SampleCount != 3 {
		t.Errorf("SampleCount = %d, want 3", got.SampleCount)
	}
	if got.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 3600000", got.MedianCycleDurationMS)
	}
	// One median-length (1h) cycle per hour → 1.0 cycles/hour per lane.
	if got.CyclesPerHour != 1.0 {
		t.Errorf("CyclesPerHour = %v, want 1.0", got.CyclesPerHour)
	}
	if got.MedianCostUSD != 0 {
		t.Errorf("MedianCostUSD = %v, want 0 (subscription)", got.MedianCostUSD)
	}
}

func TestCollect_SumsPhasesPerCycle(t *testing.T) {
	root := t.TempDir()
	// A single cycle whose phases sum to 1h → median == that sum.
	writeRun(t, root, 5, []int64{1_200_000, 1_200_000, 1_200_000}, 0)

	got := Collect(root, []int{5})

	if got.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 3600000 (phases summed)", got.MedianCycleDurationMS)
	}
	if got.CyclesPerHour != 1.0 {
		t.Errorf("CyclesPerHour = %v, want 1.0", got.CyclesPerHour)
	}
}

func TestCollect_EvenCountAveragesMiddle(t *testing.T) {
	root := t.TempDir()
	// Four cycles → median averages the two middle values: (2000+3000)/2 = 2500.
	writeRun(t, root, 1, []int64{1_000}, 0)
	writeRun(t, root, 2, []int64{2_000}, 0)
	writeRun(t, root, 3, []int64{3_000}, 0)
	writeRun(t, root, 4, []int64{4_000}, 0)

	got := Collect(root, []int{1, 2, 3, 4})

	if got.MedianCycleDurationMS != 2_500 {
		t.Errorf("MedianCycleDurationMS = %d, want 2500 (avg of two middles)", got.MedianCycleDurationMS)
	}
}

func TestCollect_MissingCyclesAreAbsentEvidence(t *testing.T) {
	root := t.TempDir()
	// Request three, but only 20 and 22 exist on disk. 21 is absent evidence,
	// skipped — never an error.
	writeRun(t, root, 20, oneHour(), 0)
	writeRun(t, root, 22, oneHour(), 0)

	got := Collect(root, []int{20, 21, 22})

	if got.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2 (missing cycle skipped)", got.SampleCount)
	}
	if got.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 3600000", got.MedianCycleDurationMS)
	}
}

func TestCollect_AllMissingIsZeroValue(t *testing.T) {
	root := t.TempDir()
	got := Collect(root, []int{100, 101})

	if got.SampleCount != 0 {
		t.Errorf("SampleCount = %d, want 0", got.SampleCount)
	}
	if got.MedianCycleDurationMS != 0 || got.CyclesPerHour != 0 {
		t.Errorf("zero-evidence Throughput = %+v, want all-zero (no fabricated pace)", got)
	}
}

func TestCollect_ZeroDurationCycleNotCounted(t *testing.T) {
	root := t.TempDir()
	// A cycle whose phases all took 0ms carries no measurable pace → skipped,
	// so it can't produce a divide-by-zero CyclesPerHour.
	writeRun(t, root, 7, []int64{0, 0}, 0)
	writeRun(t, root, 8, oneHour(), 0)

	got := Collect(root, []int{7, 8})

	if got.SampleCount != 1 {
		t.Errorf("SampleCount = %d, want 1 (zero-duration cycle excluded)", got.SampleCount)
	}
	if got.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 3600000", got.MedianCycleDurationMS)
	}
}

// TestCollect_CostCarriedButNotSizing is the plan's core invariant: cost is
// display-only, never the sizing input. Two batches with IDENTICAL durations
// but DIFFERENT costs must yield the SAME MedianCycleDurationMS + CyclesPerHour
// and only differ in MedianCostUSD.
func TestCollect_CostCarriedButNotSizing(t *testing.T) {
	cheapRoot := t.TempDir()
	writeRun(t, cheapRoot, 1, oneHour(), 0) // subscription $0
	writeRun(t, cheapRoot, 2, oneHour(), 0)

	dearRoot := t.TempDir()
	writeRun(t, dearRoot, 1, oneHour(), 5.00) // metered, same durations
	writeRun(t, dearRoot, 2, oneHour(), 5.00)

	cheap := Collect(cheapRoot, []int{1, 2})
	dear := Collect(dearRoot, []int{1, 2})

	if cheap.MedianCycleDurationMS != dear.MedianCycleDurationMS {
		t.Errorf("duration differs by cost: cheap=%d dear=%d — cost must not size",
			cheap.MedianCycleDurationMS, dear.MedianCycleDurationMS)
	}
	if cheap.CyclesPerHour != dear.CyclesPerHour {
		t.Errorf("throughput differs by cost: cheap=%v dear=%v — cost must not size",
			cheap.CyclesPerHour, dear.CyclesPerHour)
	}
	if dear.MedianCostUSD != 5.00 {
		t.Errorf("MedianCostUSD = %v, want 5.00 (carried for display)", dear.MedianCostUSD)
	}
	if dear.CostSampleCount != 2 {
		t.Errorf("dear CostSampleCount = %d, want 2 (both cycles logged cost)", dear.CostSampleCount)
	}
	// cheap cycles have timing but no event log → cost is absent evidence.
	if cheap.MedianCostUSD != 0 || cheap.CostSampleCount != 0 {
		t.Errorf("cheap cost = %v over %d samples, want 0 over 0 (no event log = no cost data)",
			cheap.MedianCostUSD, cheap.CostSampleCount)
	}
}

// TestCollect_CostSampleCountDistinguishesNoDataFromZero pins the reviewer's
// honesty concern: a genuine subscription $0 (event log present, cost 0) is
// counted in CostSampleCount, while a cycle with no event log at all is NOT —
// so a caller can tell "$0 measured" from "cost unknown", both of which show
// MedianCostUSD == 0.
func TestCollect_CostSampleCountDistinguishesNoDataFromZero(t *testing.T) {
	root := t.TempDir()
	writeRun(t, root, 1, oneHour(), 0) // timing only...
	writeEventLog(t, root, 1, 0)       // ...plus a genuine $0 event log
	writeRun(t, root, 2, oneHour(), 0) // timing only, no event log

	got := Collect(root, []int{1, 2})

	if got.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2 (both have timing)", got.SampleCount)
	}
	if got.CostSampleCount != 1 {
		t.Errorf("CostSampleCount = %d, want 1 (only cycle 1 logged cost)", got.CostSampleCount)
	}
	if got.MedianCostUSD != 0 {
		t.Errorf("MedianCostUSD = %v, want 0 (genuine subscription $0)", got.MedianCostUSD)
	}
}
