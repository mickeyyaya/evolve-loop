package statemap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

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

func TestUpdateStateMap_PreservesUnmodelledFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeRawState(t, path, map[string]any{
		"lastCycleNumber":   41,
		"version":           18,
		"expected_ship_sha": "deadbeef-must-survive",
		"currentBatch":      map[string]any{"cycleAccruedCostUSD": 239.2},
	})

	if err := UpdateStateMap(path, func(m map[string]any) {
		m["lastCycleNumber"] = 42
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

func TestReadStateMap_MissingFileReturnsEmptyMap(t *testing.T) {
	m, err := ReadStateMap(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("missing file must return an empty (non-nil) map, got %#v", m)
	}
}

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
