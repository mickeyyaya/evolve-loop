package core

// carryover_fingerprint_test.go — RED contract for the carryoverTodos P0-flood
// dedupe (2026-08-10 investigation, agent C): 124 of 254 live entries were
// near-identical per-FAIL generic P0s ("cycle N failed during audit: …"),
// distinct only by the cycle number baked into their IDs — carryoverTodoExists
// dedupes by ID, so every FAIL minted a fresh duplicate, and the 20-slot
// router window (phase_advisor) was 100% saturated by them, permanently
// shadowing memo/product carryovers. Dedupe key: the Action text with cycle
// tokens normalized — same defect, different cycle ⇒ ONE entry.

import (
	"strings"
	"testing"
)

func TestCarryoverActionFingerprint_NormalizesCycleTokens(t *testing.T) {
	t.Parallel()
	a := carryoverActionFingerprint("cycle 1421 failed during audit: disposition-preflight: MISSING")
	b := carryoverActionFingerprint("cycle 1428 failed during audit: disposition-preflight: MISSING")
	if a != b {
		t.Errorf("same failure class across cycles must fingerprint identically:\n a=%q\n b=%q", a, b)
	}
	c := carryoverActionFingerprint("Fix defect from cycle 1424: nested decoy defeats ambiguity guard")
	d := carryoverActionFingerprint("Fix defect from cycle 1427: nested decoy defeats ambiguity guard")
	if c != d {
		t.Errorf("defect-adoption entries must fingerprint identically across cycles:\n c=%q\n d=%q", c, d)
	}
	if a == c {
		t.Error("distinct failure classes must not collide")
	}
}

func TestCarryoverFingerprintExists_DedupesAcrossCycles(t *testing.T) {
	t.Parallel()
	todos := []CarryoverTodo{{ID: "cycle-1421-defect-0", Action: "Fix defect from cycle 1421: salvage parser drops fenced JSON"}}
	if !carryoverFingerprintExists(todos, "Fix defect from cycle 1428: salvage parser drops fenced JSON") {
		t.Error("cross-cycle duplicate not detected — the P0 flood regrows")
	}
	if carryoverFingerprintExists(todos, "Fix defect from cycle 1428: an entirely different defect") {
		t.Error("distinct defect wrongly deduped")
	}
}

// The mint-site wiring: adopting the same defect text from two different
// cycles' records must yield ONE carryover entry.
func TestAdoptDefects_CrossCycleFingerprintDedupe(t *testing.T) {
	t.Parallel()
	var state State
	ApplyDefectsAsCarryoverTodos(&state, FailedRecord{Cycle: 1424, Defects: []string{"nested decoy defeats ambiguity guard"}})
	ApplyDefectsAsCarryoverTodos(&state, FailedRecord{Cycle: 1427, Defects: []string{"nested decoy defeats ambiguity guard"}})
	n := 0
	for _, td := range state.CarryoverTodos {
		if strings.Contains(td.Action, "nested decoy") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("cross-cycle duplicate defect minted %d entries, want 1 (the 124-entry flood class)", n)
	}
}

// A suppressed re-mint refreshes the survivor's TTL — a class failing every
// cycle must not ride its FIRST occurrence's ExpiresAt into the boot prune
// (diff-review MEDIUM: TTL-blind dedupe erased still-live signal).
func TestAdoptDefects_SuppressedRemintRefreshesExpiry(t *testing.T) {
	t.Parallel()
	var state State
	ApplyDefectsAsCarryoverTodos(&state, FailedRecord{Cycle: 1424, ExpiresAt: "2026-08-12T00:00:00Z", Defects: []string{"gate X red"}})
	ApplyDefectsAsCarryoverTodos(&state, FailedRecord{Cycle: 1427, ExpiresAt: "2026-08-20T00:00:00Z", Defects: []string{"gate X red"}})
	if len(state.CarryoverTodos) != 1 {
		t.Fatalf("entries = %d, want 1", len(state.CarryoverTodos))
	}
	if got := state.CarryoverTodos[0].ExpiresAt; got != "2026-08-20T00:00:00Z" {
		t.Errorf("ExpiresAt = %q, want the LATER stamp — suppression must keep the class alive", got)
	}
}
