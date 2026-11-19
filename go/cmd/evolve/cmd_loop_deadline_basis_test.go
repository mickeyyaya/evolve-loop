package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func writeDeadlineBasisLane(t *testing.T, root string, cycle int, span time.Duration, shipped bool) {
	t.Helper()
	start := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	verdict := "FAIL"
	if shipped {
		verdict = "PASS"
	}
	events := []signalcenter.Event{
		{Seq: 1, TS: start.Format(time.RFC3339Nano), Cycle: cycle, Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo},
		{Seq: 2, TS: start.Add(span).Format(time.RFC3339Nano), Cycle: cycle, Phase: "ship", Module: signalcenter.ModuleOrchestrator, Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Fields: map[string]string{"verdict": verdict}},
	}
	var lines []string
	for _, e := range events {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	dir := paths.RunWorkspace(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func shippedAndFailedLaneRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for c := 1; c <= 3; c++ {
		writeDeadlineBasisLane(t, root, c, 40*time.Minute, true)
	}
	for c := 4; c <= 5; c++ {
		writeDeadlineBasisLane(t, root, c, 6*time.Hour, false)
	}
	return root
}

func TestNewLaneLeases_TheLogPrintsTheDeadlineOfBothBases(t *testing.T) {
	t.Parallel()
	for _, basis := range []string{"all", "shipped"} {
		oc := enforceOrchestrator
		oc.DeadlineFloorMinutes = 10
		oc.DeadlineBasis = basis
		var stderr bytes.Buffer
		newLaneLeases(shippedAndFailedLaneRoot(t), oc, nil, &stderr)
		for _, want := range []string{"deadline_all=6h0m0s", "deadline_shipped=40m0s"} {
			if !strings.Contains(stderr.String(), want) {
				t.Errorf("basis %s: stderr = %q, want %q", basis, stderr.String(), want)
			}
		}
	}
}

func TestNewLaneLeases_TheDeadlineBasisChoosesTheLeaseDeadline(t *testing.T) {
	t.Parallel()
	cases := map[string]time.Duration{"all": 6 * time.Hour, "shipped": 40 * time.Minute}
	for basis, want := range cases {
		oc := enforceOrchestrator
		oc.DeadlineFloorMinutes = 10
		oc.DeadlineBasis = basis
		var stderr bytes.Buffer
		_, cfg := newLaneLeases(shippedAndFailedLaneRoot(t), oc, nil, &stderr)
		if cfg.Deadline != want {
			t.Errorf("basis %s: deadline = %v, want %v", basis, cfg.Deadline, want)
		}
	}
}

func TestNewLaneLeases_AShippedBasisWithNoShippedLaneTakesTheFloor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for c := 1; c <= 3; c++ {
		writeDeadlineBasisLane(t, root, c, 6*time.Hour, false)
	}
	oc := enforceOrchestrator
	oc.DeadlineBasis = "shipped"
	var stderr bytes.Buffer
	_, cfg := newLaneLeases(root, oc, nil, &stderr)
	if cfg.Deadline != time.Hour {
		t.Errorf("deadline = %v, want the 60-minute floor", cfg.Deadline)
	}
}
