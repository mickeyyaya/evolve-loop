package core

import (
	"context"
	"regexp"
	"testing"
	"time"
)

// runid_test.go — CA.5 (concurrency-factory plan, Track C-A): the
// event-sourced run identity. RunCycle mints one ULID per run and threads
// it into the persisted CycleState and EVERY ledger entry the run emits, so
// concurrent runs' entries are attributable after interleaving.

var crockfordRE = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func TestMintRunID_ShapeAndTimeOrdering(t *testing.T) {
	t0 := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	a := MintRunID(t0)
	b := MintRunID(t0.Add(time.Second))
	for _, id := range []string{a, b} {
		if !crockfordRE.MatchString(id) {
			t.Errorf("run id %q is not a 26-char Crockford ULID", id)
		}
	}
	// ULID property: lexicographic order follows time order.
	if !(a < b) {
		t.Errorf("ULIDs not time-ordered: %q !< %q", a, b)
	}
}

func TestMintRunID_Unique(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := MintRunID(now) // same millisecond: randomness must carry uniqueness
		if seen[id] {
			t.Fatalf("duplicate run id after %d mints: %s", i, id)
		}
		seen[id] = true
	}
}

// TestRunCycleFromPhase_ReusesRunRecordRunID — resume reuses the run
// record's identity: entries appended by a resumed cycle carry the ORIGINAL
// run id from the persisted CycleState, not a fresh mint and not "".
func TestRunCycleFromPhase_ReusesRunRecordRunID(t *testing.T) {
	t.Parallel()
	const orig = "01HZZZZZZZZZZZZZZZZZZZZZZZ"
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws", RunID: orig},
	}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseBuild), CycleID: 5}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if len(led.entries) == 0 {
		t.Fatal("no ledger entries recorded")
	}
	for i, e := range led.entries {
		if e.RunID != orig {
			t.Errorf("resumed entry %d (kind=%s) run_id=%q, want %q (run record reuse)", i, e.Kind, e.RunID, orig)
		}
	}
}

// TestRunCycleFromPhase_LegacyRecordMintsFresh — a pre-CA.5 run record (no
// run_id) gets a fresh ULID rather than empty attribution.
func TestRunCycleFromPhase_LegacyRecordMintsFresh(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws"},
	}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseBuild), CycleID: 5}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if len(led.entries) == 0 {
		t.Fatal("no ledger entries recorded")
	}
	id := led.entries[0].RunID
	if !crockfordRE.MatchString(id) {
		t.Fatalf("legacy resume minted %q, want a fresh ULID", id)
	}
	for i, e := range led.entries {
		if e.RunID != id {
			t.Errorf("entry %d run_id=%q, want %q", i, e.RunID, id)
		}
	}
}

// TestRunCycle_ThreadsRunIDEverywhere — the CA.5 acceptance: after one full
// cycle, the persisted CycleState carries the run id and every ledger entry
// the run emitted carries the SAME run id.
func TestRunCycle_ThreadsRunIDEverywhere(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if len(st.cycleStateLog) == 0 {
		t.Fatal("no CycleState writes recorded")
	}
	runID := st.cycleStateLog[0].RunID
	if !crockfordRE.MatchString(runID) {
		t.Fatalf("CycleState.RunID = %q, want a ULID", runID)
	}
	for i, cs := range st.cycleStateLog {
		if cs.RunID != runID {
			t.Errorf("CycleState write %d carries run id %q, want %q", i, cs.RunID, runID)
		}
	}
	if len(led.entries) == 0 {
		t.Fatal("no ledger entries recorded")
	}
	for i, e := range led.entries {
		if e.RunID != runID {
			t.Errorf("ledger entry %d (kind=%s) run_id=%q, want %q", i, e.Kind, e.RunID, runID)
		}
	}
}
