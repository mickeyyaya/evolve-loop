package dossier

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
