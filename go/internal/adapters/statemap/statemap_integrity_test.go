package statemap

// statemap_integrity_test.go — the cycle-999/1000/1001 shared-state disease
// pins (inbox: statejson-stalerevision-cas-lost-write 0.96 +
// statejson-worktree-canonical-propagation 0.95).
//
// Two defects, one chokepoint:
//  1. PROPAGATION: WriteStateMap's tmp+rename REPLACED a symlink at path with
//     a regular file — the first statemap write inside a worktree severed the
//     .evolve/state.json link to canonical and stranded every later mutation
//     in a detached copy (cycle-999: "135->14 stranded"; ledger.jsonl's link
//     survived only because nothing rename-wrote it).
//  2. LOST WRITE: a writer holding a stale in-memory map won the flock and
//     legally overwrote newer state (cycle-1001: a Jul-14 snapshot, rev 1356,
//     clobbered the live file destroying 136 carryoverTodos). The lock
//     serialized but never VALIDATED freshness.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func writeJSON(t *testing.T, path string, m map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(m)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWriteStateMap_PreservesSymlinkAndWritesThrough pins propagation: writing
// via a symlinked path must land the bytes in the CANONICAL target and leave
// the symlink intact — never replace the link with a detached regular file.
func TestWriteStateMap_PreservesSymlinkAndWritesThrough(t *testing.T) {
	canonicalDir := t.TempDir()
	worktreeDir := t.TempDir()
	canonical := filepath.Join(canonicalDir, "state.json")
	writeJSON(t, canonical, map[string]any{"stateRevision": float64(1)})
	link := filepath.Join(worktreeDir, "state.json")
	if err := os.Symlink(canonical, link); err != nil {
		t.Fatal(err)
	}

	if err := WriteStateMap(link, map[string]any{"stateRevision": float64(1), "k": "v"}); err != nil {
		t.Fatalf("WriteStateMap through symlink: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was severed: worktree path is now a regular file (the cycle-999 defect)")
	}
	got, err := ReadStateMap(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if got["k"] != "v" {
		t.Fatalf("write did not reach canonical target; canonical=%v", got)
	}
}

// TestWriteStateMap_DanglingSymlinkCreatesTarget: a dangling link (target not
// yet written — the provisioning window) must create the TARGET, not replace
// the link.
func TestWriteStateMap_DanglingSymlinkCreatesTarget(t *testing.T) {
	canonicalDir := t.TempDir()
	worktreeDir := t.TempDir()
	canonical := filepath.Join(canonicalDir, "state.json")
	link := filepath.Join(worktreeDir, "state.json")
	if err := os.Symlink(canonical, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteStateMap(link, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("WriteStateMap through dangling symlink: %v", err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dangling symlink was replaced instead of resolved")
	}
	if got, err := ReadStateMap(canonical); err != nil || got["k"] != "v" {
		t.Fatalf("target not created through dangling link: %v %v", got, err)
	}
}

// TestWriteStateMap_RefusesStaleRevision pins the CAS floor: an incoming map
// whose stateRevision is BELOW the on-disk revision is a stale writer and must
// be refused with ErrStaleRevision — never applied, regardless of locks.
func TestWriteStateMap_RefusesStaleRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeJSON(t, path, map[string]any{"stateRevision": float64(1360), "carryoverTodos": []any{"a", "b"}})

	stale := map[string]any{"stateRevision": float64(1356)} // the Jul-14 snapshot shape
	err := WriteStateMap(path, stale)
	if err == nil {
		t.Fatal("stale write (rev 1356 over 1360) must be refused — this is the cycle-1001 lost-write")
	}
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("want ErrStaleRevision, got: %v", err)
	}
	got, _ := ReadStateMap(path)
	if got["stateRevision"] != float64(1360) {
		t.Fatalf("on-disk state clobbered despite refusal: %v", got)
	}
}

// TestUpdateStateMap_AutoBumpsOwnCounterAndTimestamp: the locked RMW must
// advance statemap's OWN lineage counter and refresh lastUpdated on every
// write (the frozen Jul-14 lastUpdated proved some writers skip both) — while
// leaving stateRevision UNTOUCHED: that counter is storage.UpdateState's
// exclusive CA.3 OCC audit trail ("a gap/repeat betrays a bypassing writer"),
// and statemap moving it would forge exactly that forensic signal.
func TestUpdateStateMap_AutoBumpsOwnCounterAndTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeJSON(t, path, map[string]any{"stateRevision": float64(7), "lastUpdated": "2026-07-14T05:01:14Z"})
	if err := UpdateStateMap(path, func(m map[string]any) { m["k"] = "v" }); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadStateMap(path)
	if got[statemapRevisionKey] != float64(1) {
		t.Fatalf("statemapRevision not seeded/bumped: %v", got[statemapRevisionKey])
	}
	if got["stateRevision"] != float64(7) {
		t.Fatalf("stateRevision must stay UNTOUCHED (storage.UpdateState exclusive); got %v", got["stateRevision"])
	}
	if got["lastUpdated"] == "2026-07-14T05:01:14Z" || got["lastUpdated"] == "" || got["lastUpdated"] == nil {
		t.Fatalf("lastUpdated not refreshed: %v", got["lastUpdated"])
	}
}

