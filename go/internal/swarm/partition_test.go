package swarm

import (
	"strings"
	"testing"
)

// fileWorker builds a worker that owns the given target files.
func fileWorker(id string, files ...string) WorkerSpec {
	return WorkerSpec{WorkerID: id, TargetFiles: files}
}

func writerPlan(workers ...WorkerSpec) SwarmPlan {
	return SwarmPlan{Mode: ModeWriter, Partitionable: true, Workers: workers}
}

func readerPlan(workers ...WorkerSpec) SwarmPlan {
	return SwarmPlan{Mode: ModeReader, Partitionable: true, Workers: workers}
}

func TestValidate_WriterDisjoint_OK(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "go/internal/foo/a.go", "go/internal/foo/a_test.go"),
		fileWorker("w1", "go/internal/bar/b.go"),
	)
	got := Validate(plan)
	if !got.OK || got.Collapse {
		t.Fatalf("disjoint writer plan should pass: %+v", got)
	}
	if len(got.MergeOrder) != 2 {
		t.Errorf("expected a 2-worker merge order, got %v", got.MergeOrder)
	}
}

func TestValidate_WriterOverlap_CollapsesToN1(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "go/internal/foo/a.go"),
		fileWorker("w1", "go/internal/foo/a.go"), // same file → conflict
	)
	got := Validate(plan)
	if got.OK || !got.Collapse {
		t.Fatalf("overlapping writer plan must collapse to N=1: %+v", got)
	}
	if len(got.Conflicts) != 1 || got.Conflicts[0].File != "go/internal/foo/a.go" {
		t.Errorf("expected one conflict on a.go, got %+v", got.Conflicts)
	}
	if len(got.Conflicts[0].Workers) != 2 {
		t.Errorf("conflict should name both workers, got %v", got.Conflicts[0].Workers)
	}
}

// Overlap must be caught even when the two spellings differ (./a.go vs a.go).
func TestValidate_WriterOverlap_NormalizedPaths(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "./go/internal/foo/a.go"),
		fileWorker("w1", "go/internal/foo/a.go"),
	)
	if got := Validate(plan); !got.Collapse {
		t.Errorf("path-normalized overlap must collapse: %+v", got)
	}
}

func TestValidate_WriterCyclicDAG_Collapses(t *testing.T) {
	plan := writerPlan(
		WorkerSpec{WorkerID: "w0", TargetFiles: []string{"a.go"}, DependsOn: []string{"w1"}},
		WorkerSpec{WorkerID: "w1", TargetFiles: []string{"b.go"}, DependsOn: []string{"w0"}},
	)
	got := Validate(plan)
	if !got.Collapse || !strings.Contains(got.Reason, "merge DAG") {
		t.Errorf("cyclic writer DAG must collapse with a DAG reason: %+v", got)
	}
}

func TestValidate_ReaderOverlap_Allowed(t *testing.T) {
	// Two readers focused on the same region — legal, no collapse.
	plan := readerPlan(
		fileWorker("w0", "go/internal/core/"),
		fileWorker("w1", "go/internal/core/"),
	)
	got := Validate(plan)
	if !got.OK || got.Collapse {
		t.Fatalf("overlapping reader plan should pass: %+v", got)
	}
	if got.MergeOrder != nil {
		t.Errorf("readers have no merge order, got %v", got.MergeOrder)
	}
}

func TestValidate_Fallback_NonPartitionable(t *testing.T) {
	plan := SwarmPlan{Mode: ModeWriter, Partitionable: false, Rationale: "inherently sequential",
		Workers: []WorkerSpec{fileWorker("w0", "a.go"), fileWorker("w1", "b.go")}}
	got := Validate(plan)
	if !got.Collapse || got.OK {
		t.Fatalf("non-partitionable plan must collapse: %+v", got)
	}
	if !strings.Contains(got.Reason, "non-partitionable") {
		t.Errorf("reason should explain non-partitionable, got %q", got.Reason)
	}
}

func TestValidate_Fallback_SingleWorker(t *testing.T) {
	plan := writerPlan(fileWorker("w0", "a.go"))
	if got := Validate(plan); !got.Collapse {
		t.Errorf("single-worker plan must collapse to N=1: %+v", got)
	}
}

func TestValidate_DuplicateWorkerID_Collapses(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "a.go"),
		fileWorker("w0", "b.go"), // same ID → silent dedupe hazard
	)
	if got := Validate(plan); !got.Collapse || !strings.Contains(got.Reason, "duplicate worker_id") {
		t.Errorf("duplicate worker_id must collapse: %+v", got)
	}
}

