package main

// cmd_loop_outputs_signal_test.go — the loop's per-cycle survey emission into
// abnormal-events.jsonl. Driven through the real emitter against a real
// on-disk workspace: the defect class is a signal that diverges from what the
// phases actually wrote, or one that quietly never fires.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

func surveySignalWorkspace(t *testing.T, completed []string, files map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	run, _ := json.Marshal(map[string]any{"completed_phases": completed})
	if err := os.WriteFile(filepath.Join(ws, "run.json"), run, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func readSurveyEvent(t *testing.T, ws string) dispatchevents.Event {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("the survey signal never reached the unified stream: %v", err)
	}
	var e dispatchevents.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestEmitPhaseOutputsSurvey_CompleteCycleIsTheInfoHeartbeat(t *testing.T) {
	t.Parallel()
	ws := surveySignalWorkspace(t, []string{"build"}, map[string]string{
		"build-report.md": "data", "build-prompt.txt": "data",
		"build-events.ndjson": "data", "build-usage.json": "data",
	})
	var stderr strings.Builder
	emitPhaseOutputsSurvey(ws, 9, &stderr)
	e := readSurveyEvent(t, ws)
	if e.EventType != dispatchevents.EventPhaseOutputsSurveyed || e.Cycle != 9 {
		t.Errorf("wrong event on the stream: %+v", e)
	}
	if e.Severity != dispatchevents.SeverityInfo {
		t.Errorf("a complete unaudited cycle is the healthy heartbeat, got %s: %q", e.Severity, e.Details)
	}
	if !strings.Contains(stderr.String(), "1/1 complete") {
		t.Errorf("the loop log must carry the per-cycle summary for live monitoring: %q", stderr.String())
	}
}

func TestEmitPhaseOutputsSurvey_GapsWarnOnTheStream(t *testing.T) {
	t.Parallel()
	// audit completed but left nothing reviewable — the silent-disposal shape.
	ws := surveySignalWorkspace(t, []string{"audit"}, map[string]string{})
	emitPhaseOutputsSurvey(ws, 10, &strings.Builder{})
	e := readSurveyEvent(t, ws)
	if e.Severity != dispatchevents.SeverityWarn {
		t.Errorf("missing review data must WARN the unified stream, got %s: %q", e.Severity, e.Details)
	}
	if !strings.Contains(e.Details, "audit-report.md") {
		t.Errorf("the signal must name what went missing: %q", e.Details)
	}
}

func TestEmitPhaseOutputsSurvey_AbortedCycleSkipsLoudlyNotSilently(t *testing.T) {
	t.Parallel()
	ws := t.TempDir() // no run.json — the aborted-cycle shape
	var stderr strings.Builder
	emitPhaseOutputsSurvey(ws, 11, &stderr)
	if _, err := os.Stat(filepath.Join(ws, "abnormal-events.jsonl")); err == nil {
		t.Error("no run.json means nothing to survey — emitting would fabricate an accounting")
	}
	if !strings.Contains(stderr.String(), "cycle 11") {
		t.Errorf("the skip must land on stderr with the cycle number: %q", stderr.String())
	}
}
