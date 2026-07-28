package dossier

// spine_failopen_rollup_test.go — RED contract for cycle-1166 Task 3
// (spine-failopen-telemetry, inbox weight 0.85), dossier half. The core half
// (recording the events) lives in internal/core/spine_failopen_telemetry_test.go.
//
// The item names two RED tests verbatim:
//   - TestSpineFailOpen_CountedInDossierWithPhaseAndArtifact
//   - TestLoopSummary_RollsUpSpineFailOpensPerBatch
//
// …plus "WARN escalation when a single cycle exceeds a threshold (e.g. 3)".
//
// The wiring follows the EXISTING SkippedPhases precedent exactly
// (cyclestate.SkippedPhase → BuildOpts.SkippedPhases → Dossier.SkippedPhases,
// build.go:78/113) rather than inventing a new shape — the scout report flags
// that precedent, and single-source-with-projection is the standing rule.
//
// RED today: cyclestate.SpineFailOpen, BuildOpts.SpineFailOpens,
// Dossier.SpineFailOpens and RollupSpineFailOpens do not exist — this file does
// not compile.
//
// Contract Builder must satisfy:
//
//	type cyclestate.SpineFailOpen struct {
//	    Phase           string `json:"phase"`
//	    MissingArtifact string `json:"missing_artifact"`
//	    Reason          string `json:"reason,omitempty"`
//	}
//	BuildOpts.SpineFailOpens []cyclestate.SpineFailOpen
//	Dossier.SpineFailOpens   []cyclestate.SpineFailOpen `json:"spine_fail_opens,omitempty"`
//	type SpineFailOpenRollup struct {
//	    Total               int
//	    ByPhase             map[string]int
//	    OverThresholdCycles []int   // cycles whose OWN count exceeded threshold
//	}
//	func RollupSpineFailOpens(ds []*Dossier, threshold int) SpineFailOpenRollup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func spineFailOpenBuildOpts(t *testing.T, events []cyclestate.SpineFailOpen) BuildOpts {
	t.Helper()
	return BuildOpts{
		WorkspacePath:  t.TempDir(),
		Goal:           "cycle-1166 spine fail-open telemetry",
		FinalVerdict:   VerdictPass,
		SpineFailOpens: events,
	}
}

// TestSpineFailOpen_CountedInDossierWithPhaseAndArtifact — the item's first
// named RED test. A cycle's fail-open events must reach the committed dossier
// with BOTH the phase that proceeded and the predecessor artifact that was
// missing, and must survive the JSON round-trip (the dossier's on-disk form is
// the only surface an operator or a later sweep can read).
func TestSpineFailOpen_CountedInDossierWithPhaseAndArtifact(t *testing.T) {
	events := []cyclestate.SpineFailOpen{
		{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"},
		{Phase: "audit", MissingArtifact: "build", Reason: "digest degraded: build-report.md"},
	}
	d, err := Build(1166, spineFailOpenBuildOpts(t, events))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(d.SpineFailOpens) != len(events) {
		t.Fatalf("dossier carries %d spine fail-opens, want %d — the dossier is where the "+
			"epidemic becomes visible; dropping events here is the status quo this item removes",
			len(d.SpineFailOpens), len(events))
	}
	if d.SpineFailOpens[0].Phase != "ship" || d.SpineFailOpens[0].MissingArtifact != "build" {
		t.Errorf("first record = %+v, want Phase=ship MissingArtifact=build — the (phase, artifact) "+
			"PAIR is what makes 76 WARNs groupable by cause", d.SpineFailOpens[0])
	}

	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal dossier: %v", err)
	}
	if !strings.Contains(string(raw), `"spine_fail_opens"`) {
		t.Errorf("serialized dossier has no spine_fail_opens key — an in-memory-only counter is " +
			"invisible to every operator surface, which is the defect being fixed")
	}
	if !strings.Contains(string(raw), `"missing_artifact"`) {
		t.Errorf("serialized record omits missing_artifact — phase alone does not identify the cause")
	}
}

