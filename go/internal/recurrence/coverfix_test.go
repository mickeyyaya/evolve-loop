package recurrence_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestRecordClosure_AZeroValueLedgerGrowsItsEntryMap(t *testing.T) {
	t.Parallel()
	var l recurrence.Ledger

	if err := l.RecordClosure("pattern:z", 4, nil, nil, recurrence.DefaultEscalationPolicy()); err != nil {
		t.Fatalf("RecordClosure on a zero ledger: %v", err)
	}

	if got := l.Count("pattern:z"); got != 1 {
		t.Fatalf("Count after the first closure on a zero ledger = %d, want 1", got)
	}
}

type failingAutofiler struct{}

func (failingAutofiler) Autofile(string, int) error { return errors.New("autofile refused") }

func TestRecordClosure_AnAutofileFailureSurfacesAndLeavesThePatternUnfiled(t *testing.T) {
	t.Parallel()
	l := recurrence.NewLedger()
	pol := recurrence.DefaultEscalationPolicy()
	_ = l.RecordClosure("pattern:f", 1, nil, nil, pol)

	err := l.RecordClosure("pattern:f", 2, nil, failingAutofiler{}, pol)

	if err == nil {
		t.Fatal("an autofile failure must surface")
	}
	if l.Entries["pattern:f"].Autofiled {
		t.Fatal("a pattern whose autofile failed must not be marked filed")
	}
}
