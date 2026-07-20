package mintregistry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPathAnchorsToProjectEvolveDir(t *testing.T) {
	got := Path("/proj")
	want := filepath.Join("/proj", ".evolve", "active-mints.json")
	if got != want {
		t.Errorf("Path=/%q, want %q", got, want)
	}
}

func TestAppendThenActiveNames(t *testing.T) {
	path := Path(t.TempDir())
	now := time.Now()
	for _, n := range []string{"alpha", "beta"} {
		if err := Append(path, n, now); err != nil {
			t.Fatalf("Append(%q): %v", n, err)
		}
	}
	names, err := ActiveNames(path, now)
	if err != nil {
		t.Fatalf("ActiveNames: %v", err)
	}
	if !names["alpha"] || !names["beta"] || len(names) != 2 {
		t.Errorf("ActiveNames=%v, want {alpha,beta}", names)
	}
}

func TestActiveNamesExcludesExpired(t *testing.T) {
	path := Path(t.TempDir())
	now := time.Now()
	if err := Append(path, "stale", now.Add(-TTL-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, "fresh", now); err != nil {
		t.Fatal(err)
	}
	names, err := ActiveNames(path, now)
	if err != nil {
		t.Fatalf("ActiveNames: %v", err)
	}
	if names["stale"] || !names["fresh"] {
		t.Errorf("ActiveNames=%v, want fresh only", names)
	}
}

func TestActiveNamesMissingFileIsEmptyNoError(t *testing.T) {
	names, err := ActiveNames(Path(t.TempDir()), time.Now())
	if err != nil {
		t.Fatalf("missing registry must not error; got: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("missing registry must be empty; got %v", names)
	}
}

func TestActiveNamesCorruptFailsSafeEmpty(t *testing.T) {
	path := Path(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := ActiveNames(path, time.Now())
	if err == nil {
		t.Error("corrupt registry must surface an error")
	}
	if len(names) != 0 {
		t.Errorf("corrupt registry must yield an EMPTY set (guard stays armed); got %v", names)
	}
}

// TestAppendSameNameReplacesEntry: a re-mint refreshes the one entry instead
// of growing the file unboundedly.
func TestAppendSameNameReplacesEntry(t *testing.T) {
	path := Path(t.TempDir())
	now := time.Now()
	if err := Append(path, "again", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, "again", now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("registry not a JSON entry array: %v\n%s", err, raw)
	}
	if len(entries) != 1 {
		t.Errorf("re-mint must replace, not append; got %d entries", len(entries))
	}
	if !entries[0].MintedAt.After(now.Add(-time.Minute)) {
		t.Errorf("re-mint must refresh MintedAt; got %v", entries[0].MintedAt)
	}
}

// TestAppendPrunesExpiredEntries: appends garbage-collect entries past TTL so
// the registry cannot grow without bound across batches.
func TestAppendPrunesExpiredEntries(t *testing.T) {
	path := Path(t.TempDir())
	now := time.Now()
	if err := Append(path, "ancient", now.Add(-TTL-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, "current", now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("registry not a JSON entry array: %v\n%s", err, raw)
	}
	if len(entries) != 1 || entries[0].Name != "current" {
		t.Errorf("append must prune expired entries; got %+v", entries)
	}
}

func TestQuarantineCorrupt_RenamesAndSelfHeals(t *testing.T) {
	path := Path(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	quarantined, err := QuarantineCorrupt(path)
	if err != nil {
		t.Fatalf("QuarantineCorrupt: %v", err)
	}
	if !quarantined {
		t.Fatal("expected corrupt registry to be quarantined")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("registry must be renamed away; stat err: %v", statErr)
	}
	if m, _ := filepath.Glob(path + ".corrupt-*"); len(m) != 1 {
		t.Errorf("expected one quarantine file; got %v", m)
	}
	names, err := ActiveNames(path, time.Now())
	if err != nil || len(names) != 0 {
		t.Errorf("post-quarantine read must be empty/no-error; got %v, %v", names, err)
	}
}

func TestQuarantineCorrupt_HealthyAndMissingUntouched(t *testing.T) {
	path := Path(t.TempDir())
	// Missing file: nothing to do.
	if q, err := QuarantineCorrupt(path); err != nil || q {
		t.Errorf("missing registry must be a no-op; got %v, %v", q, err)
	}
	// Healthy file: must not be quarantined (a concurrent Append may have
	// just repaired it between the caller's failed read and this call).
	if err := Append(path, "fine-mint", time.Now()); err != nil {
		t.Fatal(err)
	}
	if q, err := QuarantineCorrupt(path); err != nil || q {
		t.Errorf("healthy registry must be untouched; got %v, %v", q, err)
	}
	names, err := ActiveNames(path, time.Now())
	if err != nil || !names["fine-mint"] {
		t.Errorf("healthy registry lost after QuarantineCorrupt; got %v, %v", names, err)
	}
}

// TestAppendConcurrentNamesAllSurvive: concurrent lanes' appends serialize
// under the flock — no lost updates.
func TestAppendConcurrentNamesAllSurvive(t *testing.T) {
	path := Path(t.TempDir())
	now := time.Now()
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- Append(path, fmt.Sprintf("mint-%02d", i), now)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Append: %v", err)
		}
	}
	names, err := ActiveNames(path, now)
	if err != nil {
		t.Fatalf("ActiveNames: %v", err)
	}
	if len(names) != n {
		t.Errorf("lost updates: got %d names, want %d: %v", len(names), n, names)
	}
}
