package core

import "testing"

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
