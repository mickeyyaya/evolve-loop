package core

// interaction_telemetry_test.go — ADR-0045 I1 (slice 1), orchestrator side:
// the contract-correction re-dispatch (PR #60) is an interaction and must
// record a typed Outcome resolved by the re-dispatch verdict; RunCycle's
// deferred persistence writes the per-cycle interaction-summary.json rollup
// beside phase-timing.json. White-box: reuses the fakeStorage / fakeLedger /
// buildRunners / sequencedReviewer harness.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// TestCorrectionRedispatch_RecordsInteractionOutcome — one reject→approve
// correction ⇒ exactly one correction_redispatch outcome in the build ledger
// (result accepted, rung redispatch, non-empty decision id), and the cycle
// rollup summarizes it.
func TestCorrectionRedispatch_RecordsInteractionOutcome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "deliverable missing required header"},
		{Approve: true},
	}}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle should proceed after one correction; got %v", err)
	}

	ws := cycleWorkspaceDir(root, res.Cycle)
	data, rerr := os.ReadFile(filepath.Join(ws, "build-interactions.ndjson"))
	if rerr != nil {
		t.Fatalf("build interaction ledger must exist after a correction: %v", rerr)
	}
	var outs []interaction.Outcome
	for _, ln := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if ln == "" {
			continue
		}
		var ou interaction.Outcome
		if err := json.Unmarshal([]byte(ln), &ou); err != nil {
			t.Fatalf("ledger line must parse: %v\n%s", err, ln)
		}
		outs = append(outs, ou)
	}
	if len(outs) != 1 {
		t.Fatalf("correction outcomes = %d, want exactly 1; ledger=%+v", len(outs), outs)
	}
	co := outs[0]
	if co.Kind != interaction.KindCorrectionRedispatch || co.Trigger != "contract_reject" || co.Rung != "redispatch" {
		t.Errorf("event shape wrong: %+v", co.Event)
	}
	if co.Result != interaction.ResultAccepted {
		t.Errorf("result = %q, want %q (the re-dispatch verdict resolves the outcome)", co.Result, interaction.ResultAccepted)
	}
	if co.DecisionID == "" {
		t.Error("decision id must correlate the rungs of one correction decision")
	}
	if !strings.Contains(co.Payload, "missing required header") {
		t.Errorf("payload should digest the violation; got %q", co.Payload)
	}

	// The per-cycle rollup is written beside phase-timing.json.
	sdata, serr := os.ReadFile(filepath.Join(ws, "interaction-summary.json"))
	if serr != nil {
		t.Fatalf("interaction-summary.json must be written by RunCycle's deferred persistence: %v", serr)
	}
	var s interaction.Summary
	if err := json.Unmarshal(sdata, &s); err != nil {
		t.Fatalf("summary must parse: %v\n%s", err, sdata)
	}
	if s.ByRung["redispatch"] != 1 || s.Decisions != 1 {
		t.Errorf("rollup must reflect the correction: %+v", s)
	}
}

// TestCorrectionRedispatch_ExhaustionRecordsRejectedAgain — corrections
// exhaust (always reject) ⇒ every re-dispatch records rejected_again under
// ONE decision id; the abort path still flushes the rollup (the deferred
// writer runs on error returns too, the C1 posture).
func TestCorrectionRedispatch_ExhaustionRecordsRejectedAgain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "still malformed"}},
	}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort after corrections exhausted")
	}

	ws := cycleWorkspaceDir(root, res.Cycle)
	data, rerr := os.ReadFile(filepath.Join(ws, "build-interactions.ndjson"))
	if rerr != nil {
		t.Fatalf("ledger must exist on the abort path too: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("outcomes = %d, want 2 (default EVOLVE_CONTRACT_CORRECTION_RETRIES)", len(lines))
	}
	ids := map[string]bool{}
	for _, ln := range lines {
		var ou interaction.Outcome
		if err := json.Unmarshal([]byte(ln), &ou); err != nil {
			t.Fatalf("ledger line must parse: %v", err)
		}
		if ou.Result != interaction.ResultRejectedAgain {
			t.Errorf("result = %q, want %q", ou.Result, interaction.ResultRejectedAgain)
		}
		ids[ou.DecisionID] = true
	}
	if len(ids) != 1 {
		t.Errorf("both re-dispatches belong to ONE decision; got ids %v", ids)
	}
	if _, serr := os.Stat(filepath.Join(ws, "interaction-summary.json")); serr != nil {
		t.Errorf("rollup must flush on the abort path: %v", serr)
	}
}
