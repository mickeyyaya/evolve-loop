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
