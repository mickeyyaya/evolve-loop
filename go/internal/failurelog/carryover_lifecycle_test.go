package failurelog

// carryover_lifecycle_test.go — behavior + apicover naming tests for the
// carryoverTodos lifecycle trio (cycle-536 ship): backfill legacy expiry →
// prune → staleness counter. CI's apicover -enforce flagged all three
// UNCOVERED (no test named them); these tests close the gap by pinning each
// documented contract, not by merely naming the identifiers.

import (
	"encoding/json"
	"testing"
	"time"
)

// brParseTodos decodes state.json at path and returns its carryoverTodos.
func brParseTodos(t *testing.T, path string) []map[string]any {
	t.Helper()
	var state struct {
		CarryoverTodos []map[string]any `json:"carryoverTodos"`
	}
	if err := json.Unmarshal([]byte(brReadState(t, path)), &state); err != nil {
		t.Fatalf("parse state.json: %v", err)
	}
	return state.CarryoverTodos
}

// The 30-day legacy stamp is itself a pinned contract: silently changing it
// re-ages the entire pre-TTL legacy population in one release.
func TestDefaultCarryoverBackfillTTL_Is30Days(t *testing.T) {
	if want := 30 * 24 * time.Hour; DefaultCarryoverBackfillTTL != want {
		t.Errorf("DefaultCarryoverBackfillTTL = %v, want %v", DefaultCarryoverBackfillTTL, want)
	}
}

// Backfill stamps now+TTL ONLY on entries lacking expiresAt; a pre-stamped
// entry is never re-stamped, and a second pass is a byte-identical no-op.
func TestBackfillLegacyCarryoverExpiry_StampsOnlyLegacyAndIsIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	pre := now.Add(48 * time.Hour).Format(time.RFC3339)
	statePath := brWriteState(t, t.TempDir(), `{"carryoverTodos":[`+
		`{"id":"legacy-no-expiry"},`+
		`{"id":"already-stamped","expiresAt":"`+pre+`"}]}`)

	stamped, err := BackfillLegacyCarryoverExpiry(statePath, DefaultCarryoverBackfillTTL, now)
	if err != nil {
		t.Fatalf("BackfillLegacyCarryoverExpiry: %v", err)
	}
	if stamped != 1 {
		t.Fatalf("stamped = %d, want 1 (only the legacy entry)", stamped)
	}
	wantExpiry := now.Add(DefaultCarryoverBackfillTTL).Format(time.RFC3339)
	for _, todo := range brParseTodos(t, statePath) {
		switch todo["id"] {
		case "legacy-no-expiry":
			if todo["expiresAt"] != wantExpiry {
				t.Errorf("legacy entry expiresAt = %v, want now+TTL %s", todo["expiresAt"], wantExpiry)
			}
		case "already-stamped":
			if todo["expiresAt"] != pre {
				t.Errorf("pre-stamped expiry re-stamped to %v — converging TTL pushed forward", todo["expiresAt"])
			}
		}
	}

	before := brReadState(t, statePath)
	again, err := BackfillLegacyCarryoverExpiry(statePath, DefaultCarryoverBackfillTTL, now)
	if err != nil {
		t.Fatalf("second BackfillLegacyCarryoverExpiry: %v", err)
	}
	if again != 0 {
		t.Errorf("second pass stamped %d, want 0 (idempotent)", again)
	}
	if brReadState(t, statePath) != before {
		t.Error("second pass changed state.json bytes — backfill must skip the write when nothing is stamped")
	}
}

// IncrementCarryoverUnpicked bumps cycles_unpicked by exactly one on every
// entry that survived to this boot: an existing counter increments, an absent
// counter starts its life at 1 (it survived one full cycle unpicked).
func TestIncrementCarryoverUnpicked_BumpsEverySurvivor(t *testing.T) {
	statePath := brWriteState(t, t.TempDir(), `{"carryoverTodos":[`+
		`{"id":"seen-before","cycles_unpicked":2},`+
		`{"id":"first-boot"}]}`)

	n, err := IncrementCarryoverUnpicked(statePath)
	if err != nil {
		t.Fatalf("IncrementCarryoverUnpicked: %v", err)
	}
	if n != 2 {
		t.Fatalf("incremented = %d, want 2 (every survivor)", n)
	}
	for _, todo := range brParseTodos(t, statePath) {
		want := map[string]float64{"seen-before": 3, "first-boot": 1}[todo["id"].(string)]
		if got, _ := todo["cycles_unpicked"].(float64); got != want {
			t.Errorf("todo %v cycles_unpicked = %v, want %v", todo["id"], got, want)
		}
	}
}
