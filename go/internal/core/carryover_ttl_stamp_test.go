package core

// carryover_ttl_stamp_test.go — RED test (cycle 507, task
// prune-stale-carryover-todos) for the CREATION-TIME half of the TTL contract.
//
// state.json:carryoverTodos grows unboundedly (65 entries / 26,601 bytes today,
// cycles 366→506) because CarryoverTodo — unlike its structurally-parallel
// sibling failedApproaches — carries NO expiry and has NO prune path. The fix
// mirrors the sibling: stamp a TTL when the todo is created, then prune expired
// entries at loop start (the prune half lives in
// internal/failurelog/prune_carryover_test.go).
//
// ApplyDefectsAsCarryoverTodos already receives a FailedRecord whose ExpiresAt
// is the 7-day stamp the failedApproaches path computes; a created carryover
// todo must INHERIT that stamp so the prune pass can age it out. Inheriting the
// existing stamp (rather than recomputing one) keeps the two arrays' TTL logic
// single-sourced (never_duplicate_centralize_via_design_patterns).
//
// References CarryoverTodo.ExpiresAt, which the Builder adds to
// cyclestate.CarryoverTodo. RED now (no such field → core test package fails to
// compile). Do NOT modify this file — implement the production seam.

import "testing"

// AC (positive): a defect-derived carryover todo inherits the failed record's
// TTL stamp so it becomes prune-eligible after the retention window.
func TestApplyDefectsAsCarryoverTodos_StampsExpiryFromRecord(t *testing.T) {
	st := &State{}
	const exp = "2026-07-12T00:00:00Z" // the 7-day stamp the failedApproaches path already computes
	ApplyDefectsAsCarryoverTodos(st, FailedRecord{
		Cycle:     507,
		ExpiresAt: exp,
		Defects:   []string{"boot recovery never wired"},
	})
	if len(st.CarryoverTodos) != 1 {
		t.Fatalf("one defect must create one carryover todo; got %d", len(st.CarryoverTodos))
	}
	if got := st.CarryoverTodos[0].ExpiresAt; got != exp {
		t.Errorf("created carryover todo must inherit the record's TTL stamp so PruneExpiredCarryoverTodos can age it out; got ExpiresAt=%q want %q", got, exp)
	}
}

// AC (negative / no-false-stamp): when the failed record carries NO expiry
// (a true legacy record), the created todo must ALSO carry none — the prune
// pass keeps untimestamped entries, so a fabricated stamp would wrongly age out
// data whose age we cannot actually know.
func TestApplyDefectsAsCarryoverTodos_NoRecordExpiryLeavesTodoUnstamped(t *testing.T) {
	st := &State{}
	ApplyDefectsAsCarryoverTodos(st, FailedRecord{
		Cycle:   366,
		Defects: []string{"legacy defect"},
	})
	if len(st.CarryoverTodos) != 1 {
		t.Fatalf("one defect must create one carryover todo; got %d", len(st.CarryoverTodos))
	}
	if got := st.CarryoverTodos[0].ExpiresAt; got != "" {
		t.Errorf("a record with no ExpiresAt must not fabricate a TTL stamp on its todo; got %q", got)
	}
}
