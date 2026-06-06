// cmd_loop_cyclelevel_test.go — cycle-234 task `cycle-level-bridge-failure` (RED).
//
// Loop-side half of Invariant 3: when RunCycle returns a CYCLE-level failure
// (core.ErrCycleLevelFailure — bridge exhaustion, contract dead-end, …), the
// batch must log it and CONTINUE to the next cycle. Today cmd_loop.go breaks
// on ANY error with StopReason="error" rc=2 — the exact batch-fatal inversion
// that killed batches c225/c230/c231.
//
// Uses the wireOrchestratorDepsFn seam + the m4 stub fakes (noopRunner,
// newFakeLedger, fixtures.FakeStorage) defined in cmd_loop_m4_test.go.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// secondCallErrRunner PASSes every invocation except its 2nd one, which
// fails with a plain (non-transient, non-integrity) error. In a 3-cycle
// batch with one scout dispatch per cycle, that makes cycle 2 the failing
// cycle while cycles 1 and 3 succeed.
type secondCallErrRunner struct {
	name  string
	calls int
}

func (r *secondCallErrRunner) Name() string { return r.name }
func (r *secondCallErrRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	if r.calls == 2 {
		return core.PhaseResponse{}, errors.New("synthetic scout bridge death (cycle-level)")
	}
	return core.PhaseResponse{Verdict: core.VerdictPASS}, nil
}

// TestLoop_CycleLevelFailureContinues — scout AC: "loop with 3 cycles,
// second fails with ErrCycleLevelFailure → third cycle still runs".
func TestLoop_CycleLevelFailureContinues(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")

	projectRoot := t.TempDir()
	scout := &secondCallErrRunner{name: "scout"}

	prev := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prev }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		st := &fixtures.FakeStorage{}
		ld := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseIntent:       noopRunner{name: "intent"},
			core.PhaseScout:        scout,
			core.PhaseTriage:       noopRunner{name: "triage"},
			core.PhaseTDD:          noopRunner{name: "tdd"},
			core.PhaseBuildPlanner: noopRunner{name: "build-planner"},
			core.PhaseBuild:        noopRunner{name: "build"},
			core.PhaseAudit:        noopRunner{name: "audit"},
			core.PhaseShip:         noopRunner{name: "ship"},
			core.PhaseRetro:        noopRunner{name: "retro"},
		}
		return orchDeps{Storage: st, Ledger: ld, Orchestrator: core.NewOrchestrator(st, ld, runners)}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--goal-text", "supervision tree",
		"--cycles", "3",
	}, nil, &stdout, &stderr)

	// 1. Batch survives: rc=2 is the batch-fatal contract and must NOT fire
	// for a cycle-level failure. (rc=0 or the recoverable-failure rc=3 are
	// both acceptable "batch completed" codes.)
	if rc == 2 {
		t.Fatalf("rc=2 (batch-fatal) for a cycle-level failure — batch must continue; stderr=%q stdout=%q",
			stderr.String(), stdout.String())
	}

	// 2. The loop ran to its cycle cap instead of stopping at the failure.
	var lr struct {
		StopReason string           `json:"stop_reason"`
		Cycles     []map[string]any `json:"cycles"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &lr); err != nil {
		t.Fatalf("loop output is not the canonical JSON envelope: %v\n%s", err, stdout.String())
	}
	if lr.StopReason != "max_cycles" {
		t.Errorf("stop_reason=%q, want \"max_cycles\" (the failed cycle must not stop the batch)", lr.StopReason)
	}
	if len(lr.Cycles) != 3 {
		t.Errorf("cycles in batch output = %d, want 3 (cycle 2 failed, cycles 1+3 ran)", len(lr.Cycles))
	}

	// 3. The third cycle actually DISPATCHED (not just an empty result row).
	if scout.calls != 3 {
		t.Errorf("scout dispatches = %d, want 3 — the cycle after the failure never ran", scout.calls)
	}

	// 4. The failure is LOGGED (fail loudly), not swallowed.
	if !strings.Contains(stderr.String(), "synthetic scout bridge death") {
		t.Errorf("stderr does not mention the cycle failure cause; failures must be logged loudly\nstderr=%q", stderr.String())
	}
}
