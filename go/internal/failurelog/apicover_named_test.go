//go:build integration

package failurelog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeStateFile(t *testing.T, state map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return path
}

func TestLegacyEffectiveTTL_PrunesAtBoundary(t *testing.T) {
	if LegacyEffectiveTTL != 24*time.Hour {
		t.Fatalf("LegacyEffectiveTTL = %v, want 24h", LegacyEffectiveTTL)
	}
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	recA := now.Add(-LegacyEffectiveTTL - time.Minute).Format(time.RFC3339)
	recB := now.Add(-LegacyEffectiveTTL + time.Minute).Format(time.RFC3339)
	path := writeStateFile(t, map[string]any{
		"failedApproaches": []map[string]any{
			{"cycle": float64(1), "recordedAt": recA},
			{"cycle": float64(2), "recordedAt": recB},
		},
	})
	res, err := PruneExpired(path, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if res.Removed != 1 || res.After != 1 {
		t.Fatalf("result=%+v want exactly the past-TTL entry removed (Removed=1, After=1)", res)
	}
}

func TestMaxEntries_CapsFIFO(t *testing.T) {
	if MaxEntries != 50 {
		t.Fatalf("MaxEntries = %d, want 50", MaxEntries)
	}
	entries := make([]map[string]any, 0, MaxEntries)
	for i := 0; i < MaxEntries; i++ {
		entries = append(entries, map[string]any{"cycle": float64(i), "classification": "audit-fail"})
	}
	path := writeStateFile(t, map[string]any{
		"lastCycleNumber":  float64(MaxEntries),
		"failedApproaches": entries,
	})
	if _, err := Record(path, "", RecordRequest{
		Cycle:          MaxEntries + 1,
		Classification: "audit-fail",
		Now:            time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("parse back: %v", err)
	}
	got := len(st["failedApproaches"].([]any))
	if got != MaxEntries {
		t.Fatalf("entries after append = %d, want MaxEntries (%d) — FIFO cap not applied", got, MaxEntries)
	}
}

func TestPruneResult_FullStructEquality(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	path := writeStateFile(t, map[string]any{
		"failedApproaches": []map[string]any{
			{"cycle": float64(1), "expiresAt": "2026-05-22T11:59:59Z"},
			{"cycle": float64(2), "expiresAt": "2026-06-01T00:00:00Z"},
		},
	})
	got, err := PruneExpired(path, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	want := PruneResult{Before: 2, After: 1, Removed: 1}
	if got != want {
		t.Fatalf("PruneResult = %+v, want %+v", got, want)
	}
}

func TestRecorded_FullStructEquality(t *testing.T) {
	path := writeStateFile(t, map[string]any{"lastCycleNumber": float64(4)})
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	got, err := Record(path, "", RecordRequest{
		Cycle:          5,
		Classification: "infrastructure",
		Summary:        "stop_reason=boot_timeout",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	want := Recorded{
		Cycle:          5,
		Classification: InfrastructureTransient,
		Summary:        "stop_reason=boot_timeout",
		RecordedAt:     "2026-05-23T12:00:00Z",
		ExpiresAt:      ComputeExpiresAt(InfrastructureTransient, now),
	}
	if got != want {
		t.Fatalf("Recorded = %+v, want %+v", got, want)
	}
}
