package recurrence_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestApplyBoundary_AWeightWriteFailureSurfacesAndStampsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	path := seedItem(t, inboxDir, "x", 0.10)
	stage(t, root, dispositionrouter.ActionEscalate, "x", "pattern:x", 3, 0.50)
	if err := os.Chmod(inboxDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inboxDir, 0o755) })
	if probe, err := os.Create(filepath.Join(inboxDir, "probe")); err == nil {
		probe.Close()
		t.Skip("directory permissions are not enforced here")
	}

	res, err := recurrence.ApplyBoundary(opts(root, 5, false))

	if err == nil {
		t.Fatalf("a failed weight write must surface, got res=%+v", res)
	}
	if len(res.Bumped) != 0 {
		t.Fatalf("nothing may be reported bumped on a failed write: %+v", res)
	}
	if got := readWeight(t, path); got != 0.10 {
		t.Fatalf("the item weight changed despite the failure: %v", got)
	}
}

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
