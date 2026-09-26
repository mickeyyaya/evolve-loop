package core

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
)

func TestOrchestrator_WithDirectivesProvider_SnapshotStampThread(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	want := directives.Set{Merged: "## Operator Directives\n\nBe env-agnostic.", Version: "abc123"}
	var calls int32
	o := NewOrchestrator(st, led, runners, WithDirectivesProvider(func(context.Context, int) directives.Set {
		atomic.AddInt32(&calls, 1)
		return want
	}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("provider called %d times, want exactly 1 (one snapshot per cycle)", n)
	}

	stamped := 0
	for _, e := range led.entries {
		if e.Kind == "operator_directives" {
			stamped++
			if e.Action != want.Version {
				t.Errorf("ledger directives version = %q, want %q", e.Action, want.Version)
			}
		}
	}
	if stamped != 1 {
		t.Fatalf("want exactly 1 operator_directives ledger entry, got %d", stamped)
	}

	scout, ok := runners[PhaseScout].(*fakeRunner)
	if !ok || len(scout.requests) == 0 {
		t.Fatal("scout runner did not run")
	}
	if scout.requests[0].OperatorDirectives != want.Merged {
		t.Errorf("phase OperatorDirectives = %q, want %q", scout.requests[0].OperatorDirectives, want.Merged)
	}
}

func TestOrchestrator_WithDirectivesProvider_EmptySetNoStamp(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners, WithDirectivesProvider(func(context.Context, int) directives.Set {
		return directives.Set{}
	}))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	for _, e := range led.entries {
		if e.Kind == "operator_directives" {
			t.Errorf("empty Set must not stamp the ledger; got %+v", e)
		}
	}
	if scout, ok := runners[PhaseScout].(*fakeRunner); ok && len(scout.requests) > 0 {
		if scout.requests[0].OperatorDirectives != "" {
			t.Errorf("empty Set must thread empty directives; got %q", scout.requests[0].OperatorDirectives)
		}
	}
}
