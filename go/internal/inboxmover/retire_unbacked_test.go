package inboxmover

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRetireUnbacked_RetiresOnlyIDsWithNoLifecycleEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "pending", nil)
	writeTask(t, filepath.Join(inbox, "processed", "cycle-3"), "done", nil)
	var stderr bytes.Buffer
	opts := Options{ProjectRoot: root, Stderr: &stderr}

	retired, err := RetireUnbacked(opts, 7, StateProcessed, "ship-promote-processed: no inbox item backs the id", "51edfb7f", []string{"pending", "done", "ghost", "ghost", ""})

	if err != nil || len(retired) != 1 || retired[0] != "ghost" {
		t.Fatalf("retired = %v, %v; want only the id with no evidence, once", retired, err)
	}
	if ds := ResolveDispatchState(opts, "ghost"); ds.State != StateProcessed || ds.Detail != "cycle-7" {
		t.Errorf("ghost is now processed in cycle-7: %+v", ds)
	}
	if ds := ResolveDispatchState(opts, "pending"); ds.State != StatePending {
		t.Errorf("the pending item is untouched: %+v", ds)
	}
	if ds := ResolveDispatchState(opts, "done"); ds.State != StateProcessed || ds.Detail != "cycle-3" {
		t.Errorf("the processed item keeps its own record: %+v", ds)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("retired unbacked: ghost")) {
		t.Errorf("the console line names the id: %q", stderr.String())
	}
}

func TestRetireUnbacked_ReportsAWriteFaultAndRetiresTheRest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "rejected"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root}

	retired, err := RetireUnbacked(opts, 7, StateRejected, "planned no-work", "", []string{"ghost"})

	if err == nil || len(retired) != 0 {
		t.Fatalf("retired = %v, err = %v; want the write fault and no retirement", retired, err)
	}
	if ds := ResolveDispatchState(opts, "ghost"); ds.State != StateUnknown {
		t.Errorf("a failed write leaves no evidence: %+v", ds)
	}
}
