package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestDecideAfterRetro_NthOccurrenceForcesAdapt(t *testing.T) {
	led := recurrence.NewLedger()
	pol := recurrence.DefaultEscalationPolicy()
	// Same pattern closed out in two cycles => count=2 (Nth-occurrence).
	if err := led.RecordClosure("loop-fatal", 1, nil, nil, pol); err != nil {
		t.Fatal(err)
	}
	if err := led.RecordClosure("loop-fatal", 2, nil, nil, pol); err != nil {
		t.Fatal(err)
	}

	got := escalateRetroReason("proceed: no failures requiring adaptation", "loop-fatal", led)
	if strings.HasPrefix(got, "proceed:") {
		t.Fatalf("Nth-occurrence pattern must not emit bare 'proceed': %q", got)
	}
	if !strings.Contains(got, "adapt: escalated") || !strings.Contains(got, "count=2") {
		t.Fatalf("want adapt-with-escalation wording, got %q", got)
	}

	// A single-occurrence (or unseen) pattern keeps the deterministic branch.
	if got := escalateRetroReason("proceed: none", "unseen-pattern", led); got != "proceed: none" {
		t.Fatalf("single/zero-occurrence must stay 'proceed', got %q", got)
	}
}
