package fixability_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
)

func TestClassify_Empty(t *testing.T) {
	d := fixability.Classify(fixability.Input{})
	if d.Stop || d.Reason != "" {
		t.Fatalf("unexpected stop: %+v", d)
	}
	if len(d.Lane) != 0 || len(d.Handoff) != 0 || len(d.Blockers) != 0 || len(d.ConsoleItems) != 0 {
		t.Fatalf("unexpected non-empty lists: %+v", d)
	}
}

func TestClassify_Severities(t *testing.T) {
	d := fixability.Classify(fixability.Input{
		Defects: []string{
			"C1: critical finding",
			"H1: high finding",
			"M1: medium finding",
			"L1: low finding",
			"X1: unknown finding",
			"bare finding without colon",
		},
	})
	if d.Stop {
		t.Fatalf("lane findings should not stop: %+v", d)
	}
	if len(d.Lane) != 6 {
		t.Fatalf("got %d lane findings, want 6", len(d.Lane))
	}
	wantSev := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "", ""}
	for i, f := range d.Lane {
		if f.Severity != wantSev[i] {
			t.Errorf("finding %d severity = %q, want %q", i, f.Severity, wantSev[i])
		}
	}
}

func TestClassify_ConsoleItemDuplicate(t *testing.T) {
	d := fixability.Classify(fixability.Input{
		Defects: []string{
			"H1: protected go/internal/guards/role.go fix is console item item-a item-b",
			"H2: protected go/internal/guards/role.go fix is console item item-a",
		},
		OpenConsoleItems: []string{"item-a", "item-b"},
	})
	if !d.Stop {
		t.Fatalf("expected stop: %+v", d)
	}
	if want := []string{"item-a", "item-b"}; !reflect.DeepEqual(d.ConsoleItems, want) {
		t.Fatalf("console items = %v, want %v", d.ConsoleItems, want)
	}
}
