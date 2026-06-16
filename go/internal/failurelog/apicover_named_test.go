//go:build integration

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises exported symbols apicover flagged uncovered in this package:
//   - const LegacyEffectiveTTL (prune.go) — asserted via PruneExpired's legacy
//     recordedAt fallback (an entry recordedAt+LegacyEffectiveTTL in the past
//     is pruned; one inside the window is kept).
//   - const MaxEntries (record.go) — asserted via Record's FIFO trim cap.
//   - type PruneResult (prune.go) — asserted by full-struct equality on the
//     value PruneExpired returns.
//   - type Recorded (record.go) — asserted by full-struct equality on the
//     value Record returns.
//
// Each test asserts a real contract (Rule 9), not a no-op reference.
package failurelog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeStateFile seeds state.json with the given top-level map and returns its
// path. Local to this tagged file so it does not collide with the untagged
// helpers (writeState/seedStateWithEntries) which live in non-integration tests.
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

// TestLegacyEffectiveTTL_PrunesAtBoundary pins LegacyEffectiveTTL's role: a
// legacy entry (recordedAt, no expiresAt) is expired exactly when
// recordedAt + LegacyEffectiveTTL is in the past. We seed two entries that
// straddle that boundary relative to `now` and assert only the older one is
// removed — proving the constant is the TTL the pruner actually applies.
func TestLegacyEffectiveTTL_PrunesAtBoundary(t *testing.T) {
	if LegacyEffectiveTTL != 24*time.Hour {
		t.Fatalf("LegacyEffectiveTTL = %v, want 24h", LegacyEffectiveTTL)
	}
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	// Entry A: recordedAt is LegacyEffectiveTTL+1m before now → expired.
	recA := now.Add(-LegacyEffectiveTTL - time.Minute).Format(time.RFC3339)
	// Entry B: recordedAt is LegacyEffectiveTTL-1m before now → still inside TTL.
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

// TestMaxEntries_CapsFIFO pins MaxEntries as the FIFO cap Record enforces:
// seeding MaxEntries existing entries then appending one more must leave
// exactly MaxEntries (the oldest dropped).
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

// TestPruneResult_FullStructEquality asserts the PruneResult value PruneExpired
// returns equals the expected summary field-for-field (Before/After/Removed).
func TestPruneResult_FullStructEquality(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	path := writeStateFile(t, map[string]any{
		"failedApproaches": []map[string]any{
			{"cycle": float64(1), "expiresAt": "2026-05-22T11:59:59Z"}, // expired
			{"cycle": float64(2), "expiresAt": "2026-06-01T00:00:00Z"}, // kept
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

// TestRecorded_FullStructEquality asserts the Recorded value Record returns
// equals the expected entry field-for-field — the deterministic Now lets us pin
// RecordedAt and ExpiresAt (1 day later for infrastructure-transient).
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
