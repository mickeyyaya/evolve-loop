package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type laneOutcome struct {
	phase   string
	verdict string
	at      time.Duration
}

func writeOutcomeCycle(t *testing.T, runs string, cycle int, outcomes ...laneOutcome) {
	t.Helper()
	dir := filepath.Join(runs, "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	events := []signalcenter.Event{{Seq: 1, TS: epoch.Format(time.RFC3339Nano), Cycle: cycle, Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo}}
	for i, o := range outcomes {
		events = append(events, signalcenter.Event{
			Seq: uint64(i + 2), TS: epoch.Add(o.at).Format(time.RFC3339Nano), Cycle: cycle, Phase: o.phase,
			Module: signalcenter.ModuleOrchestrator, Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo,
			Fields: map[string]string{"verdict": o.verdict},
		})
	}
	var lines []string
	for _, e := range events {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLaneRuns_OnlyAPassingShipOutcomeMarksALaneShipped(t *testing.T) {
	runs := t.TempDir()
	writeOutcomeCycle(t, runs, 1, laneOutcome{"ship", "PASS", 40 * time.Minute})
	writeOutcomeCycle(t, runs, 2, laneOutcome{"ship", "FAIL", 20 * time.Minute}, laneOutcome{"ship", "PASS", 55 * time.Minute})
	writeOutcomeCycle(t, runs, 3, laneOutcome{"ship", "FAIL", 6 * time.Hour})
	writeOutcomeCycle(t, runs, 4, laneOutcome{"audit", "PASS", 3 * time.Hour})
	writeOutcomeCycle(t, runs, 5, laneOutcome{"ship", "PASS", 12 * time.Minute})

	got, err := LaneRuns(runs, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := []LaneRun{
		{Elapsed: 40 * time.Minute, Shipped: true},
		{Elapsed: 55 * time.Minute, Shipped: true},
		{Elapsed: 6 * time.Hour, Shipped: false},
		{Elapsed: 3 * time.Hour, Shipped: false},
		{Elapsed: 12 * time.Minute, Shipped: true},
	}
	if len(got) != len(want) {
		t.Fatalf("LaneRuns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lane %d = %+v, want %+v", i+1, got[i], want[i])
		}
	}
}

func TestLaneRuns_TakesOnlyTheLastCycles(t *testing.T) {
	runs := t.TempDir()
	for c := 1; c <= 4; c++ {
		writeOutcomeCycle(t, runs, c, laneOutcome{"ship", "PASS", time.Duration(c) * time.Minute})
	}
	got, err := LaneRuns(runs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Elapsed != 3*time.Minute || got[1].Elapsed != 4*time.Minute {
		t.Errorf("LaneRuns(last 2) = %+v, want cycles 3 and 4", got)
	}
}

func TestLaneRuns_ANoRunsDirectoryIsAnError(t *testing.T) {
	if _, err := LaneRuns(filepath.Join(t.TempDir(), "missing"), 50); err == nil {
		t.Error("LaneRuns of a missing directory returned no error")
	}
}

func TestDeadlineByBasis(t *testing.T) {
	shipped := func(n int, d time.Duration) []LaneRun {
		out := make([]LaneRun, n)
		for i := range out {
			out[i] = LaneRun{Elapsed: d, Shipped: true}
		}
		return out
	}
	failed := func(n int, d time.Duration) []LaneRun {
		out := make([]LaneRun, n)
		for i := range out {
			out[i] = LaneRun{Elapsed: d}
		}
		return out
	}
	mixed := append(shipped(3, 40*time.Minute), failed(2, 6*time.Hour)...)
	cases := []struct {
		name  string
		runs  []LaneRun
		basis string
		floor time.Duration
		want  time.Duration
	}{
		{"shipped ignores the failed lanes", mixed, "shipped", 10 * time.Minute, 40 * time.Minute},
		{"all counts the failed lanes", mixed, "all", 10 * time.Minute, 6 * time.Hour},
		{"shipped with no shipped lane takes the floor", failed(4, 6*time.Hour), "shipped", time.Hour, time.Hour},
		{"all with no history takes the floor", nil, "all", time.Hour, time.Hour},
		{"shipped with no history takes the floor", nil, "shipped", time.Hour, time.Hour},
		{"a shipped p95 under the floor takes the floor", shipped(3, 20*time.Minute), "shipped", time.Hour, time.Hour},
		{"a failed lane never lifts the shipped deadline", append(shipped(3, 40*time.Minute), failed(1, 48*time.Hour)...), "shipped", 10 * time.Minute, 40 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeadlineByBasis(tc.runs, tc.basis, tc.floor); got != tc.want {
				t.Errorf("DeadlineByBasis(%s) = %v, want %v", tc.basis, got, tc.want)
			}
		})
	}
}

func TestDeadlineByBasis_AllEqualsTheLegacyDeadlineOverEveryLaneTime(t *testing.T) {
	runs := []LaneRun{{Elapsed: 5 * time.Minute, Shipped: true}, {Elapsed: 90 * time.Minute}, {Elapsed: 30 * time.Minute, Shipped: true}, {Elapsed: 2 * time.Hour}}
	var times []time.Duration
	for _, r := range runs {
		times = append(times, r.Elapsed)
	}
	if got, want := DeadlineByBasis(runs, "all", 10*time.Minute), DeadlineOf(times, 10*time.Minute); got != want {
		t.Errorf("DeadlineByBasis(all) = %v, want the legacy %v", got, want)
	}
}

func TestDeadlineByBasis_DoesNotReorderItsInput(t *testing.T) {
	runs := []LaneRun{{Elapsed: 3, Shipped: true}, {Elapsed: 1, Shipped: true}, {Elapsed: 2}}
	DeadlineByBasis(runs, "shipped", 0)
	if runs[0].Elapsed != 3 || runs[1].Elapsed != 1 || runs[2].Elapsed != 2 {
		t.Errorf("input = %v, want it unchanged", runs)
	}
}
