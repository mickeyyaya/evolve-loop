package core

import (
	"strings"
	"testing"
)

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
