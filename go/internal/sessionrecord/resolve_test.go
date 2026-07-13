package sessionrecord

import (
	"os"
	"testing"
)

// TDD RED (cycle-806, task sweep-tombstone-attribution).
//
// ReadAllResolving and ReapedSuffix do not yet exist → this file fails to
// compile until Builder adds the tombstone-aware resolver. Contract: after
// ReapOrphans tombstones a fully-reaped registry (rename <path> →
// <path>+ReapedSuffix, sessionreaper.go:92-95), a caller must STILL be able to
// attribute the killed sessions to their owning run. Attribution is preserved
// in the `.reaped` tombstone, not lost — but the live-path ReadAll can no
// longer discover it. ReadAllResolving closes that gap by unioning the live
// registry with its tombstone, deduped by session id.

// AC1.1 — a session recorded then tombstoned (live path renamed to .reaped)
// must still resolve. Behavioral: exercises the resolver against a real
// on-disk tombstone produced the way ReapOrphans produces it.
func TestReadAllResolving_ReadsTombstoneAfterReap(t *testing.T) {
	dir := t.TempDir()
	path := PathIn(dir)
	rec := Record{Session: "evolve-bridge-x", RunID: "run1", Cycle: 1}
	if err := Append(path, rec); err != nil {
		t.Fatal(err)
	}
	// Simulate the reaper's tombstone: the live registry no longer exists.
	if err := os.Rename(path, path+ReapedSuffix); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("precondition: live registry should be gone after tombstone, stat err=%v", err)
	}
	recs, err := ReadAllResolving(path)
	if err != nil {
		t.Fatalf("ReadAllResolving: %v", err)
	}
	if len(recs) != 1 || recs[0].Session != "evolve-bridge-x" {
		t.Errorf("post-tombstone resolve: got %+v, want the single tombstoned record", recs)
	}
}

// AC1.2 (negative / no-fabrication) — a run with NEITHER a live registry NOR a
// tombstone must resolve to zero records and no error. A resolver that ever
// fabricates attribution is worse than one that loses it.
func TestReadAllResolving_MissingBothIsZeroNoFabrication(t *testing.T) {
	dir := t.TempDir()
	recs, err := ReadAllResolving(PathIn(dir)) // neither <path> nor <path>.reaped exists
	if err != nil {
		t.Fatalf("ReadAllResolving on empty dir: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("no registry and no tombstone must resolve to zero records, got %d (%+v) — no fabrication", len(recs), recs)
	}
}

// AC1.3 (edge) — the same session present in BOTH the live registry and the
// tombstone (a run relaunched after a reap re-creates the registry) must NOT be
// double-counted: the union deduplicates by session id.
func TestReadAllResolving_LiveAndTombstoneNoDoubleCount(t *testing.T) {
	dir := t.TempDir()
	path := PathIn(dir)
	dup := Record{Session: "evolve-bridge-dup", RunID: "run1", Cycle: 1}
	if err := Append(path, dup); err != nil {
		t.Fatal(err)
	}
	if err := Append(path+ReapedSuffix, dup); err != nil {
		t.Fatal(err)
	}
	recs, err := ReadAllResolving(path)
	if err != nil {
		t.Fatalf("ReadAllResolving: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("live+tombstone with the same session must not double-count: got %d records %+v", len(recs), recs)
	}
}
