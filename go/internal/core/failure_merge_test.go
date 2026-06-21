package core

import "testing"

// TestMergeFailedRecords_UnionsDiskAndIncoming: under concurrent fleet runs, a
// run's write must NOT clobber a peer's failure record already on disk. The merge
// keeps disk-only records (peer's), adds incoming-only records (this run's), and
// lets incoming win for a shared key (this run's own update, e.g. Retrospected).
func TestMergeFailedRecords_UnionsDiskAndIncoming(t *testing.T) {
	disk := []FailedRecord{
		{Cycle: 1, TS: "t1", Verdict: "FAIL"},
		{Cycle: 2, TS: "t2", Verdict: "FAIL"}, // a peer's concurrent record
	}
	incoming := []FailedRecord{
		{Cycle: 1, TS: "t1", Verdict: "FAIL", Retrospected: true}, // same key, updated
		{Cycle: 3, TS: "t3", Verdict: "WARN"},                     // this run's new record
	}
	got := mergeFailedRecords(disk, incoming)
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3 (cycle 1 merged, 2 preserved, 3 added): %+v", len(got), got)
	}
	byCycle := map[int]FailedRecord{}
	for _, r := range got {
		byCycle[r.Cycle] = r
	}
	if !byCycle[1].Retrospected {
		t.Error("cycle 1: incoming update (Retrospected) should win for a shared key")
	}
	if _, ok := byCycle[2]; !ok {
		t.Error("cycle 2: a peer's concurrent disk record must be preserved (not clobbered)")
	}
	if _, ok := byCycle[3]; !ok {
		t.Error("cycle 3: this run's new record must be added")
	}
}

func TestMergeCarryoverTodos_DedupByID(t *testing.T) {
	disk := []CarryoverTodo{{ID: "a"}, {ID: "b"}}     // b = a peer's concurrent todo
	incoming := []CarryoverTodo{{ID: "a"}, {ID: "c"}} // a = shared, c = new
	got := mergeCarryoverTodos(disk, incoming)
	if len(got) != 3 {
		t.Fatalf("got %d todos, want 3 (a,b,c): %+v", len(got), got)
	}
	ids := map[string]bool{}
	for _, td := range got {
		if ids[td.ID] {
			t.Errorf("duplicate todo id %q", td.ID)
		}
		ids[td.ID] = true
	}
	if !ids["a"] || !ids["b"] || !ids["c"] {
		t.Errorf("missing ids; got %v, want a,b,c", ids)
	}
}
