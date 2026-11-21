package wavehealth

import (
	"strings"
	"testing"
)

func TestReportText_OneRowForEachStepAndTheOutcomeLast(t *testing.T) {
	t.Parallel()
	r := Report{
		Steps: []Step{
			{ID: "A1", Reading: "loop running, brake not engaged", Threshold: "not running ends Part A", Result: ResultOK},
			{ID: "A5", Reading: "3 LOOP_ORCH_LANE_STALLED line(s)", Threshold: "3 line(s) is a STALL", Result: ResultBrake, Action: "run the BRAKE procedure"},
		},
		Outcome: OutcomeBrake,
	}
	want := strings.Join([]string{
		"A1       OK        reading: loop running, brake not engaged | threshold: not running ends Part A",
		"A5       BRAKE     reading: 3 LOOP_ORCH_LANE_STALLED line(s) | threshold: 3 line(s) is a STALL",
		"    action: run the BRAKE procedure",
		"outcome: BRAKE",
		"",
	}, "\n")
	if got := r.Text(); got != want {
		t.Fatalf("Text() =\n%s\nwant\n%s", got, want)
	}
}

func TestReportText_ARowWithNoActionHasNoActionLine(t *testing.T) {
	t.Parallel()
	r := Report{Steps: []Step{{ID: "A1", Reading: "r", Threshold: "t", Result: ResultOK}}, Outcome: OutcomeContinue}
	if strings.Contains(r.Text(), "action:") {
		t.Fatalf("Text() must not print an action line for an OK row:\n%s", r.Text())
	}
}
