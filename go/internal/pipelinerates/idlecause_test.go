package pipelinerates

import (
	"math"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func statusAt(t *testing.T, r qualitypanel.Reading) string {
	t.Helper()
	for _, e := range qualitypanel.Events(r, qualitypanel.StageReport) {
		if e.Kind == "quality.reading" {
			return e.Fields["status"]
		}
	}
	t.Fatal("no reading event")
	return ""
}

func TestIdleCauseShare_GivesPlainCensusReadingsAndTheRegistryDecidesTheBreach(t *testing.T) {
	t.Parallel()
	flows := []WaveFlow{{Wave: 167, IdleHours: map[string]float64{"run_cap": 55, "family_held": 45}}, {Wave: 168, IdleHours: map[string]float64{"bench": 1, "run_cap": 1}}}

	readings := IdleCauseShare(flows)

	want := map[string]string{"wave:167/cause:run_cap": "breach", "wave:167/cause:family_held": "ok", "wave:168/cause:bench": "ok", "wave:168/cause:run_cap": "ok"}
	if len(readings) != len(want) {
		t.Fatalf("readings = %d, want %d", len(readings), len(want))
	}
	for _, r := range readings {
		if r.Index != "PQ8" || r.N != 1 || r.Threshold != nil || r.Status != "" || r.Source != "signals.ndjson" {
			t.Errorf("reading %+v, want a plain PQ8 census reading with no threshold and no status", r)
		}
		if got := statusAt(t, r); got != want[r.Window] {
			t.Errorf("%s judged %s, want %s: 0.55 is a breach and exactly 0.50 is ok", r.Window, got, want[r.Window])
		}
	}
}

func TestIdleCauseShare_SharesSumToOneAndCarryTheSlotHours(t *testing.T) {
	t.Parallel()

	readings := IdleCauseShare([]WaveFlow{{Wave: 1, IdleHours: map[string]float64{"run_cap": 6, "family_held": 2, "unknown": 2}}})

	got := map[string]float64{}
	for _, r := range readings {
		got[r.Window] = r.Interval.Value
	}
	if len(readings) != 3 || math.Abs(got["wave:1/cause:run_cap"]-0.6) > 1e-9 || math.Abs(got["wave:1/cause:unknown"]-0.2) > 1e-9 || readings[0].Extra["idle_slot_hours"] != "2.00" {
		t.Errorf("readings = %+v, want shares 0.6, 0.2, 0.2 in cause order with the slot hours", readings)
	}
}

func TestIdleCauseShare_AWaveWithNoPositiveIdleTotalGivesNoReading(t *testing.T) {
	t.Parallel()
	flows := []WaveFlow{{Wave: 1}, {Wave: 2, IdleHours: map[string]float64{"bench": 0}}, {Wave: 3, IdleHours: map[string]float64{"bench": -1}}}

	if readings := IdleCauseShare(flows); len(readings) != 0 {
		t.Errorf("readings = %+v, want none: a share over a zero or negative total is not a reading", readings)
	}
}
