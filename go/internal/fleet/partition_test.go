package fleet

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func bucketIDs(b []Todo) []string {
	ids := make([]string, len(b))
	for i, t := range b {
		ids[i] = t.ID
	}
	return ids
}

func assertCrossBucketDisjoint(t *testing.T, buckets [][]Todo) {
	t.Helper()
	owner := map[string]int{}
	for bi, b := range buckets {
		for _, td := range b {
			for _, f := range td.Files {
				key := filepath.Clean(f)
				if prev, ok := owner[key]; ok && prev != bi {
					t.Errorf("file %q owned by buckets %d AND %d — concurrent cycles would collide on the shared tree", key, prev, bi)
				}
				owner[key] = bi
			}
		}
	}
}

func TestPartition_CrossBucketFileDisjoint(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"shared.go"}},
		{ID: "b", Files: []string{"shared.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	assertCrossBucketDisjoint(t, buckets)
	if len(deferred) != 0 {
		t.Errorf("two same-file todos should cluster in one bucket, not defer: deferred=%v", deferred)
	}
}

func TestPartition_DisjointTodos_SpreadAcrossCycles(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"pkg/a.go"}},
		{ID: "b", Files: []string{"pkg/b.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	if len(deferred) != 0 {
		t.Fatalf("disjoint todos deferred: %v", deferred)
	}
	assertCrossBucketDisjoint(t, buckets)
	if len(buckets[0]) != 1 || len(buckets[1]) != 1 {
		t.Fatalf("disjoint todos should spread one-per-bucket; got %v / %v", bucketIDs(buckets[0]), bucketIDs(buckets[1]))
	}
}

func TestPartition_SameFileTodos_ClusterInOneBucket(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"shared.go", "x.go"}},
		{ID: "b", Files: []string{"shared.go", "y.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	assertCrossBucketDisjoint(t, buckets)
	if len(deferred) != 0 {
		t.Fatalf("unexpected deferred: %v", deferred)
	}
	for i, b := range buckets {
		if len(b) == 1 {
			t.Errorf("bucket %d holds only %v — same-file todos a,b must cluster together", i, bucketIDs(b))
		}
	}
}

func TestPartition_AllSameFile_AllClusterNoneDeferred(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"hot.go"}},
		{ID: "b", Files: []string{"hot.go"}},
		{ID: "c", Files: []string{"hot.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	assertCrossBucketDisjoint(t, buckets)
	if len(deferred) != 0 {
		t.Errorf("deferred=%v, want 0 — same-file todos cluster, never defer", deferred)
	}
	placed := 0
	for _, b := range buckets {
		placed += len(b)
	}
	if placed != 3 {
		t.Errorf("placed=%d, want all 3 in one cycle", placed)
	}
}

func TestPartition_DeferOnlyWhenFilesBridgeBuckets(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"a.go"}},
		{ID: "b", Files: []string{"b.go"}},
		{ID: "c", Files: []string{"a.go", "b.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	assertCrossBucketDisjoint(t, buckets)
	if len(deferred) != 1 || deferred[0].ID != "c" {
		t.Errorf("deferred=%v, want exactly [c] (it bridges buckets 0 and 1)", bucketIDs(deferred))
	}
}

func TestPartition_NormalizesPaths(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"./pkg/a.go"}},
		{ID: "b", Files: []string{"pkg/a.go"}},
	}
	buckets, deferred := Partition(todos, 2)
	assertCrossBucketDisjoint(t, buckets)
	if len(deferred) != 0 {
		t.Errorf("path-equal todos should cluster, not defer: %v", deferred)
	}
}

