package phaseoutputs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func files(names ...string) map[string]int64 {
	m := map[string]int64{}
	for _, n := range names {
		m[n] = 1000
	}
	return m
}

func TestSurvey_CompletePhaseIsAccountedComplete(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"build"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	), phasecontract.BuiltinResolver{})
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

func TestSurvey_MissingReportIsAGap(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"build", "audit"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	), phasecontract.BuiltinResolver{})
	gaps := got.Gaps()
	if len(gaps) == 0 {
		t.Fatal("a completed phase with NO artifacts reported no gap — the exact blindness this package exists to end")
	}
	joined := strings.Join(gaps, " | ")
	if !strings.Contains(joined, "audit") || !strings.Contains(joined, "audit-report.md") {
		t.Errorf("the gap must name the phase and the missing artifact; got %v", gaps)
	}
}

func TestSurvey_EmptyArtifactIsNotEnoughData(t *testing.T) {
	t.Parallel()
	fs := files("build-prompt.txt", "build-events.ndjson", "build-usage.json")
	fs["build-report.md"] = 0
	got := Survey([]string{"build"}, fs, phasecontract.BuiltinResolver{})
	r := got.Rows[0]
	if r.Report.Present && !r.Report.Empty {
		t.Error("a zero-byte report was counted as reviewable data")
	}
	if len(got.Gaps()) == 0 {
		t.Error("an empty report must surface as a gap — it satisfies a stat and informs nobody")
	}
}

func TestSurvey_RetroLiveShapeExercisesTheAgentFallbackAndSurfacesTheEventsGap(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"retro"}, files(
		"retrospective-report.md", "retrospective-prompt.txt", "retro-usage.json",
	), phasecontract.BuiltinResolver{})
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

func TestSurvey_NoPhaseIsExemptWithoutLiveEvidence(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"inherited-defect-reconcile", "coverage-gate"}, map[string]int64{}, phasecontract.BuiltinResolver{})
	gaps := got.Gaps()
	if len(gaps) != 8 {
		t.Fatalf("two dispatched phases with NO outputs owe all four artifacts each; got %d gaps: %v", len(gaps), gaps)
	}
}

func TestSurvey_NativeShipOwesOnlyUsage(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"ship"}, files("ship-usage.json"), phasecontract.BuiltinResolver{})
	if gaps := got.Gaps(); len(gaps) != 0 {
		t.Errorf("ship's live shape (usage only) is complete — the wave must not report false ship gaps: %v", gaps)
	}
	r := got.Rows[0]
	if !r.Report.NotOwed || !r.Prompt.NotOwed || !r.Events.NotOwed {
		t.Errorf("non-owed artifacts must be marked so a rows consumer cannot re-derive them as gaps: %+v", r)
	}
	if r.Usage.NotOwed {
		t.Error("usage is owed by every phase, native included")
	}
	if gaps := Survey([]string{"ship"}, map[string]int64{}, phasecontract.BuiltinResolver{}).Gaps(); len(gaps) != 1 || !strings.Contains(gaps[0], "ship-usage.json") {
		t.Errorf("a ship with no usage sidecar is a real recording failure: %v", gaps)
	}
}

type memoAwareResolver struct{}

func (memoAwareResolver) Resolve(name string) (phasecontract.Contract, bool) {
	if name == "memo" {
		return phasecontract.Contract{Phase: "memo", AgentName: "memo", ArtifactName: "memo.md"}, true
	}
	return phasecontract.BuiltinResolver{}.Resolve(name)
}

func TestSurvey_ReportNameComesFromTheResolver(t *testing.T) {
	t.Parallel()
	live := files("memo.md", "memo-prompt.txt", "memo-events.ndjson", "memo-usage.json")
	if gaps := Survey([]string{"memo"}, live, memoAwareResolver{}).Gaps(); len(gaps) != 0 {
		t.Errorf("memo.md is the resolver-declared report; surveying memo-report.md was the cycle-1452 false gap: %v", gaps)
	}
	degraded := Survey([]string{"memo"}, live, phasecontract.BuiltinResolver{})
	if gaps := degraded.Gaps(); len(gaps) != 1 || !strings.Contains(gaps[0], "memo-report.md") {
		t.Errorf("builtin-only degrade falls back to the convention and reports honestly: %v", gaps)
	}
}

func TestNativePhases_RegisterIsExactlyShip(t *testing.T) {
	t.Parallel()
	if len(nativePhases) != 1 || !nativePhases["ship"] {
		t.Errorf("nativePhases = %v — admission requires a cited live cycle and a deliberate change here", nativePhases)
	}
}

func TestSurvey_DuplicateCompletedPhasesCollapseToOneRow(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"audit", "audit"}, files(
		"audit-report.md", "audit-prompt.txt", "audit-events.ndjson", "audit-usage.json",
	), phasecontract.BuiltinResolver{})
	if len(got.Rows) != 1 {
		t.Fatalf("audit completed twice is still ONE phase to account: got %d rows", len(got.Rows))
	}
	if !strings.Contains(got.SummaryLine(), "1/1") {
		t.Errorf("the denominator must count distinct phases: %q", got.SummaryLine())
	}
}

func TestSurvey_SummaryLineIsScannable(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"build", "audit"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	), phasecontract.BuiltinResolver{})
	line := got.SummaryLine()
	if !strings.Contains(line, "1/2") {
		t.Errorf("summary must carry complete/surveyed counts; got %q", line)
	}
	if !strings.Contains(line, "audit") {
		t.Errorf("summary must name the gapped phase; got %q", line)
	}
}

func TestRowArtifactResult_AreTheWireShape(t *testing.T) {
	t.Parallel()
	var res Result = Survey([]string{"build"}, files(
		"build-report.md", "build-prompt.txt", "build-events.ndjson", "build-usage.json",
	), phasecontract.BuiltinResolver{})
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
	for _, key := range []string{`"name"`, `"present"`, `"bytes"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("Artifact lost wire key %s: %s", key, raw)
		}
	}
	if empties, err := json.Marshal(Artifact{Name: "x", Present: true, Empty: true}); err != nil || !strings.Contains(string(empties), `"empty"`) {
		t.Errorf("Artifact lost the empty marker: %s err=%v", empties, err)
	}
	if notOwed, err := json.Marshal(Artifact{Name: "x", NotOwed: true}); err != nil || !strings.Contains(string(notOwed), `"not_owed"`) {
		t.Errorf("Artifact lost the not_owed marker — a rows consumer would re-derive native phases as gapped: %s err=%v", notOwed, err)
	}
	if !art.Present || art.Bytes != 1000 {
		t.Errorf("Artifact observation drifted: %+v", art)
	}
}
