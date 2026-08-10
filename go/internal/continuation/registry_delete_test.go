package continuation

// registry_delete_test.go — pins DeleteRegistryEntry, the missing half of the
// registry lifecycle (2026-08-10 stall investigation: 84 immortal entries,
// ~60 pointing at dead state, no delete API anywhere). A binding the adopter
// declines must be releasable, or every lane on that scope auto-FAILs at the
// defect-ledger gate's out-of-band check forever (cycles 1412/1418 — the
// absorbing-FAIL state).

import "testing"

func TestDeleteRegistryEntry_RemovesOnlyTheNamedScope(t *testing.T) {
	root := t.TempDir()
	if err := WriteRegistryEntry(root, "scope-a", Continuation{Cycle: 1405, SnapshotSHA: "c4f6b41b"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteRegistryEntry(root, "scope-b", Continuation{Cycle: 1421, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}

	if err := DeleteRegistryEntry(root, "scope-a"); err != nil {
		t.Fatalf("DeleteRegistryEntry: %v", err)
	}
	if _, ok, _ := ReadRegistryEntry(root, "scope-a"); ok {
		t.Error("scope-a binding survived deletion — the absorbing-FAIL state cannot be released")
	}
	if c, ok, err := ReadRegistryEntry(root, "scope-b"); !ok || err != nil || c.Cycle != 1421 {
		t.Errorf("scope-b binding must be untouched, got ok=%v err=%v cycle=%d", ok, err, c.Cycle)
	}
}

func TestDeleteRegistryEntry_AbsentScopeIsACleanNoop(t *testing.T) {
	root := t.TempDir()
	if err := DeleteRegistryEntry(root, "never-existed"); err != nil {
		t.Errorf("deleting an absent binding must be a clean no-op, got %v", err)
	}
	if err := WriteRegistryEntry(root, "scope-a", Continuation{Cycle: 7, SnapshotSHA: "abc"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteRegistryEntry(root, "never-existed"); err != nil {
		t.Errorf("deleting an absent scope from a populated registry must be a clean no-op, got %v", err)
	}
}

func TestDeleteRegistryEntry_EmptyScopeRejected(t *testing.T) {
	if err := DeleteRegistryEntry(t.TempDir(), ""); err == nil {
		t.Error("empty scope id must be rejected (mirrors WriteRegistryEntry)")
	}
	if _, err := DeleteRegistryEntryIfCycle(t.TempDir(), "", 7); err == nil {
		t.Error("empty scope id must be rejected on the conditional path too")
	}
}

func TestDeleteRegistryEntryIfCycle_ReleasesOnlyTheNamedAncestor(t *testing.T) {
	// The TOCTOU pin (adversarial-review HIGH): a sibling lane that rebinds
	// the scope to a FRESH ancestor between an unlocked read and the delete
	// must keep its binding — check and delete share one lock hold.
	root := t.TempDir()
	if err := WriteRegistryEntry(root, "todo-42", Continuation{Cycle: 1409, SnapshotSHA: "fresh"}); err != nil {
		t.Fatal(err)
	}
	released, err := DeleteRegistryEntryIfCycle(root, "todo-42", 1405)
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Error("released=true for a binding naming a DIFFERENT ancestor — the guard is a rubber stamp")
	}
	if c, ok, _ := ReadRegistryEntry(root, "todo-42"); !ok || c.Cycle != 1409 {
		t.Errorf("the fresh cycle-1409 binding must survive a stale cycle-1405 release attempt, got ok=%v cycle=%d", ok, c.Cycle)
	}

	released, err = DeleteRegistryEntryIfCycle(root, "todo-42", 1409)
	if err != nil || !released {
		t.Fatalf("matching-ancestor release: released=%v err=%v, want true nil", released, err)
	}
	if _, ok, _ := ReadRegistryEntry(root, "todo-42"); ok {
		t.Error("matching binding survived its conditional release")
	}
}
