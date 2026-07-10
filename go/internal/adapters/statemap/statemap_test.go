package statemap

// statemap_test.go — RED contract for the cycle-659 task
// `statefile-rmw-flock-single-source`.
//
// The single source of truth for state.json read-modify-write is promoted here
// from ship/statefile.go's readStateMap/writeStateMap: a NEW LEAF PACKAGE
// importing ONLY internal/adapters/flock + stdlib (the verified-acyclic home
// from cycle-644's retrospective; storage was rejected because storage imports
// core, and stateutil was rejected as orphaned scaffolding). core/reset.go
// (SealCycle), phaseintegrity/repin.go (RepinShipSHA) and ship become callers;
// their three duplicate RMW implementations are deleted.
//
// This file is `package statemap` in a dir that (until Builder writes
// statemap.go) has NO non-test Go file, so `go test ./internal/adapters/statemap/`
// FAILS TO COMPILE — the RED state. Builder makes it GREEN by implementing:
//
//	func ReadStateMap(path string) (map[string]any, error)
//	func UpdateStateMap(path string, mutate func(map[string]any)) error
//
// UpdateStateMap MUST hold flock.PathLock(path) across the whole read→mutate→
// write and write atomically (tmp + rename), mirroring ship/statefile.go's
// documented contract and storage.UpdateState's lock discipline (CA.3).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// writeRawState is a local fixture helper — the test may NOT depend on the
// SUT's own writer to seed state (that would make the round-trip test circular).
func writeRawState(t *testing.T, path string, body any) {
	t.Helper()
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func readRawState(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

// TestUpdateStateMap_PreservesUnmodelledFields is the AC4 legacy-compat axis:
// full-fidelity map round-trip. A mutation touching ONE key must leave every
// other key — including keys no typed struct models (expected_ship_sha) and
// nested objects (currentBatch) — byte-for-byte intact. This is the exact
// property that makes a map-based RMW necessary instead of the typed storage
// adapter (which drops unmodelled keys).
func TestUpdateStateMap_PreservesUnmodelledFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeRawState(t, path, map[string]any{
		"lastCycleNumber":   41,
		"version":           18,
		"expected_ship_sha": "deadbeef-must-survive", // unmodelled by core.State
		"currentBatch":      map[string]any{"cycleAccruedCostUSD": 239.2},
	})

	if err := UpdateStateMap(path, func(m map[string]any) {
		m["lastCycleNumber"] = 42 // the ONLY field this writer touches
	}); err != nil {
		t.Fatalf("UpdateStateMap: %v", err)
	}

	final := readRawState(t, path)
	if got, _ := final["lastCycleNumber"].(float64); got != 42 {
		t.Errorf("mutation not applied: lastCycleNumber=%v want 42", final["lastCycleNumber"])
	}
	if got, _ := final["expected_ship_sha"].(string); got != "deadbeef-must-survive" {
		t.Errorf("UNMODELLED field lost: expected_ship_sha=%q want %q — map RMW must be full-fidelity",
			got, "deadbeef-must-survive")
	}
	cb, ok := final["currentBatch"].(map[string]any)
	if !ok {
		t.Fatalf("nested currentBatch object lost on round-trip: %#v", final["currentBatch"])
	}
	if got, _ := cb["cycleAccruedCostUSD"].(float64); got != 239.2 {
		t.Errorf("nested field lost: currentBatch.cycleAccruedCostUSD=%v want 239.2", cb["cycleAccruedCostUSD"])
	}
	if got, _ := final["version"].(float64); got != 18 {
		t.Errorf("untouched field mutated: version=%v want 18", final["version"])
	}
}

// TestUpdateStateMap_SerializesConcurrentWriters is the AC1 primitive-level
// serialization axis (run under -race). N goroutines each do a read→+1→write of
// the same counter via UpdateStateMap. If UpdateStateMap does NOT hold
// flock.PathLock across the whole RMW, two goroutines read the same value and
// one increment is lost, so the final counter is < N (lost update). A correct
// lock-owning implementation yields exactly N. This is the anti-no-op behavioral
// core of the whole task: an implementation that skips the lock FAILS here.
func TestUpdateStateMap_SerializesConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeRawState(t, path, map[string]any{"counter": 0})

	const writers = 20
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := UpdateStateMap(path, func(m map[string]any) {
				cur, _ := m["counter"].(float64)
				m["counter"] = cur + 1
			}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent UpdateStateMap: %v", err)
	}

	final := readRawState(t, path)
	if got, _ := final["counter"].(float64); int(got) != writers {
		t.Errorf("lost update: counter=%d want %d — UpdateStateMap must hold flock.PathLock(path) "+
			"across the whole read-modify-write so concurrent writers serialize", int(got), writers)
	}
}

// TestReadStateMap_MissingFileReturnsEmptyMap is the edge/OOD axis: a not-yet-
// created state.json must read as an empty map with no error (mirrors
// ship/statefile.go:readStateMap and reset.go:readJSONMapFile), so the first
// writer of a fresh project does not crash.
func TestReadStateMap_MissingFileReturnsEmptyMap(t *testing.T) {
	m, err := ReadStateMap(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("missing file must return an empty (non-nil) map, got %#v", m)
	}
}

// TestUpdateStateMap_MalformedJSONRefusesToClobber is the negative axis (the
// strongest anti-no-op signal): when state.json is present but malformed,
// UpdateStateMap MUST return an error and leave the file untouched, never
// silently overwrite an operator's hand-broken file with a fresh empty map.
// Mirrors storage.UpdateState's "malformed (...); refusing to clobber" contract.
func TestUpdateStateMap_MalformedJSONRefusesToClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	const garbage = "{ this is not valid json"
	if err := os.WriteFile(path, []byte(garbage), 0o644); err != nil {
		t.Fatalf("seed malformed: %v", err)
	}

	err := UpdateStateMap(path, func(m map[string]any) {
		m["lastCycleNumber"] = 99
	})
	if err == nil {
		t.Fatalf("malformed state.json must produce an error, got nil (would silently clobber)")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read after refusal: %v", readErr)
	}
	if string(after) != garbage {
		t.Errorf("malformed file was clobbered: content=%q want unchanged %q", string(after), garbage)
	}
}

// TestReadStateMap_EmptyFileReturnsEmptyMap is a second edge case: a
// zero-length state.json (a truncated-write artifact) reads as an empty map,
// not a parse error — matching the existing readStateMap/readJSONMapFile
// helpers this package consolidates.
func TestReadStateMap_EmptyFileReturnsEmptyMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seed empty: %v", err)
	}
	m, err := ReadStateMap(path)
	if err != nil {
		t.Fatalf("empty file must not error: %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("empty file must return an empty (non-nil) map, got %#v", m)
	}
}
