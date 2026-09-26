package inboxmover

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCycleOutcome_PassRetiresACommittedIDNoInboxItemBacks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "backed", nil)
	opts := Options{ProjectRoot: root, Stderr: io.Discard, IsLandedFn: func(string) (bool, error) { return true, nil }}

	res, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 7, Passed: true, CommittedIDs: []string{"backed", "ghost"}, CommitSHA: "51edfb7fabcd"})

	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 1 || res.Promoted[0] != "backed" || len(res.RetiredUnbacked) != 1 || res.RetiredUnbacked[0] != "ghost" {
		t.Fatalf("promoted = %v, retired unbacked = %v", res.Promoted, res.RetiredUnbacked)
	}
	if ds := ResolveDispatchState(opts, "ghost"); ds.State != StateProcessed || ds.Detail != "cycle-7" {
		t.Errorf("the shipped id is processed in cycle-7: %+v", ds)
	}
	if path, _ := FindFileByTaskID(filepath.Join(inbox, "processed", "cycle-7"), "ghost"); filepath.Base(path) != "51edfb7f-ghost.json" {
		t.Errorf("the record carries the ship sha like a promoted item: %s", path)
	}

	again, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 8, Passed: true, CommittedIDs: []string{"ghost"}})

	if err != nil || len(again.Promoted) != 0 || len(again.RetiredUnbacked) != 0 {
		t.Errorf("evidence already stands, so a later PASS retires nothing: %+v %v", again, err)
	}
}

func TestApplyCycleOutcome_FailRetiresNoUnbackedID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	opts := Options{ProjectRoot: root, Stderr: io.Discard}

	_, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 7, Passed: false, CommittedIDs: []string{"ghost"}})

	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if ds := ResolveDispatchState(opts, "ghost"); ds.State != StateUnknown {
		t.Errorf("a failed cycle keeps the id retryable: %+v", ds)
	}
}

func TestApplyCycleOutcome_PassNeverRetiresAnItemItCouldNotMove(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeTask(t, inbox, "backed", nil)
	blocker := filepath.Join(inbox, "processed", "cycle-7", "51edfb7f-2026-07-13T00-00-00Z-backed.json")
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: io.Discard, IsLandedFn: func(string) (bool, error) { return true, nil }}

	res, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 7, Passed: true, CommittedIDs: []string{"backed"}, CommitSHA: "51edfb7fabcd"})

	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 0 || len(res.RetiredUnbacked) != 0 {
		t.Fatalf("a move that failed is neither a promotion nor an unbacked id: %+v", res)
	}
	if ds := ResolveDispatchState(opts, "backed"); ds.State != StatePending {
		t.Errorf("the item stays where it was, with its own lifecycle: %+v", ds)
	}
}
