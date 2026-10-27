package fixability_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
)

func TestApicoverNamed(t *testing.T) {
	if fixability.ReasonHandoffBlocker != "handoff_blocker" {
		t.Fatal(fixability.ReasonHandoffBlocker)
	}
	var in fixability.Input
	var f fixability.Finding
	if f.ID != "" || f.Severity != "" || f.Text != "" || len(f.ConsoleItems) != 0 {
		t.Fatal(f)
	}
	var d fixability.Decision
	if d.Stop || d.Reason != "" || len(d.Lane) != 0 || len(d.Handoff) != 0 || len(d.Blockers) != 0 || len(d.ConsoleItems) != 0 || d.Salvage != "" {
		t.Fatal(d)
	}
	got := fixability.Classify(in)
	if got.Stop {
		t.Fatal(got)
	}
}
