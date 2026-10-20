package testverdict

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const (
	floorGate = "build-floor"
	shipGate  = "ship"
)

func gated(t *testing.T, gate string) Record {
	t.Helper()
	r := greenRecord(t)
	r.Gate = gate
	return r
}

func plant(t *testing.T, path string, r Record) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, sealRecord(r), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStore_PutRefusesARecordWithNoGateID(t *testing.T) {
	for name, gate := range map[string]string{"empty": "", "a path": "../ship", "upper case": "Ship", "a dot": "ship.x"} {
		t.Run(name, func(t *testing.T) {
			s, root := newStore(t)
			if err := s.Put(gated(t, gate)); !errors.Is(err, ErrNoGate) {
				t.Fatalf("Put(gate %q) = %v, want ErrNoGate", gate, err)
			}
			if got := files(t, root); len(got) != 0 {
				t.Fatalf("a refused record leaves no file, got %v", got)
			}
		})
	}
}

func TestStore_ARedWithAGateIsStillRefusedAsNotGreen(t *testing.T) {
	s, _ := newStore(t)
	r := gated(t, floorGate)
	r.Outcome = Fail
	if err := s.Put(r); !errors.Is(err, ErrNotGreen) {
		t.Fatalf("Put(fail) = %v, want ErrNotGreen", err)
	}
}

func TestStore_EachGateHoldsItsOwnEntryAndALookupHitsAny(t *testing.T) {
	s, root := newStore(t)
	floor := gated(t, floorGate)
	mustPut(t, s, floor)

	hit, ok := s.Lookup(floor.Key.File, currentEnv())
	if !ok || hit.Gate != floorGate {
		t.Fatalf("the ship must hit the green that the floor stored on the same key; got %+v, %v", hit, ok)
	}
	mustPut(t, s, gated(t, shipGate))
	for _, gate := range []string{floorGate, shipGate} {
		if _, err := os.Stat(entryPath(root, floor.Key.File, floor.Key.Full, gate)); err != nil {
			t.Errorf("the entry of gate %s: %v", gate, err)
		}
	}
}

func TestStore_ARecordWithoutAGateOrUnderAnotherGatesNameIsAMiss(t *testing.T) {
	cases := map[string]struct {
		gate string
		at   func(root string, r Record) string
	}{
		"a record of the older layout with no gate": {"", func(root string, r Record) string {
			return filepath.Join(root, r.Key.File, r.Key.Full+entryFile)
		}},
		"a ship record under the name of the floor": {shipGate, func(root string, r Record) string {
			return entryPath(root, r.Key.File, r.Key.Full, floorGate)
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, root := newStore(t)
			r := gated(t, c.gate)
			r.StoredAt = epoch
			plant(t, c.at(root, r), r)
			if hit, ok := s.Lookup(r.Key.File, currentEnv()); ok {
				t.Fatalf("a record at a path that its gate does not give hit: %+v", hit)
			}
		})
	}
}

func TestStore_AStoredFailIsNeverReused(t *testing.T) {
	s, root := newStore(t)
	r := gated(t, floorGate)
	r.Outcome = Fail
	r.StoredAt = epoch
	plant(t, entryPath(root, r.Key.File, r.Key.Full, r.Gate), r)
	if hit, ok := s.Lookup(r.Key.File, currentEnv()); ok {
		t.Fatalf("a sealed fail at its own path must never be a hit: %+v", hit)
	}
}

func TestStore_StatsCountsTheEntriesOfEachGate(t *testing.T) {
	s, root := newStore(t)
	empty, err := s.Stats()
	if err != nil || empty.Files != 0 || len(empty.ByGate) != 0 {
		t.Fatalf("a missing store is empty; got %+v, %v", empty, err)
	}
	mustPut(t, s, gated(t, floorGate))
	mustPut(t, s, gated(t, shipGate))
	legacy := greenRecord(t)
	plant(t, filepath.Join(root, legacy.Key.File, digest("e")+entryFile), legacy)

	got, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if got.Files != 4 || got.Entries != 3 || got.ByGate[floorGate] != 1 || got.ByGate[shipGate] != 1 || got.ByGate[""] != 1 || got.Bytes <= 0 {
		t.Fatalf("two gated entries, one entry with no gate and one index file; got %+v", got)
	}
}

func TestStore_WipeEmptiesTheStoreInOneStep(t *testing.T) {
	s, root := newStore(t)
	r := gated(t, floorGate)
	mustPut(t, s, r)
	mustPut(t, s, gated(t, shipGate))

	wiped, err := s.Wipe()
	if err != nil || wiped.Entries != 2 {
		t.Fatalf("Wipe = %+v, %v; want the two entries counted", wiped, err)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the store root must be gone: %v", err)
	}
	if _, ok := s.Lookup(r.Key.File, currentEnv()); ok {
		t.Fatal("a wiped store has no hit")
	}
	again, err := s.Wipe()
	if err != nil || again.Files != 0 {
		t.Fatalf("a wipe of a missing store is a no-op; got %+v, %v", again, err)
	}
}

func TestStore_WipeReportsAStoreItCannotListOrRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads and removes every directory")
	}
	s, root := newStore(t)
	r := gated(t, floorGate)
	mustPut(t, s, r)
	keyDir := filepath.Join(root, r.Key.File)

	if err := os.Chmod(keyDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(keyDir, 0o755) })
	if _, err := s.Wipe(); err == nil {
		t.Fatal("an entry that cannot be removed fails the wipe loudly")
	}

	if err := os.Chmod(keyDir, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Wipe(); err == nil {
		t.Fatal("a store that cannot be listed fails the wipe before it removes anything")
	}
}

func TestDirFor_IsTheOneHomeOfTheStoreUnderTheProject(t *testing.T) {
	if got := DirFor("/p"); got != filepath.Join("/p", ".evolve", "landing", "testcache") {
		t.Fatalf("got %q", got)
	}
}

func TestStore_RootIsTheDirectoryItWasMadeWith(t *testing.T) {
	s, root := newStore(t)
	if s.Root() != root {
		t.Fatalf("got %q, want %q", s.Root(), root)
	}
}
