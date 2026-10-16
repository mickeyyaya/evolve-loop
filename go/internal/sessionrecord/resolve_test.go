package sessionrecord

import (
	"os"
	"testing"
)

func TestReadAllResolving_ReadsTombstoneAfterReap(t *testing.T) {
	dir := t.TempDir()
	path := PathIn(dir)
	rec := Record{Session: "evolve-bridge-x", RunID: "run1", Cycle: 1}
	if err := Append(path, rec); err != nil {
		t.Fatal(err)
	}
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

func TestReadAllResolving_MissingBothIsZeroNoFabrication(t *testing.T) {
	dir := t.TempDir()
	recs, err := ReadAllResolving(PathIn(dir))
	if err != nil {
		t.Fatalf("ReadAllResolving on empty dir: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("no registry and no tombstone must resolve to zero records, got %d (%+v) — no fabrication", len(recs), recs)
	}
}

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
