package cyclestate

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestState_RoundTrip pins the on-disk shape of state.json: a fully-populated
// State must marshal and unmarshal back to an equal value, and the wire field
// names (the byte-identity boundary that the ledger SHA-chain and resume path
// depend on) must be exactly as declared.
func TestState_RoundTrip(t *testing.T) {
	in := State{
		LastUpdated:              "2026-06-21T00:00:00Z",
		LastCycleNumber:          7,
		Version:                  3,
		CurrentBatch:             BatchAccrual{CycleAccruedCostUSD: 1.5, GoalHash: "abc"},
		FailedAt:                 []FailedRecord{{Cycle: 6, Verdict: "FAIL", Defects: []string{"x"}}},
		CarryoverTodos:           []CarryoverTodo{{ID: "t1", Action: "do", Priority: "high", FirstSeenCycle: 5}},
		TriageThroughput:         []TriageThroughputEntry{{Cycle: 6, Floors: 2}},
		StateRevision:            4,
		LastAllocatedCycleNumber: 8,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out State
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
	// Byte-identity guard: the State wire names AND the sub-record wire names
	// (BatchAccrual/FailedRecord/CarryoverTodo/TriageThroughputEntry) must be
	// exactly these — a tag rename that still produced an equal Go value would
	// slip past the round-trip DeepEqual above.
	for _, want := range []string{
		`"lastCycleNumber"`, `"failedApproaches"`, `"carryoverTodos"`, `"currentBatch"`,
		`"cycleAccruedCostUSD"`, `"retrospected"`, `"first_seen_cycle"`, `"floors"`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("State JSON missing wire field %s; got %s", want, b)
		}
	}
}

// TestCycleState_RoundTrip pins the snake_case wire shape of cycle-state.json.
func TestCycleState_RoundTrip(t *testing.T) {
	in := CycleState{
		CycleID:         9,
		Phase:           "build",
		StartedAt:       "t0",
		PhaseStartedAt:  "t1",
		ActiveAgent:     "builder",
		ActiveWorktree:  "/wt",
		CompletedPhases: []string{"scout", "tdd"},
		WorkspacePath:   "/ws",
		IntentRequired:  true,
		RunID:           "01H",
		WorktreeBaseSHA: "deadbeef",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out CycleState
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
	for _, want := range []string{`"cycle_id"`, `"worktree_base_sha"`, `"completed_phases"`, `"intent_required"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("CycleState JSON missing wire field %s; got %s", want, b)
		}
	}
}

// TestStateOmitempty guards that the optional fields drop out of a zero State —
// pre-feature state.json files must stay byte-clean (no spurious keys).
func TestStateOmitempty(t *testing.T) {
	b, err := json.Marshal(State{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, absent := range []string{"failedApproaches", "carryoverTodos", "triageThroughput", "setupCompletedAt", "setupVersion", "stateRevision", "lastAllocatedCycleNumber"} {
		if strings.Contains(string(b), absent) {
			t.Errorf("zero State should omit %q; got %s", absent, b)
		}
	}
}
