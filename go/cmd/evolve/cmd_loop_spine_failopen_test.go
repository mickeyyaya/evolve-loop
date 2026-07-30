package main

// cmd_loop_spine_failopen_test.go — RED contract for the BATCH half of
// spine-failopen-telemetry (inbox weight 0.85).
//
// Cycle-1166 landed the per-cycle half: the spine gate records every fail-open
// into CycleResult.SpineFailOpens and finalizeCycle projects it into the
// committed dossier. It also landed dossier.RollupSpineFailOpens — but NOTHING
// in production called it, so the batch-level number the item actually asks for
// ("76 occurrences in one width-3 batch") was still nobody's output. A rollup no
// caller invokes is indistinguishable from the counter that never existed.
//
// The item names this test verbatim: TestLoopSummary_RollsUpSpineFailOpensPerBatch.
// Here it is asserted where the loop summary is actually produced — loopResult.emit,
// the single output chokepoint every exit path funnels through.
//
// RED today: loopResult has no SpineFailOpens field and spineFailOpenRollup does
// not exist — this file does not compile.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// spineFailOpenCycles is the fixture batch: a quiet cycle (1 event) and a noisy
// one (4 events, over the threshold of 3).
func spineFailOpenCycles() []core.CycleResult {
	return []core.CycleResult{
		{Cycle: 1160, FinalVerdict: "PASS", SpineFailOpens: []cyclestate.SpineFailOpen{
			{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"},
		}},
		{Cycle: 1161, FinalVerdict: "PASS", SpineFailOpens: []cyclestate.SpineFailOpen{
			{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
			{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
			{Phase: "audit", MissingArtifact: "tdd", Reason: "digest degraded: test-report.md"},
			{Phase: "ship", MissingArtifact: "audit", Reason: "would-block at enforce"},
		}},
	}
}

// TestLoopSummary_RollsUpSpineFailOpensPerBatch — the item's named RED, asserted
// through the PRODUCTION caller (emit). The batch total, the per-phase breakdown
// and the escalated cycles must all reach the summary JSON operators and
// dispatchers actually read.
func TestLoopSummary_RollsUpSpineFailOpensPerBatch(t *testing.T) {
	lr := loopResult{StopReason: "completed", Cycles: spineFailOpenCycles()}
	var buf bytes.Buffer
	lr.emit(&buf)

	var got struct {
		SpineFailOpens *struct {
			Total               int            `json:"total"`
			ByPhase             map[string]int `json:"by_phase"`
			OverThresholdCycles []int          `json:"over_threshold_cycles"`
		} `json:"spine_fail_opens"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal loop summary: %v\n%s", err, buf.String())
	}
	if got.SpineFailOpens == nil {
		t.Fatalf("loop summary carries no spine_fail_opens block:\n%s\n"+
			"76 fail-opens in one batch with nothing summing them is the defect — the "+
			"per-cycle records are useless if no batch surface reports them", buf.String())
	}
	if got.SpineFailOpens.Total != 5 {
		t.Errorf("spine_fail_opens.total = %d, want 5 (the SUM across the batch's cycles)", got.SpineFailOpens.Total)
	}
	if got.SpineFailOpens.ByPhase["audit"] != 3 || got.SpineFailOpens.ByPhase["ship"] != 2 {
		t.Errorf("spine_fail_opens.by_phase = %v, want audit=3 ship=2 — the breakdown is what "+
			"points at the phase whose predecessor artifact keeps vanishing", got.SpineFailOpens.ByPhase)
	}
	if len(got.SpineFailOpens.OverThresholdCycles) != 1 || got.SpineFailOpens.OverThresholdCycles[0] != 1161 {
		t.Errorf("spine_fail_opens.over_threshold_cycles = %v, want [1161] — only the cycle whose OWN "+
			"count breaches the threshold escalates", got.SpineFailOpens.OverThresholdCycles)
	}
}

// writeSpineDossier commits a cycle dossier the way a lane subprocess does: into
// the PARENT project root's knowledge-base/cycles.
func writeSpineDossier(t *testing.T, root string, cycle int, events []cyclestate.SpineFailOpen) {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := &dossier.Dossier{
		Cycle:          cycle,
		Goal:           "lane cycle",
		FinalVerdict:   "PASS",
		Phases:         []dossier.PhaseRecord{{Name: "cycle-recorded", Verdict: "PASS"}},
		SpineFailOpens: events,
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("cycle-%d.json", cycle)), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoopSummary_RollsUpFleetLaneCyclesFromCommittedDossiers is the mode the
// telemetry was FILED about: a width-N fleet batch. Every cycle runs in a lane
// subprocess, so lr.Cycles is EMPTY in the parent — a roll-up that folds only
// in-memory results reports zero for the very batches it exists to measure. The
// lanes' committed dossiers are the channel.
func TestLoopSummary_RollsUpFleetLaneCyclesFromCommittedDossiers(t *testing.T) {
	root := t.TempDir()
	writeSpineDossier(t, root, 1200, []cyclestate.SpineFailOpen{
		{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"},
	})
	writeSpineDossier(t, root, 1201, []cyclestate.SpineFailOpen{
		{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
		{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
		{Phase: "audit", MissingArtifact: "tdd", Reason: "digest degraded: test-report.md"},
		{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
	})
	// A pre-batch cycle that must NOT be counted: the window starts at 1200.
	writeSpineDossier(t, root, 1199, []cyclestate.SpineFailOpen{
		{Phase: "ship", MissingArtifact: "audit", Reason: "would-block at enforce"},
	})

	lr := loopResult{StopReason: "max_cycles", classifyRoot: root, batchFirstCycle: 1200}
	var buf bytes.Buffer
	lr.emit(&buf)

	if lr.SpineFailOpens == nil {
		t.Fatalf("a fleet batch (lr.Cycles empty, lanes' dossiers committed) rolled up to nothing:\n%s", buf.String())
	}
	if lr.SpineFailOpens.Total != 5 {
		t.Errorf("total = %d, want 5 — the pre-batch cycle 1199 must be outside the window and both "+
			"lane cycles inside it", lr.SpineFailOpens.Total)
	}
	if len(lr.SpineFailOpens.OverThresholdCycles) != 1 || lr.SpineFailOpens.OverThresholdCycles[0] != 1201 {
		t.Errorf("over_threshold_cycles = %v, want [1201]", lr.SpineFailOpens.OverThresholdCycles)
	}

	// An unknown window (batchFirstCycle 0) must NOT read the corpus — reporting
	// all of history as this batch's total is worse than reporting nothing.
	unbounded := loopResult{StopReason: "max_cycles", classifyRoot: root}
	var ubuf bytes.Buffer
	unbounded.emit(&ubuf)
	if unbounded.SpineFailOpens != nil {
		t.Errorf("an unknown batch window folded the corpus anyway: %+v", unbounded.SpineFailOpens)
	}
}

// TestSpineFailOpenRollup_PrefersTheCommittedDossierPerCycle pins the union rule:
// a cycle present in BOTH sources is counted once (from its dossier), and a cycle
// whose best-effort dossier write failed is still counted from memory.
func TestSpineFailOpenRollup_PrefersTheCommittedDossierPerCycle(t *testing.T) {
	root := t.TempDir()
	writeSpineDossier(t, root, 1160, []cyclestate.SpineFailOpen{
		{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"},
	})
	var warn bytes.Buffer
	// 1160 is in both sources; 1161 (4 events) only in memory.
	rollup := spineFailOpenRollup(spineFailOpenCycles(), root, 1160, &warn)
	if rollup == nil {
		t.Fatal("rollup = nil for a batch with fail-opens in both sources")
	}
	if rollup.Total != 5 {
		t.Errorf("total = %d, want 5 — a cycle in both sources must be counted ONCE, and a cycle with "+
			"no committed dossier must still be counted from memory", rollup.Total)
	}
	if len(rollup.OverThresholdCycles) != 1 || rollup.OverThresholdCycles[0] != 1161 {
		t.Errorf("over_threshold_cycles = %v, want [1161]", rollup.OverThresholdCycles)
	}
}

// TestSpineFailOpenRollup_WarnsOnThresholdBreach pins the human channel: a cycle
// over the threshold must produce a loud, self-explanatory stderr WARN naming the
// cycle and its count. The JSON block alone is a field in a blob nobody greps
// until after the batch is over.
func TestSpineFailOpenRollup_WarnsOnThresholdBreach(t *testing.T) {
	var warn bytes.Buffer
	rollup := spineFailOpenRollup(spineFailOpenCycles(), "", 0, &warn)
	if rollup == nil {
		t.Fatal("spineFailOpenRollup returned nil for a batch WITH fail-opens")
	}
	got := warn.String()
	if !strings.Contains(got, "WARN") || !strings.Contains(got, "1161") {
		t.Errorf("threshold-breach WARN = %q, want a WARN naming cycle 1161", got)
	}
	if !strings.Contains(got, "spine") {
		t.Errorf("WARN %q does not name the spine gate — an unattributable alarm is noise", got)
	}
}

// TestLoopSummary_CleanBatchOmitsSpineFailOpens is the NEGATIVE twin: a healthy
// batch must emit NO spine_fail_opens block and NO WARN. An always-present block
// (or an alarm that fires on a clean batch) is exactly how a measurement-first
// change gets ignored before it can measure anything.
func TestLoopSummary_CleanBatchOmitsSpineFailOpens(t *testing.T) {
	lr := loopResult{StopReason: "completed", Cycles: []core.CycleResult{
		{Cycle: 1162, FinalVerdict: "PASS"},
		{Cycle: 1163, FinalVerdict: "PASS"},
	}}
	var buf bytes.Buffer
	lr.emit(&buf)
	if strings.Contains(buf.String(), "spine_fail_opens") {
		t.Errorf("clean batch emitted a spine_fail_opens block:\n%s", buf.String())
	}

	var warn bytes.Buffer
	if rollup := spineFailOpenRollup(lr.Cycles, "", 0, &warn); rollup != nil {
		t.Errorf("clean batch rolled up to %+v, want nil (nothing to report)", rollup)
	}
	if warn.Len() != 0 {
		t.Errorf("clean batch WARNed %q, want silence", warn.String())
	}
	// An empty batch (every early-exit path calls emit with zero cycles) must not
	// panic or fabricate a block — including with a project root whose corpus is
	// absent entirely.
	if rollup := spineFailOpenRollup(nil, t.TempDir(), 1, &warn); rollup != nil {
		t.Errorf("empty batch rolled up to %+v, want nil", rollup)
	}
}
