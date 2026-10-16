package cycleoutcome

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
)

type recordingLifecycleLedger struct{ records []ledger.LifecycleRecord }

func (r *recordingLifecycleLedger) AppendLifecycle(_ context.Context, rec ledger.LifecycleRecord) error {
	r.records = append(r.records, rec)
	return nil
}

func TestApplyFailure_AppendsLifecycleThroughTheInjectedLedger(t *testing.T) {
	root, _, ws := seedProject(t, "poison", []string{"poison"})
	rec := &recordingLifecycleLedger{}
	if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Ceiling: 1, Stderr: io.Discard}.WithLedger(rec)); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if len(rec.records) == 0 {
		t.Fatal("the failure walk's lifecycle records must go through the injected ledger")
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "ledger.jsonl")); !os.IsNotExist(err) {
		t.Errorf("a self-constructed ledger wrote beside the injected one (stat err=%v)", err)
	}
}

func TestFailureInputs_WithLedgerReturnsACopy(t *testing.T) {
	in := FailureInputs{ProjectRoot: "/r", Cycle: 3, Ceiling: 2}
	rec := &recordingLifecycleLedger{}
	out := in.WithLedger(rec)
	if in.Ledger != nil {
		t.Error("WithLedger must not mutate its receiver")
	}
	if out.Ledger != rec || out.ProjectRoot != "/r" || out.Cycle != 3 || out.Ceiling != 2 {
		t.Errorf("WithLedger returns the inputs with the ledger set: %+v", out)
	}
}
