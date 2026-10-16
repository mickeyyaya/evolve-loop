package cyclebudget_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
)

func TestNext_OffStageNeverDecides(t *testing.T) {
	for _, backlog := range []int{0, 5} {
		d := cyclebudget.Next(cyclebudget.Off, 1, 3, backlog, true)
		if d.Stop || d.Advisory || d.Reason != "" {
			t.Fatalf("Off must be a no-op, got %+v (backlog=%d)", d, backlog)
		}
	}
}

func TestNext_EnforceStopsOnCompletion(t *testing.T) {
	d := cyclebudget.Next(cyclebudget.Enforce, 2, 25, 0, false)
	if !d.Stop || d.Reason != "goal_complete" {
		t.Fatalf("drained backlog must stop with goal_complete, got %+v", d)
	}
	d = cyclebudget.Next(cyclebudget.Enforce, 1, 25, 7, true)
	if !d.Stop || d.Reason != "goal_complete" {
		t.Fatalf("advisor goal-complete must stop, got %+v", d)
	}
}

func TestNext_EnforceContinuesWhileWorkRemains(t *testing.T) {
	d := cyclebudget.Next(cyclebudget.Enforce, 1, 25, 4, false)
	if d.Stop || d.Reason != "" {
		t.Fatalf("work remaining (backlog=4) must continue, got %+v", d)
	}
}

func TestNext_EnforceStopsAtCap(t *testing.T) {
	d := cyclebudget.Next(cyclebudget.Enforce, 25, 25, 9, false)
	if !d.Stop || d.Reason != "cap" {
		t.Fatalf("at cap must stop with cap, got %+v", d)
	}
	d = cyclebudget.Next(cyclebudget.Enforce, 25, 25, 0, false)
	if !d.Stop || d.Reason != "goal_complete" {
		t.Fatalf("completion must win over cap, got %+v", d)
	}
}

func TestNext_AdvisoryNeverStopsButReports(t *testing.T) {
	d := cyclebudget.Next(cyclebudget.Advisory, 2, 25, 0, false)
	if d.Stop {
		t.Fatalf("advisory must not stop the loop, got %+v", d)
	}
	if !d.Advisory || d.Reason != "goal_complete" {
		t.Fatalf("advisory must report the would-stop reason, got %+v", d)
	}
	d = cyclebudget.Next(cyclebudget.Advisory, 1, 25, 3, false)
	if d.Stop || d.Advisory || d.Reason != "" {
		t.Fatalf("advisory with work left must be silent, got %+v", d)
	}
}

func TestParseStage(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want cyclebudget.Stage
	}{
		{"", cyclebudget.Off},
		{"off", cyclebudget.Off},
		{"advisory", cyclebudget.Advisory},
		{"enforce", cyclebudget.Enforce},
		{"bogus", cyclebudget.Off},
	} {
		if got := cyclebudget.ParseStage(tc.in); got != tc.want {
			t.Errorf("ParseStage(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if cyclebudget.Enforce.String() != "enforce" {
		t.Errorf("Stage.String() = %q, want enforce", cyclebudget.Enforce.String())
	}
}

func TestDecision_ZeroValueIsNoStop(t *testing.T) {
	if got := cyclebudget.Next(cyclebudget.Off, 5, 3, 0, true); got != (cyclebudget.Decision{}) {
		t.Errorf("Off stage must return zero cyclebudget.Decision, got %+v", got)
	}
}
