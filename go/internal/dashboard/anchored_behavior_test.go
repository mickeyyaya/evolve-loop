package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

var windowT0 = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

func hourWindow() phaseCallWindow {
	return phaseCallWindow{start: windowT0, end: windowT0.Add(time.Hour)}
}

func TestCallForPhaseWindow_UsedCallNotRematched(t *testing.T) {
	calls := []llmCall{{TS: windowT0.Add(time.Minute).Format(time.RFC3339)}}
	if _, _, ok := callForPhaseWindow(hourWindow(), calls, 1, map[int]struct{}{0: {}}); ok {
		t.Fatal("used call matched again")
	}
}

func TestCallForPhaseWindow_CallAfterWindowNotMatched(t *testing.T) {
	calls := []llmCall{{TS: windowT0.Add(2 * time.Hour).Format(time.RFC3339)}}
	if _, _, ok := callForPhaseWindow(hourWindow(), calls, 1, map[int]struct{}{}); ok {
		t.Fatal("call after window matched")
	}
}

func TestCallForPhaseWindow_CallStartedBeforeWindowNotMatched(t *testing.T) {
	calls := []llmCall{{StartedAt: windowT0.Add(-time.Minute).Format(time.RFC3339), EndedAt: windowT0.Add(time.Minute).Format(time.RFC3339)}}
	if _, _, ok := callForPhaseWindow(hourWindow(), calls, 1, map[int]struct{}{}); ok {
		t.Fatal("call started before window matched")
	}
}

func TestCallForPhaseWindow_OccurrenceFallback(t *testing.T) {
	calls := []llmCall{{CLI: "a"}, {CLI: "b"}}
	if c, i, ok := callForPhaseWindow(phaseCallWindow{}, calls, 1, map[int]struct{}{}); !ok || i != 0 || c.CLI != "a" {
		t.Fatalf("occurrence 1 = %+v %d %v", c, i, ok)
	}
	if _, _, ok := callForPhaseWindow(phaseCallWindow{}, calls, 1, map[int]struct{}{0: {}}); ok {
		t.Fatal("used occurrence reused")
	}
}

func TestCollect_OwnLaneStateIsTheLoop(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	writeCycleState(t, root, cyclestate.CycleState{CycleID: 5, Phase: "scout"})
	seedFleetLane(t, root, 5, "build", now)
	s := Collect(root, now)
	if s.Loop.CycleID != 5 || s.Loop.Phase != "build" {
		t.Fatalf("loop = %+v", s.Loop)
	}
}

func TestCollect_NewestRunningLaneIsTheLoop(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	seedFleetLane(t, root, 3, "build", now)
	seedFleetLane(t, root, 7, "audit", now)
	if s := Collect(root, now); s.Loop.CycleID != 7 {
		t.Fatalf("loop = %+v", s.Loop)
	}
}

func TestCollect_BrakeReachesStatelessRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, paths.LoopStopPath(filepath.Join(root, ".evolve")), "")
	if err := os.MkdirAll(core.RunWorkspacePath(root, 9), 0o755); err != nil {
		t.Fatal(err)
	}
	s := Collect(root, time.Now())
	if len(s.Cycles) != 1 || s.Cycles[0].StateName != "paused (brake)" {
		t.Fatalf("cycles = %+v", s.Cycles)
	}
}

func TestCollect_FleetLoopIsEnriched(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	seedFleetLane(t, root, 5, "build", now)
	writeNDJSON(t, filepath.Join(core.RunWorkspacePath(root, 5), "llm-calls.ndjson"),
		`{"ts":"2026-09-09T11:50:00Z","agent":"build","phase":"build","cli":"codex-tmux","model":"deep","attempt":1}`)
	if s := Collect(root, now); s.Loop.CLI != "codex-tmux" || s.Loop.Model != "deep" {
		t.Fatalf("loop = %+v", s.Loop)
	}
}

func TestReadPlan_PlanWarningsCarryCyclePrefix(t *testing.T) {
	root := t.TempDir()
	ws := writePlanFixture(t, root, 1679, []phasetiming.Entry{entry("scout", "PASS", "2026-09-14T08:15:00Z", "2026-09-14T08:18:00Z", 1)}, nil, "")
	writeFile(t, filepath.Join(ws, "phase-replan.json"), `[{"phase":"bug-repro`)
	_, warns := planFor(t, root, 1679, LoopStatus{})
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "cycle 1679 ") {
		t.Fatalf("warns = %v", warns)
	}
}

func TestReadPlan_UnorderedMandatoryPhaseIsUnreachedNotSkipped(t *testing.T) {
	set := mandatorySet{mandatory: []string{"scout", "build", "memo"}, conditional: map[string]bool{}, order: []string{"scout", "build", "audit"}}
	cs := CycleSummary{ID: 9, HasWorkspace: true, State: StatePass, Phases: []PhaseRun{{Phase: "build", Verdict: "PASS"}}}
	plan, _ := readPlan(set, t.TempDir(), cs, LoopStatus{}, newStreamReader())
	got := statuses(plan)
	if strings.Join(got, ",") != "build:pass,scout:skipped,memo:unreached" {
		t.Fatalf("statuses = %v", got)
	}
}
