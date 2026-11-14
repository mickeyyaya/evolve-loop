package qualitypanel

import (
	"math"
	"testing"
)

var baselineUtilization = []float64{0.98, 0.92, 0.97, 0.97, 0.94, 0.98, 0.97, 0.90, 0.97, 0.96, 0.84, 0.82}

func TestUtilizationChart_MarksWave164(t *testing.T) {
	t.Parallel()
	var chart XmRChart
	chart, ok := NewXmRChart(baselineUtilization)
	if !ok {
		t.Fatal("chart not built")
	}
	if math.Abs(chart.Center-0.935) > 1e-9 || math.Abs(chart.LCL-0.8189) > 1e-3 {
		t.Errorf("center=%v lcl=%v, want 0.935 and about 0.819", chart.Center, chart.LCL)
	}
	for i, v := range baselineUtilization {
		if chart.Outside(v) {
			t.Errorf("baseline wave %d at %v is marked, want in control", i, v)
		}
	}
	if !chart.Outside(0.49) {
		t.Error("wave 164 at 49% is not marked, want rule 1")
	}
	if !chart.Outside(1.2) {
		t.Error("a point above the upper limit is not marked")
	}
}

func TestUtilizationReading_MarksTheWaveAndDoesNotBreachAtShadow(t *testing.T) {
	t.Parallel()
	chart, _ := NewXmRChart(baselineUtilization)
	if got := chart.Rule1(0.49); got != "outside" {
		t.Errorf("Rule1(0.49) = %q, want outside", got)
	}
	out := Reading{Index: "PQ7", Interval: Interval{Value: 0.49}, Method: MethodCensus, N: 1, Window: "wave:164",
		Extra: map[string]string{XmRRule1Field: chart.Rule1(0.49)}}
	for _, stage := range []Stage{StageShadow, StageReport, StageGate} {
		for _, e := range Events(out, stage) {
			if e.Kind == "quality.breach" || e.Kind == "quality_breach" {
				t.Errorf("stage %s emitted a breach: %+v", stage, e)
			}
		}
	}
	if got := chart.Rule1(0.95); got != "in_control" {
		t.Errorf("Rule1(0.95) = %q, want in_control", got)
	}
}

func TestNewXmRChart_NeedsTwoPoints(t *testing.T) {
	t.Parallel()
	if _, ok := NewXmRChart([]float64{0.9}); ok {
		t.Error("one point built a chart")
	}
}