func TestPlanCycles_DisjointScopesPlusDeferred(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"a.go"}},
		{ID: "b", Files: []string{"b.go"}},
		{ID: "c", Files: []string{"a.go", "b.go"}},
	}
	specs, deferred := PlanCycles(todos, 2)
	if len(specs) != 2 {
		t.Fatalf("specs=%d want 2", len(specs))
	}
	seen := map[string]bool{}
	for i, s := range specs {
		if len(s.Scope) == 0 {
			t.Errorf("spec %d has empty Scope", i)
		}
		if s.Env[ipcenv.FleetScopeKey] == "" {
			t.Errorf("spec %d missing %s env", i, ipcenv.FleetScopeKey)
		}
		for _, id := range s.Scope {
			if seen[id] {
				t.Errorf("id %q assigned to two cycles", id)
			}
			seen[id] = true
		}
	}
	if len(deferred) != 1 || deferred[0].ID != "c" {
		t.Errorf("deferred=%v, want [c]", bucketIDs(deferred))
	}
}

func TestPlanCycles_SkipsEmptyBuckets(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"h.go"}},
		{ID: "b", Files: []string{"h.go"}},
	}
	specs, _ := PlanCycles(todos, 3)
	if len(specs) != 1 {
		t.Fatalf("specs=%d, want 1 (2 empty buckets skipped)", len(specs))
	}
	if len(specs[0].Scope) != 2 {
		t.Errorf("scope=%v, want both a,b clustered", specs[0].Scope)
	}
	if specs[0].Env[ipcenv.FleetScopeKey] != "a,b" {
		t.Errorf("EVOLVE_FLEET_SCOPE=%q, want a,b", specs[0].Env[ipcenv.FleetScopeKey])
	}
}

func locateTodo(buckets [][]Todo, deferred []Todo, id string) (bucket int, isDeferred bool, found bool) {
	for i, b := range buckets {
		for _, td := range b {
			if td.ID == id {
				return i, false, true
			}
		}
	}
	for _, td := range deferred {
		if td.ID == id {
			return -1, true, true
		}
	}
	return -1, false, false
}

func assertGraphConnectedNotSplit(t *testing.T, buckets [][]Todo, deferred []Todo, idA, idB string) {
	t.Helper()
	ba, da, fa := locateTodo(buckets, deferred, idA)
	bb, db, fb := locateTodo(buckets, deferred, idB)
	if !fa || !fb {
		t.Fatalf("todo missing from PartitionGraph result: %s found=%v, %s found=%v", idA, fa, idB, fb)
	}
	if !da && !db && ba != bb {
		t.Errorf("%s (bucket %d) and %s (bucket %d) are package-graph-connected but split across concurrent buckets", idA, ba, idB, bb)
	}
}

func TestPartitionGraph_PackageGraphConnectedTodos_NeverSplitAcrossBuckets(t *testing.T) {
	// Disjoint files, but package fleet imports ipcenv, so the package graph connects them.
	todos := []Todo{
		{ID: "a", Files: []string{"internal/fleet/partition.go"}},
		{ID: "b", Files: []string{"internal/ipcenv/ipcenv.go"}},
	}
	buckets, deferred, err := PartitionGraph(todos, 2, "../..")
	if err != nil {
		t.Fatalf("PartitionGraph: %v", err)
	}
	assertGraphConnectedNotSplit(t, buckets, deferred, "a", "b")
}

func TestPartitionGraph_UnrelatedPackages_StillSpreadAcrossBuckets(t *testing.T) {
	// fleet and acsrunner share no import edge in either direction.
	todos := []Todo{
		{ID: "a", Files: []string{"internal/acsrunner/runner.go"}},
		{ID: "b", Files: []string{"internal/fleet/partition.go"}},
	}
	buckets, deferred, err := PartitionGraph(todos, 2, "../..")
	if err != nil {
		t.Fatalf("PartitionGraph: %v", err)
	}
	if len(deferred) != 0 {
		t.Fatalf("unrelated packages must not defer: %v", bucketIDs(deferred))
	}
	ba, _, fa := locateTodo(buckets, deferred, "a")
	bb, _, fb := locateTodo(buckets, deferred, "b")
	if !fa || !fb {
		t.Fatalf("todo missing from result: a found=%v b found=%v", fa, fb)
	}
	if ba == bb {
		t.Errorf("unrelated packages a (bucket %d) and b (bucket %d) collapsed into the same bucket — over-conflicting defeats fleet concurrency", ba, bb)
	}
}

