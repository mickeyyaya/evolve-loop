package failurelog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedStateWithEntries(t *testing.T, entries []map[string]any) string {
	t.Helper()
	state := map[string]any{
		"lastCycleNumber":  float64(10),
		"failedApproaches": entries,
	}
	raw, _ := json.Marshal(state)
	return mustWrite(t, filepath.Join(t.TempDir(), "state.json"), string(raw))
}

func TestPruneExpired_RemovesPastExpiresAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "expiresAt": "2026-05-22T11:59:59Z"},
		{"cycle": float64(2), "expiresAt": "2026-05-23T13:00:00Z"},
		{"cycle": float64(3), "expiresAt": "2026-05-23T11:59:59Z"},
	})
	res, err := PruneExpired(path, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Before != 3 || res.After != 1 || res.Removed != 2 {
		t.Fatalf("result=%+v want {3,1,2}", res)
	}
	state := readState(t, path)
	kept := state["failedApproaches"].([]any)
	if len(kept) != 1 {
		t.Fatalf("kept=%d want 1", len(kept))
	}
	if kept[0].(map[string]any)["cycle"].(float64) != 2 {
		t.Fatalf("kept[0] should be cycle 2; got %v", kept[0])
	}
}

func TestPruneExpired_LegacyRecordedAtFallback(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "recordedAt": "2026-05-21T12:00:00Z"},
		{"cycle": float64(2), "recordedAt": "2026-05-23T11:30:00Z"},
	})
	res, err := PruneExpired(path, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 1 {
		t.Fatalf("removed=%d want 1", res.Removed)
	}
}

func TestPruneExpired_KeepsEntriesWithoutTimestamps(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "classification": "infrastructure-transient"},
	})
	res, err := PruneExpired(path, time.Now())
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 0 {
		t.Fatalf("removed=%d want 0 (no timestamps → keep)", res.Removed)
	}
}

func TestPruneExpired_MalformedExpiresAtKept(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "expiresAt": "not-a-timestamp"},
	})
	res, err := PruneExpired(path, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 0 {
		t.Fatalf("removed=%d want 0 (malformed kept)", res.Removed)
	}
}

func TestPruneExpired_NonObjectEntryKept(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]any{
		"failedApproaches": []any{
			"just-a-string-somehow",
			map[string]any{"cycle": float64(2), "expiresAt": "2020-01-01T00:00:00Z"},
		},
	})
	path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), string(raw))
	res, err := PruneExpired(path, time.Now().UTC())
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 1 {
		t.Fatalf("removed=%d want 1 (only object-entry expired)", res.Removed)
	}
	state := readState(t, path)
	kept := state["failedApproaches"].([]any)
	if len(kept) != 1 {
		t.Fatalf("kept=%d want 1 (string survives)", len(kept))
	}
}

func TestPruneExpired_NoStateFile(t *testing.T) {
	t.Parallel()
	res, err := PruneExpired(filepath.Join(t.TempDir(), "nope.json"), time.Now())
	if err != nil {
		t.Fatalf("missing state should not error: %v", err)
	}
	if res.Before != 0 || res.After != 0 {
		t.Fatalf("res=%+v want zero", res)
	}
}

func TestPruneExpired_BadJSON(t *testing.T) {
	t.Parallel()
	path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), "{bad")
	if _, err := PruneExpired(path, time.Now()); err == nil {
		t.Fatalf("expected error on bad JSON")
	}
}

func TestPruneExpired_EmptyFailedApproaches(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{})
	res, err := PruneExpired(path, time.Now())
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Before != 0 {
		t.Fatalf("empty list should yield before=0; got %d", res.Before)
	}
}

func TestPruneExpired_NoRemovalSkipsWrite(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "expiresAt": "2026-06-01T00:00:00Z"},
	})
	before, _ := os.Stat(path)
	time.Sleep(15 * time.Millisecond)
	if _, err := PruneExpired(path, now); err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("no-prune-needed should skip write; mtime moved from %s to %s",
			before.ModTime(), after.ModTime())
	}
}

func TestPruneExpired_MalformedRecordedAtKept(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "recordedAt": "garbage"},
	})
	res, err := PruneExpired(path, time.Now())
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 0 {
		t.Fatalf("malformed recordedAt should be kept; got removed=%d", res.Removed)
	}
}

func TestPruneExpired_AtomicWriteError(t *testing.T) {
	prev := atomicWriteJSON
	defer func() { atomicWriteJSON = prev }()
	atomicWriteJSON = func(string, map[string]any) error {
		return errors.New("synthetic write error")
	}
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "expiresAt": "2026-05-22T11:59:59Z"},
	})
	if _, err := PruneExpired(path, now); err == nil {
		t.Fatalf("expected write error")
	}
}
