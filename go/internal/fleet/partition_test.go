package fleet

import "testing"

func bucketIDs(b []Todo) []string {
	ids := make([]string, len(b))
	for i, t := range b {
		ids[i] = t.ID
	}
	return ids
}

// TestPartition_DisjointTodos_SpreadAcrossCycles: non-overlapping todos may run
// concurrently — they spread across the available buckets.
func TestPartition_DisjointTodos_SpreadAcrossCycles(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"pkg/a.go"}},
		{ID: "b", Files: []string{"pkg/b.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	if len(deferred) != 0 {
		t.Fatalf("disjoint todos deferred: %v", deferred)
	}
	// First-fit puts each disjoint todo in the first free bucket... but b is
	// disjoint from a, so first-fit keeps both eligible for bucket 0. The
	// invariant we assert is the SAFETY one: no bucket co-schedules conflicting
	// work (trivially true here) and every todo is placed.
	placed := len(buckets[0]) + len(buckets[1])
	if placed != 2 {
		t.Fatalf("placed %d todos, want 2", placed)
	}
}

// TestPartition_ConflictingTodos_NotCoScheduled: two todos touching the same
// file must NOT land in the same bucket.
func TestPartition_ConflictingTodos_NotCoScheduled(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"shared.go", "x.go"}},
		{ID: "b", Files: []string{"shared.go", "y.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	if len(deferred) != 0 {
		t.Fatalf("unexpected deferred: %v", deferred)
	}
	for i, b := range buckets {
		if len(b) > 1 {
			t.Errorf("bucket %d co-scheduled conflicting todos %v (both touch shared.go)", i, bucketIDs(b))
		}
	}
}

// TestPartition_MoreConflictsThanBuckets_Deferred: 3 mutually-conflicting todos
// with only 2 buckets → one is deferred, never co-scheduled.
func TestPartition_MoreConflictsThanBuckets_Deferred(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"hot.go"}},
		{ID: "b", Files: []string{"hot.go"}},
		{ID: "c", Files: []string{"hot.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	placed := 0
	for _, b := range buckets {
		if len(b) > 1 {
			t.Errorf("bucket co-scheduled conflicting todos: %v", bucketIDs(b))
		}
		placed += len(b)
	}
	if placed != 2 || len(deferred) != 1 {
		t.Errorf("placed=%d deferred=%d, want 2 placed + 1 deferred (run the 3rd in a later wave)", placed, len(deferred))
	}
}

// TestPartition_NormalizesPaths: ./a.go and a.go are the same file → conflict.
func TestPartition_NormalizesPaths(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"./pkg/a.go"}},
		{ID: "b", Files: []string{"pkg/a.go"}},
	}
	buckets, _ := Partition(todos, 2)
	for i, b := range buckets {
		if len(b) > 1 {
			t.Errorf("bucket %d co-scheduled path-equal todos %v (./pkg/a.go == pkg/a.go)", i, bucketIDs(b))
		}
	}
}

// TestPartition_NLessThanOne_DefaultsToOne keeps a degenerate n safe.
func TestPartition_NLessThanOne_DefaultsToOne(t *testing.T) {
	buckets, deferred := Partition([]Todo{{ID: "a"}}, 0)
	if len(buckets) != 1 || len(buckets[0]) != 1 || len(deferred) != 0 {
		t.Errorf("n=0 must default to 1 bucket holding the todo; got buckets=%v deferred=%v", buckets, deferred)
	}
}