// TestSpineFailOpen_HealthyCycleOmitsTheField is the NEGATIVE twin. A cycle with
// zero fail-opens must serialize with NO spine_fail_opens key (omitempty), so an
// operator scanning dossiers sees the field only where there is something to see
// — and so a degenerate always-emit implementation cannot pass the test above.
func TestSpineFailOpen_HealthyCycleOmitsTheField(t *testing.T) {
	d, err := Build(1166, spineFailOpenBuildOpts(t, nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(d.SpineFailOpens) != 0 {
		t.Fatalf("healthy cycle carries %d fail-open records, want 0", len(d.SpineFailOpens))
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal dossier: %v", err)
	}
	if strings.Contains(string(raw), "spine_fail_opens") {
		t.Error("a cycle with zero fail-opens still emitted the spine_fail_opens key — " +
			"the field must be omitempty, mirroring skipped_phases")
	}
}

// TestLoopSummary_RollsUpSpineFailOpensPerBatch — the item's second named RED
// test. A width-N batch's summary must state the batch TOTAL, the breakdown by
// phase, and which cycles individually breached the WARN threshold ("e.g. 3").
// Per-cycle records alone do not surface an epidemic; the rollup is the dashboard.
func TestLoopSummary_RollsUpSpineFailOpensPerBatch(t *testing.T) {
	quiet, err := Build(1160, spineFailOpenBuildOpts(t, []cyclestate.SpineFailOpen{
		{Phase: "ship", MissingArtifact: "build", Reason: "would-block at enforce"},
	}))
	if err != nil {
		t.Fatalf("Build quiet cycle: %v", err)
	}
	// A cycle over the threshold of 3: four events, three of them on audit.
	noisy, err := Build(1161, spineFailOpenBuildOpts(t, []cyclestate.SpineFailOpen{
		{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
		{Phase: "audit", MissingArtifact: "build", Reason: "would-block at enforce"},
		{Phase: "audit", MissingArtifact: "tdd", Reason: "digest degraded: test-report.md"},
		{Phase: "ship", MissingArtifact: "audit", Reason: "would-block at enforce"},
	}))
	if err != nil {
		t.Fatalf("Build noisy cycle: %v", err)
	}

	got := RollupSpineFailOpens([]*Dossier{quiet, noisy}, 3)

	if got.Total != 5 {
		t.Errorf("batch Total = %d, want 5 — the rollup must SUM across the batch's cycles", got.Total)
	}
	if got.ByPhase["audit"] != 3 {
		t.Errorf("ByPhase[audit] = %d, want 3 — the per-phase breakdown is what points at the "+
			"phase whose predecessor artifact keeps vanishing", got.ByPhase["audit"])
	}
	if got.ByPhase["ship"] != 2 {
		t.Errorf("ByPhase[ship] = %d, want 2", got.ByPhase["ship"])
	}
	if len(got.OverThresholdCycles) != 1 || got.OverThresholdCycles[0] != 1161 {
		t.Errorf("OverThresholdCycles = %v, want [1161] — only the cycle whose OWN count exceeds "+
			"the threshold escalates; the batch total must not drag a quiet cycle over",
			got.OverThresholdCycles)
	}
}

// TestRollupSpineFailOpens_CleanBatchIsSilent is the NEGATIVE twin of the
// rollup: a batch where no cycle fails open must produce a zero rollup with NO
// escalation. An alarm that fires on a clean batch is noise, and it is exactly
// how a "measurement-first" change gets rolled back before it can measure anything.
func TestRollupSpineFailOpens_CleanBatchIsSilent(t *testing.T) {
	a, err := Build(1162, spineFailOpenBuildOpts(t, nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := Build(1163, spineFailOpenBuildOpts(t, nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	got := RollupSpineFailOpens([]*Dossier{a, b}, 3)

	if got.Total != 0 {
		t.Errorf("clean batch Total = %d, want 0", got.Total)
	}
	if len(got.OverThresholdCycles) != 0 {
		t.Errorf("clean batch escalated cycles %v, want none", got.OverThresholdCycles)
	}
	if got := RollupSpineFailOpens(nil, 3); got.Total != 0 || len(got.OverThresholdCycles) != 0 {
		t.Errorf("empty batch rollup = %+v, want a zero value (no panic, no escalation)", got)
	}
}

// TestSpineFailOpenRollup_ZeroValueIsSafeToRead names the rollup type itself and
// pins that a zero value is readable without a nil-map panic guard at every call
// site — an operator surface that panics on an empty batch reports nothing.
func TestSpineFailOpenRollup_ZeroValueIsSafeToRead(t *testing.T) {
	var zero SpineFailOpenRollup
	if zero.Total != 0 || len(zero.OverThresholdCycles) != 0 || zero.ByPhase["ship"] != 0 {
		t.Errorf("zero SpineFailOpenRollup = %+v, want an all-empty summary", zero)
	}
	got := RollupSpineFailOpens(nil, 0)
	if got.ByPhase == nil {
		t.Error("RollupSpineFailOpens must always allocate ByPhase so callers can read it directly")
	}
}
