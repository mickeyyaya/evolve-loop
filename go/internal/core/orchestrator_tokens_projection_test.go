package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func TestRecordPhaseOutcome_ProjectsTokensToSidecarAndTiming(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	o := NewOrchestrator(nil, nil, nil)

	var result CycleResult
	var timings []phaseTimingEntry
	o.recordPhaseOutcome(&result, &timings, ws, recovery.PhaseOutcome{
		Phase:        "build",
		Verdict:      "PASS",
		CostUSD:      0.42,
		DurationMS:   1000,
		AttemptCount: 1,
		StartedAt:    "2026-07-07T00:00:00Z",
		Tokens:       cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 90, CacheWrite: 7},
	})

	if len(timings) != 1 {
		t.Fatalf("want 1 timing entry, got %d", len(timings))
	}
	got := timings[0].Tokens
	want := cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 90, CacheWrite: 7}
	if got != want {
		t.Errorf("timing entry tokens = %+v, want %+v", got, want)
	}

	data, err := os.ReadFile(filepath.Join(ws, "build-usage.json"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var sidecar outcome.UsageSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	if sidecar.Tokens != want {
		t.Errorf("sidecar tokens = %+v, want %+v", sidecar.Tokens, want)
	}
}

func TestSidecar_LegacyWithoutTokensParses(t *testing.T) {
	t.Parallel()
	legacy := `{"phase":"scout","cost_usd":0.1,"duration_ms":500,"attempt_count":1,"verdict":"PASS"}`
	var sidecar outcome.UsageSidecar
	if err := json.Unmarshal([]byte(legacy), &sidecar); err != nil {
		t.Fatalf("legacy sidecar must parse: %v", err)
	}
	if sidecar.Phase != "scout" {
		t.Errorf("phase = %q, want scout", sidecar.Phase)
	}
	if (sidecar.Tokens != cyclestate.TokenUsage{}) {
		t.Errorf("legacy sidecar tokens = %+v, want zero value", sidecar.Tokens)
	}
}
