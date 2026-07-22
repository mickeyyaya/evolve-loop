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

// assertCrossBucketDisjoint is the load-bearing concurrency invariant: every
// repo file is owned by AT MOST ONE bucket. Buckets run as CONCURRENT cycles
// (each its own `evolve cycle run` + worktree), so a file appearing in two
// buckets means two cycles edit it at once and collide on the shared tree at
// ship time. (Two todos in the SAME bucket touching one file is fine — one
// cycle, one worktree, sequential.)
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

// TestPartition_CrossBucketFileDisjoint: two todos touching the same file must
// land in the SAME bucket (one sequential cycle), never spread across buckets.
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
	assertCrossBucketDisjoint(t, buckets)
	// Independent (file-disjoint) todos spread to distinct buckets via
	// least-loaded assignment, so both cycles get work.
	if len(buckets[0]) != 1 || len(buckets[1]) != 1 {
		t.Fatalf("disjoint todos should spread one-per-bucket; got %v / %v", bucketIDs(buckets[0]), bucketIDs(buckets[1]))
	}
}

// TestPartition_SameFileTodos_ClusterInOneBucket: two todos sharing a file must
// land in the SAME bucket — one cycle runs them sequentially in one worktree, so
// no two CONCURRENT cycles ever touch shared.go. (The pre-fix algorithm spread
// them across buckets, which let two cycles collide on shared.go.)
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
	// a and b share shared.go → exactly one bucket holds both.
	for i, b := range buckets {
		if len(b) == 1 {
			t.Errorf("bucket %d holds only %v — same-file todos a,b must cluster together", i, bucketIDs(b))
		}
	}
}

// TestPartition_AllSameFile_AllClusterNoneDeferred: N todos all touching one file
// cannot be parallelized safely, so they all cluster into ONE cycle (the others
// stay empty) and NONE defer — deferring a same-file todo to a later wave would
// not help (it still needs exclusive ownership of that file).
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

// TestPartition_DeferOnlyWhenFilesBridgeBuckets: a todo whose files are split
// across TWO already-distinct buckets cannot be placed without bridging them
// into a cross-tree collision → it is deferred to a later wave.
func TestPartition_DeferOnlyWhenFilesBridgeBuckets(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"a.go"}},         // -> bucket 0
		{ID: "b", Files: []string{"b.go"}},         // -> bucket 1 (least-loaded)
		{ID: "c", Files: []string{"a.go", "b.go"}}, // bridges 0 and 1 -> defer
	}
	buckets, deferred := Partition(todos, 2)
	assertCrossBucketDisjoint(t, buckets)
	if len(deferred) != 1 || deferred[0].ID != "c" {
		t.Errorf("deferred=%v, want exactly [c] (it bridges buckets 0 and 1)", bucketIDs(deferred))
	}
}

// TestPartition_NormalizesPaths: ./a.go and a.go are the same file → the two
// todos cluster in one bucket (cross-bucket disjointness honors normalization).
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

// TestPlanCycles_DisjointScopesPlusDeferred: PlanCycles maps the partition into
// concurrent cycle specs, each carrying a DISJOINT, non-empty Scope and the
// EVOLVE_FLEET_SCOPE env the launched cycle reads; bridging todos defer.
func TestPlanCycles_DisjointScopesPlusDeferred(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"a.go"}},
		{ID: "b", Files: []string{"b.go"}},
		{ID: "c", Files: []string{"a.go", "b.go"}}, // bridges 0 and 1 -> deferred
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

// TestPlanCycles_SkipsEmptyBuckets: when the backlog can't fill `count` cycles
// (all work clusters), empty buckets produce NO spec — fewer cycles, not idle ones.
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

// locateTodo returns the bucket index of id (found=true, deferred=false), or
// deferred=true if id is in the deferred slice, or found=false if missing from
// both — a caller-contract violation.
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

// assertGraphConnectedNotSplit is the cycle-871 load-bearing invariant (rung 1
// of the merge ladder, research finding #2): two todos whose transitive
// package sets intersect (or either touches a global-zone file) must be
// co-scheduled into the SAME bucket, or deferred — NEVER split across two
// distinct concurrent buckets, since that would let two cycles independently
// touch reachable code and merge-skew (rename + call-site) on landing.
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

// TestPartitionGraph_PackageGraphConnectedTodos_NeverSplitAcrossBuckets pins
// the rung-1 fix: todo "a" edits go/internal/fleet/partition.go (package
// fleet) and todo "b" edits go/internal/ipcenv/ipcenv.go (package ipcenv,
// transitively imported BY fleet — see partition.go's import block). Their
// Files sets are disjoint, but the package graph connects them, so today's
// file-only Partition would wrongly spread them to two concurrent buckets —
// exactly the merge-skew gap (rename in ipcenv, call-site in fleet) research
// finding #2 warns about.
func TestPartitionGraph_PackageGraphConnectedTodos_NeverSplitAcrossBuckets(t *testing.T) {
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

// TestPartitionGraph_UnrelatedPackages_StillSpreadAcrossBuckets is the
// no-regression baseline: fleet and acsrunner have ZERO import relationship
// in either direction (verified via `go list -deps` at authoring time), so
// they must still spread to distinct concurrent buckets and NEVER defer —
// over-conflicting everything would defeat the point of fleet concurrency.
func TestPartitionGraph_UnrelatedPackages_StillSpreadAcrossBuckets(t *testing.T) {
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

// TestPartitionGraph_GlobalZoneFile_ConflictsWithEveryBucket pins the fixed
// global-zone list (go.mod, go.sum, policy/hook/generated files): a todo
// touching go.mod must conflict with every other bucket regardless of the
// package graph, since a go.mod edit can change ANY package's build.
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

// TestPartitionGraph_PureFileDisjointness_Unchanged pins the no-regression AC:
// with no package-graph or global-zone relationship at all, PartitionGraph's
// behavior matches plain Partition — same-file todos cluster, disjoint-file
// todos spread, bridging todos defer.
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

// TestPartition_NLessThanOne_DefaultsToOne keeps a degenerate n safe.
func TestPartition_NLessThanOne_DefaultsToOne(t *testing.T) {
	buckets, deferred := Partition([]Todo{{ID: "a"}}, 0)
	if len(buckets) != 1 || len(buckets[0]) != 1 || len(deferred) != 0 {
		t.Errorf("n=0 must default to 1 bucket holding the todo; got buckets=%v deferred=%v", buckets, deferred)
	}
}

// laneOwning returns the index of the single spec whose Scope owns id, or -1.
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

// TestPlanFromTriage_OverlappingDeclaredFilesCollapseToOneLane pins the cycle-523
// fix: two top_n cards declaring the SAME file must land in ONE lane. Before the
// fix PlanFromTriage set Todo{Files: []string{id}}, so distinct ids were trivially
// file-disjoint and spread to two colliding lanes (the fictional-disjoint defect
// inbox item wave-seed-partitions-on-id-not-real-files, weight 0.92).
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

// TestPlanFromTriage_DisjointDeclaredFilesSpreadToCountLanes is the baseline the
// fix must not over-collapse: cards touching disjoint files still spread to
// `count` lanes.
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

// TestPlanFromTriage_NoDeclaredFilesFallsBackToIdIsland pins the fallback: a card
// with no files[] stays an id-island, so file-less cards remain independent.
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

// TestPlanFromTriage_CommittedFloorsKeepIdIsland pins that bare string sources
// (committed_floors) are unaffected by the files change — each floor's id is its
// own footprint, so distinct floors still spread.
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
