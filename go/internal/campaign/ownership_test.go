package campaign

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func sampleOwner(pid int, wt string) Owner {
	return Owner{PID: pid, Worktree: wt, Host: "host-a", StartedAt: "2026-06-21T23:00:00Z"}
}

func TestAcquireOwnership_FreshSucceedsAndRecordsOwner(t *testing.T) {
	dir := t.TempDir()
	lease, err := AcquireOwnership(dir, "hashA", sampleOwner(111, "/wt/a"))
	if err != nil {
		t.Fatalf("AcquireOwnership: %v", err)
	}
	if lease == nil {
		t.Fatal("a successful acquire must return a lease")
	}
	defer lease.Release()

	got, ok := ReadOwner(dir, "hashA")
	if !ok {
		t.Fatal("ReadOwner must find the recorded owner after acquire")
	}
	if got.PID != 111 || got.Worktree != "/wt/a" {
		t.Fatalf("recorded owner = %+v, want PID 111 worktree /wt/a", got)
	}
	if got.GoalHash != "hashA" {
		t.Fatalf("acquire must stamp GoalHash, got %q", got.GoalHash)
	}
}

func TestAcquireOwnership_SecondReturnsHeldErrorWithLiveOwner(t *testing.T) {
	dir := t.TempDir()
	lease, err := AcquireOwnership(dir, "hashA", sampleOwner(111, "/wt/a"))
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer lease.Release()

	lease2, err2 := AcquireOwnership(dir, "hashA", sampleOwner(222, "/wt/b"))
	if lease2 != nil {
		t.Fatal("a held campaign must not grant a second lease")
	}
	var held *HeldError
	if !errors.As(err2, &held) {
		t.Fatalf("second acquire must return *HeldError, got %v", err2)
	}
	if held.Owner.PID != 111 || held.Owner.Worktree != "/wt/a" {
		t.Fatalf("HeldError must report the LIVE owner (PID 111 /wt/a), got %+v", held.Owner)
	}
}

func TestAcquireOwnership_ReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	lease, err := AcquireOwnership(dir, "hashA", sampleOwner(111, "/wt/a"))
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	lease.Release()

	lease2, err2 := AcquireOwnership(dir, "hashA", sampleOwner(222, "/wt/b"))
	if err2 != nil {
		t.Fatalf("re-acquire after release must succeed, got %v", err2)
	}
	defer lease2.Release()
	if got, _ := ReadOwner(dir, "hashA"); got.PID != 222 {
		t.Fatalf("re-acquire must overwrite the owner, got PID %d", got.PID)
	}
}

func TestAcquireOwnership_DifferentGoalHashesAreIndependent(t *testing.T) {
	dir := t.TempDir()
	a, err := AcquireOwnership(dir, "hashA", sampleOwner(111, "/wt/a"))
	if err != nil {
		t.Fatalf("acquire A: %v", err)
	}
	defer a.Release()
	b, err := AcquireOwnership(dir, "hashB", sampleOwner(222, "/wt/b"))
	if err != nil {
		t.Fatalf("a different goal-hash must acquire independently, got %v", err)
	}
	defer b.Release()
}

func TestReadOwner_AbsentReturnsFalse(t *testing.T) {
	if _, ok := ReadOwner(t.TempDir(), "never-leased"); ok {
		t.Fatal("ReadOwner on a never-leased hash must report ok=false")
	}
}

func TestAcquireOwnership_CreatesMissingLeaseDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does", "not", "exist", "yet")
	lease, err := AcquireOwnership(dir, "hashA", sampleOwner(111, "/wt/a"))
	if err != nil {
		t.Fatalf("acquire must create a missing lease dir, got %v", err)
	}
	defer lease.Release()
	if _, ok := ReadOwner(dir, "hashA"); !ok {
		t.Fatal("owner must be readable after creating the lease dir")
	}
}

func TestOwnershipLease_PublicTypeIsNamed(t *testing.T) {
	// apicover: name the exported lease type in-package — it is otherwise only
	// reachable through AcquireOwnership's inferred return type.
	var _ *OwnershipLease
}

func TestHeldError_MessageNamesOwner(t *testing.T) {
	e := &HeldError{Owner: Owner{PID: 4242, Worktree: "/wt/x", GoalHash: "abcdef1234567890"}}
	msg := e.Error()
	for _, want := range []string{"4242", "/wt/x"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("HeldError message %q must name %q", msg, want)
		}
	}
}
