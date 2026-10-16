package dispatchevents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readEmittedEvent(t *testing.T, workspace string) Event {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspace, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var e Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestEventPhaseOutputsSurveyed_IsInTheClosedVocabulary(t *testing.T) {
	t.Parallel()
	if !IsKnownEventType(EventPhaseOutputsSurveyed) {
		t.Error("EventPhaseOutputsSurveyed declared but not in the vocabulary — typed readers would skip its records silently")
	}
	if EventPhaseOutputsSurveyed != "phase-outputs-surveyed" {
		t.Errorf("wire value = %q — consumed by operator jq filters and the dashboard; must not change", EventPhaseOutputsSurveyed)
	}
}

func TestEmitPhaseOutputsSurveyed_HealthyIsInfoAbnormalIsWarn(t *testing.T) {
	t.Parallel()

	healthyWS := t.TempDir()
	if err := NewWriter(healthyWS).EmitPhaseOutputsSurveyed(7, "phase outputs: 5/5 complete; chain: chain-present", false); err != nil {
		t.Fatal(err)
	}
	healthy := readEmittedEvent(t, healthyWS)
	if healthy.Severity != SeverityInfo || healthy.EventType != EventPhaseOutputsSurveyed || healthy.Cycle != 7 {
		t.Errorf("healthy survey must be the INFO heartbeat: %+v", healthy)
	}
	if healthy.SourcePhase != "phase-outputs" {
		t.Errorf("source_phase must identify the sub-system, got %q", healthy.SourcePhase)
	}

	abnormalWS := t.TempDir()
	if err := NewWriter(abnormalWS).EmitPhaseOutputsSurveyed(8, "phase outputs: 4/5 complete — gaps: audit: audit-report.md missing; chain: record-missing", true); err != nil {
		t.Fatal(err)
	}
	abnormal := readEmittedEvent(t, abnormalWS)
	if abnormal.Severity != SeverityWarn {
		t.Errorf("an abnormal survey at INFO hides the exact silence the survey exists to end: %+v", abnormal)
	}
	if !strings.Contains(abnormal.RemediationHint, "evolve cycle outputs 8") {
		t.Errorf("the WARN must point the operator at the per-phase rows: %q", abnormal.RemediationHint)
	}
}