func TestPartitionGraph_GlobalZoneFile_ConflictsWithEveryBucket(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"internal/acsrunner/runner.go"}},
		{ID: "b", Files: []string{"go.mod"}},
	}
	buckets, deferred, err := PartitionGraph(todos, 2, "../..")
	if err != nil {
		t.Fatalf("PartitionGraph: %v", err)
	}
	assertGraphConnectedNotSplit(t, buckets, deferred, "a", "b")
}

func TestPartitionGraph_PureFileDisjointness_Unchanged(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"internal/fleet/partition.go"}},
		{ID: "b", Files: []string{"internal/fleet/partition.go"}},
	}
	buckets, deferred, err := PartitionGraph(todos, 2, "../..")
	if err != nil {
		t.Fatalf("PartitionGraph: %v", err)
	}
	if len(deferred) != 0 {
		t.Fatalf("same-file todos should cluster, not defer: %v", bucketIDs(deferred))
	}
	assertCrossBucketDisjoint(t, buckets)
	for i, b := range buckets {
		if len(b) == 1 {
			t.Errorf("bucket %d holds only %v — same-file todos a,b must cluster together", i, bucketIDs(b))
		}
	}
}

func TestPartition_NLessThanOne_DefaultsToOne(t *testing.T) {
	buckets, deferred := Partition([]Todo{{ID: "a"}}, 0)
	if len(buckets) != 1 || len(buckets[0]) != 1 || len(deferred) != 0 {
		t.Errorf("n=0 must default to 1 bucket holding the todo; got buckets=%v deferred=%v", buckets, deferred)
	}
}

func laneOwning(specs []CycleSpec, id string) int {
	for i, s := range specs {
		for _, x := range s.Scope {
			if x == id {
				return i
			}
		}
	}
	return -1
}

func TestPlanFromTriage_OverlappingDeclaredFilesCollapseToOneLane(t *testing.T) {
	decision := []byte(`{"top_n":[
		{"id":"alpha","files":["go/internal/fleet/triageplan.go"]},
		{"id":"beta","files":["go/internal/fleet/triageplan.go"]}
	]}`)
	specs, _, err := PlanFromTriage(decision, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("two cards sharing a declared file must collapse to 1 lane, got %d", len(specs))
	}
	if a, b := laneOwning(specs, "alpha"), laneOwning(specs, "beta"); a != b || a < 0 {
		t.Errorf("alpha (lane %d) and beta (lane %d) share a file but are not co-located", a, b)
	}
}

func TestPlanFromTriage_DisjointDeclaredFilesSpreadToCountLanes(t *testing.T) {
	decision := []byte(`{"top_n":[
		{"id":"alpha","files":["go/internal/fleet/a.go"]},
		{"id":"beta","files":["go/internal/fleet/b.go"]}
	]}`)
	specs, _, err := PlanFromTriage(decision, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("disjoint cards at count=2 must spread to 2 lanes, got %d", len(specs))
	}
	if a, b := laneOwning(specs, "alpha"), laneOwning(specs, "beta"); a == b {
		t.Errorf("disjoint alpha and beta collapsed into the same lane %d", a)
	}
}

func TestPlanFromTriage_NoDeclaredFilesFallsBackToIdIsland(t *testing.T) {
	decision := []byte(`{"top_n":[{"id":"gamma"},{"id":"delta"}]}`)
	specs, _, err := PlanFromTriage(decision, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("two file-less cards must fall back to id-islands and spread to 2 lanes, got %d", len(specs))
	}
}

func TestPlanFromTriage_CommittedFloorsKeepIdIsland(t *testing.T) {
	decision := []byte(`{"committed_floors":["pkg/one","pkg/two"]}`)
	specs, _, err := PlanFromTriage(decision, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("two distinct committed floors at count=2 must spread to 2 lanes, got %d", len(specs))
	}
}