// TestWriteStateMap_RefusesStaleStatemapRevision: the namespaced counter gets
// the same CAS floor as stateRevision.
func TestWriteStateMap_RefusesStaleStatemapRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeJSON(t, path, map[string]any{statemapRevisionKey: float64(9)})
	err := WriteStateMap(path, map[string]any{statemapRevisionKey: float64(4)})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale statemapRevision must be refused; got: %v", err)
	}
}

// TestUpdateStateMap_ConcurrentWritersAllLand: N concurrent locked RMWs must
// each land exactly once (serialized re-read inside the lock) and the revision
// must advance by N.
func TestUpdateStateMap_ConcurrentWritersAllLand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeJSON(t, path, map[string]any{"stateRevision": float64(0)})
	const n = 12
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = UpdateStateMap(path, func(m map[string]any) {
				m[fmt.Sprintf("w%02d", i)] = true
			})
		}(i)
	}
	wg.Wait()
	got, _ := ReadStateMap(path)
	for i := 0; i < n; i++ {
		if got[fmt.Sprintf("w%02d", i)] != true {
			t.Fatalf("writer %d lost", i)
		}
	}
	if got[statemapRevisionKey] != float64(n) {
		t.Fatalf("statemapRevision=%v, want %d", got[statemapRevisionKey], n)
	}
	if got["stateRevision"] != float64(0) {
		t.Fatalf("stateRevision must stay untouched; got %v", got["stateRevision"])
	}
}

// TestUpdateStateMap_CrossTreeWritersSerializeOnCanonical: a worktree writer
// (via symlink) and a canonical writer must share ONE lock — the lock must be
// taken on the RESOLVED path's sidecar, else cross-tree writers race.
func TestUpdateStateMap_CrossTreeWritersSerializeOnCanonical(t *testing.T) {
	canonicalDir := t.TempDir()
	worktreeDir := t.TempDir()
	canonical := filepath.Join(canonicalDir, "state.json")
	writeJSON(t, canonical, map[string]any{"stateRevision": float64(0)})
	link := filepath.Join(worktreeDir, "state.json")
	if err := os.Symlink(canonical, link); err != nil {
		t.Fatal(err)
	}
	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := canonical
			if i%2 == 1 {
				p = link
			}
			_ = UpdateStateMap(p, func(m map[string]any) { m[fmt.Sprintf("x%02d", i)] = true })
		}(i)
	}
	wg.Wait()
	got, _ := ReadStateMap(canonical)
	for i := 0; i < n; i++ {
		if got[fmt.Sprintf("x%02d", i)] != true {
			t.Fatalf("cross-tree writer %d lost (locks not unified on resolved path)", i)
		}
	}
	if got[statemapRevisionKey] != float64(n) {
		t.Fatalf("statemapRevision=%v, want %d", got[statemapRevisionKey], n)
	}
}
