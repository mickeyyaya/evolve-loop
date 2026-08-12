package phaseoutputs

// phaseoutputs_test.go — per-cycle, per-phase output accounting: for every
// phase a cycle COMPLETED, did the workspace end up holding the data a
// reviewer needs?
//
// The question this answers is the operator's, verbatim: "can the system
// correctly track the output, and did each phase generate enough data for
// review?" Today nobody can answer it without hand-listing a workspace —
// which is how the pipeline ran for weeks with orchestrator-report.md having
// zero writers, dossiers 89% stubs, and a shadow record whose absence
// conflated "didn't comply" with "never ran". Absence only means something
// when somebody is looking for the presence.

import (
	"encoding/json"
	"strings"
	"testing"
)

func files(names ...string) map[string]int64 {
	m := map[string]int64{}
	for _, n := range names {
		m[n] = 1000 // non-trivial size by default
	}
	return m
}

func TestSurvey_CompletePhaseIsAccountedComplete(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"build"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	))
	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(got.Rows))
	}
	r := got.Rows[0]
	if !r.Report.Present || !r.Prompt.Present || !r.Events.Present || !r.Usage.Present {
		t.Errorf("a fully-instrumented phase reported gaps: %+v", r)
	}
	if len(got.Gaps()) != 0 {
		t.Errorf("no gaps expected, got %v", got.Gaps())
	}
}

// The load-bearing direction: a phase that COMPLETED but left no report is the
// silent-disposal shape again — work happened and nothing reviewable remains.
func TestSurvey_MissingReportIsAGap(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"build", "audit"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
		// audit completed but wrote nothing at all
	))
	gaps := got.Gaps()
	if len(gaps) == 0 {
		t.Fatal("a completed phase with NO artifacts reported no gap — the exact blindness this package exists to end")
	}
	joined := strings.Join(gaps, " | ")
	if !strings.Contains(joined, "audit") || !strings.Contains(joined, "audit-report.md") {
		t.Errorf("the gap must name the phase and the missing artifact; got %v", gaps)
	}
}

// An EMPTY artifact is not data. A zero-byte report satisfies a stat check and
// tells a reviewer nothing, so presence must be size-aware.
func TestSurvey_EmptyArtifactIsNotEnoughData(t *testing.T) {
	t.Parallel()
	fs := files("build-prompt.txt", "build-events.ndjson", "build-usage.json")
	fs["build-report.md"] = 0
	got := Survey([]string{"build"}, fs)
	r := got.Rows[0]
	if r.Report.Present && !r.Report.Empty {
		t.Error("a zero-byte report was counted as reviewable data")
	}
	if len(got.Gaps()) == 0 {
		t.Error("an empty report must surface as a gap — it satisfies a stat and informs nobody")
	}
}

// Phase→artifact naming is not uniform, and retro is the proof — its LIVE
// shape (verified in cycles 1432/1441/1442, corrected by review after a wrong
// first guess) is: registry-named report (retrospective-report.md), AGENT-named
// prompt (retrospective-prompt.txt — retro dispatches outside the runner path
// that keys streams on the phase name), PHASE-named usage (retro-usage.json —
// the C1 chokepoint writes %s-usage.json from the phase), and NO events stream
// under either name (retro-observer-events.ndjson exists but is the observer's,
// a different mechanism). This test pins all three namings — including the
// agent-named fallback candidate, whose only live consumer is retro's prompt —
// and pins that the missing events stream IS reported: it is a real
// instrumentation gap this tool exists to surface, not a naming artifact.
func TestSurvey_RetroLiveShapeExercisesTheAgentFallbackAndSurfacesTheEventsGap(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"retro"}, files(
		"retrospective-report.md", "retrospective-prompt.txt", "retro-usage.json",
	))
	gaps := got.Gaps()
	if len(gaps) != 1 {
		t.Fatalf("retro's live shape has exactly ONE gap (the events stream); got %v", gaps)
	}
	if !strings.Contains(gaps[0], "retro-events.ndjson") {
		t.Errorf("the one gap must be the events stream — the known live instrumentation defect; got %q", gaps[0])
	}
	r := got.Rows[0]
	if !r.Prompt.Present || r.Prompt.Name != "retrospective-prompt.txt" {
		t.Errorf("retro's prompt is AGENT-named live; the fallback candidate must find it: %+v", r.Prompt)
	}
	if !r.Usage.Present || r.Usage.Name != "retro-usage.json" {
		t.Errorf("retro's usage is PHASE-named live: %+v", r.Usage)
	}
}

// The exemption register is EMPTY by evidence: both development-time guesses
// (inherited-defect-reconcile, coverage-gate) are fully-dispatched phases with
// complete outputs in live workspaces (cycles 1432/1442). This pins that a
// phase nobody exempted gets full accounting — the guess must never silently
// return.
func TestSurvey_NoPhaseIsExemptWithoutLiveEvidence(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"inherited-defect-reconcile", "coverage-gate"}, map[string]int64{})
	gaps := got.Gaps()
	if len(gaps) == 0 {
		t.Fatal("phases with NO outputs surveyed clean — an exemption guess crept back in without the live cycle that justifies it")
	}
	for _, row := range got.Rows {
		if row.Exempt {
			t.Errorf("%s is exempt — the register must only grow with cited live evidence", row.Phase)
		}
	}
}

// The summary is what the wave monitor prints per cycle: phases surveyed,
// complete, and the gap list — one line an operator can scan.
func TestSurvey_SummaryLineIsScannable(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"build", "audit"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	))
	line := got.SummaryLine()
	if !strings.Contains(line, "1/2") {
		t.Errorf("summary must carry complete/surveyed counts; got %q", line)
	}
	if !strings.Contains(line, "audit") {
		t.Errorf("summary must name the gapped phase; got %q", line)
	}
}

// TestRowArtifactResult_AreTheWireShape names the three exported types
// (apicover) and pins the contract their JSON consumers depend on: the CLI's
// -json envelope embeds Row verbatim, so a renamed field here silently blanks
// a column in whatever the operator's tooling reads.
func TestRowArtifactResult_AreTheWireShape(t *testing.T) {
	t.Parallel()
	var res Result = Survey([]string{"build"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	))
	var row Row = res.Rows[0]
	var art Artifact = row.Report

	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"phase"`, `"report"`, `"prompt"`, `"events"`, `"usage"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("Row lost wire key %s: %s", key, raw)
		}
	}
	// Artifact's own keys are part of the same contract — a rename here blanks
	// a COLUMN in the consumer, not a row, which is quieter and worse.
	for _, key := range []string{`"name"`, `"present"`, `"bytes"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("Artifact lost wire key %s: %s", key, raw)
		}
	}
	if empties, err := json.Marshal(Artifact{Name: "x", Present: true, Empty: true}); err != nil || !strings.Contains(string(empties), `"empty"`) {
		t.Errorf("Artifact lost the empty marker: %s err=%v", empties, err)
	}
	if !art.Present || art.Bytes != 1000 {
		t.Errorf("Artifact observation drifted: %+v", art)
	}
}