// Case-insensitive filesystems: two workers claiming the same file with
// different casing must collide (macOS/Windows treat them as one file).
func TestValidate_WriterOverlap_CaseInsensitive(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "go/internal/Foo/A.go"),
		fileWorker("w1", "go/internal/foo/a.go"),
	)
	if got := Validate(plan); !got.Collapse {
		t.Errorf("case-different overlap must collapse on case-insensitive FS: %+v", got)
	}
}

func TestValidate_UnknownMode_Collapses(t *testing.T) {
	plan := SwarmPlan{Mode: "bogus", Partitionable: true,
		Workers: []WorkerSpec{fileWorker("w0", "a.go"), fileWorker("w1", "b.go")}}
	if got := Validate(plan); !got.Collapse || !strings.Contains(got.Reason, "unknown swarm mode") {
		t.Errorf("unknown mode must collapse: %+v", got)
	}
}

func TestSwarmPlan_IsFallback(t *testing.T) {
	cases := []struct {
		name string
		plan SwarmPlan
		want bool
	}{
		{"non-partitionable", SwarmPlan{Partitionable: false, Workers: []WorkerSpec{{}, {}}}, true},
		{"single worker", SwarmPlan{Partitionable: true, Workers: []WorkerSpec{{}}}, true},
		{"ok", SwarmPlan{Partitionable: true, Workers: []WorkerSpec{{}, {}}}, false},
	}
	for _, tc := range cases {
		if got := tc.plan.IsFallback(); got != tc.want {
			t.Errorf("%s: IsFallback=%v want %v", tc.name, got, tc.want)
		}
	}
}

// ——— adversarial edge cases ———

// Reader plans with a cyclic DependsOn DAG must collapse — the planner authored a
// dependency cycle that is unreachable (readers rarely declare deps), which is a
// bug worth surfacing rather than silently ignoring.
func TestValidate_ReaderCyclicDAG_Collapses(t *testing.T) {
	plan := readerPlan(
		WorkerSpec{WorkerID: "w0", TargetFiles: []string{"a.go"}, DependsOn: []string{"w1"}},
		WorkerSpec{WorkerID: "w1", TargetFiles: []string{"b.go"}, DependsOn: []string{"w0"}},
	)
	got := Validate(plan)
	if !got.Collapse {
		t.Fatalf("cyclic reader DAG must collapse, got OK=%v Collapse=%v Reason=%q", got.OK, got.Collapse, got.Reason)
	}
	if !strings.Contains(got.Reason, "DAG") {
		t.Errorf("collapse reason should mention DAG, got %q", got.Reason)
	}
}

// fallbackReason coverage: three branches — non-partitionable with rationale,
// non-partitionable without rationale, and "too few workers".
func TestFallbackReason_WithRationale(t *testing.T) {
	plan := SwarmPlan{Partitionable: false, Rationale: "inherently sequential"}
	r := fallbackReason(plan)
	if !strings.Contains(r, "inherently sequential") {
		t.Errorf("fallbackReason with rationale must include it, got %q", r)
	}
}

func TestFallbackReason_NoRationale(t *testing.T) {
	plan := SwarmPlan{Partitionable: false, Rationale: ""}
	r := fallbackReason(plan)
	if !strings.Contains(r, "non-partitionable") {
		t.Errorf("fallbackReason without rationale must say non-partitionable, got %q", r)
	}
	if strings.Contains(r, ":") {
		// No colon suffix since there's no rationale to append.
		t.Errorf("fallbackReason without rationale should not have a colon, got %q", r)
	}
}

func TestFallbackReason_TooFewWorkers(t *testing.T) {
	plan := SwarmPlan{Partitionable: true, Workers: []WorkerSpec{{WorkerID: "w0"}}}
	r := fallbackReason(plan)
	if !strings.Contains(r, "1 worker") {
		t.Errorf("fallbackReason for single worker should mention count, got %q", r)
	}
}

func TestNormalizePath_EdgeCases(t *testing.T) {
	if got := normalizePath(""); got != "" {
		t.Errorf("empty path must normalize to empty, got %q", got)
	}
	if got := normalizePath("   "); got != "" {
		t.Errorf("whitespace-only path must normalize to empty, got %q", got)
	}
	if got := normalizePath("Foo/A.GO"); got != "foo/a.go" {
		t.Errorf("normalizePath must lowercase, got %q", got)
	}
	if got := normalizePath("./foo/bar/"); got != "foo/bar" {
		t.Errorf("normalizePath must clean and trim trailing slash, got %q", got)
	}
}
