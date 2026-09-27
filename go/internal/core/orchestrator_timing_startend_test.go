package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// readTimings unmarshals <workspace>/phase-timing.json. Duplicated here
// (rather than reused) because orchestrator_phaseoutcome_test.go's equivalent
// sits behind a build tag this default-suite file cannot see.
func readTimings(t *testing.T, workspace string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workspace, "phase-timing.json"))
	if err != nil {
		t.Fatalf("phase-timing.json must exist: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("phase-timing.json must be a JSON array: %v\n%s", err, data)
	}
	return entries
}

func timingEntry(t *testing.T, entries []map[string]any, phase string) map[string]any {
	t.Helper()
	var found []map[string]any
	for _, e := range entries {
		if e["phase"] == phase {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly 1 timing entry for %q, got %d: %v", phase, len(found), entries)
	}
	return found[0]
}

// advancingClock returns a monotonically increasing clock. Mutex-guarded so a
// concurrent observer probe cannot race the dispatch under `go test -race`.
func advancingClock(start time.Time, step time.Duration) func() time.Time {
	var mu sync.Mutex
	var n int64
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		t := start.Add(time.Duration(n) * step)
		n++
		return t
	}
}

func TestPhaseTiming_StartEndPopulated_HappyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	workspace := cycleWorkspaceDir(root, res.Cycle)
	entries := readTimings(t, workspace)
	if len(entries) == 0 {
		t.Fatalf("no timing entries written")
	}
	for _, e := range entries {
		phase, _ := e["phase"].(string)
		start, _ := e["started_at"].(string)
		end, _ := e["ended_at"].(string)
		if start == "" {
			t.Errorf("phase %q: started_at missing/empty (entry=%v)", phase, e)
			continue
		}
		if end == "" {
			t.Errorf("phase %q: ended_at missing/empty (entry=%v)", phase, e)
			continue
		}
		ts, perr := time.Parse(time.RFC3339, start)
		if perr != nil {
			t.Errorf("phase %q: started_at %q not RFC3339: %v", phase, start, perr)
		}
		te, perr := time.Parse(time.RFC3339, end)
		if perr != nil {
			t.Errorf("phase %q: ended_at %q not RFC3339: %v", phase, end, perr)
		}
		if te.Before(ts) {
			t.Errorf("phase %q: ended_at %q is before started_at %q", phase, end, start)
		}
	}
}

func TestPhaseTiming_EndStrictlyAfterStart_AdvancingClock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	o.now = advancingClock(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC), time.Second)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	entries := readTimings(t, cycleWorkspaceDir(root, res.Cycle))
	scout := timingEntry(t, entries, "scout")
	start, _ := scout["started_at"].(string)
	end, _ := scout["ended_at"].(string)
	ts, _ := time.Parse(time.RFC3339, start)
	te, _ := time.Parse(time.RFC3339, end)
	if !te.After(ts) {
		t.Errorf("scout: ended_at %q must be strictly after started_at %q under an advancing clock", end, start)
	}
}

func TestPhaseTiming_StartEndOnAbort(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTimeout(), failUntil: 99}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root})
	if err == nil {
		t.Fatalf("RunCycle should abort after exhausting retries")
	}
	scout := timingEntry(t, readTimings(t, cycleWorkspaceDir(root, res.Cycle)), "scout")
	if s, _ := scout["started_at"].(string); s == "" {
		t.Errorf("aborted scout entry must carry started_at; got %v", scout)
	}
	if e, _ := scout["ended_at"].(string); e == "" {
		t.Errorf("aborted scout entry must carry ended_at; got %v", scout)
	}
}

func TestPhaseTiming_ArchetypeClassified(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	entries := readTimings(t, cycleWorkspaceDir(root, res.Cycle))
	// Spot-check one phase per archetype against the canonical taxonomy.
	want := map[string]string{
		"scout": "plan", "build": "build", "audit": "evaluate", "ship": "control",
	}
	for phase, archetype := range want {
		e := timingEntry(t, entries, phase)
		if got, _ := e["archetype"].(string); got != archetype {
			t.Errorf("phase %q archetype=%q, want %q (entry=%v)", phase, got, archetype, e)
		}
	}
}

func TestPhaseTiming_UsageSidecarCarriesStartEnd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	data, rerr := os.ReadFile(filepath.Join(cycleWorkspaceDir(root, res.Cycle), "scout-usage.json"))
	if rerr != nil {
		t.Fatalf("scout-usage.json must exist: %v", rerr)
	}
	var sc map[string]any
	if err := json.Unmarshal(data, &sc); err != nil {
		t.Fatalf("scout-usage.json must be valid JSON: %v\n%s", err, data)
	}
	if s, _ := sc["started_at"].(string); s == "" {
		t.Errorf("scout-usage.json must carry started_at; got %s", data)
	}
	if e, _ := sc["ended_at"].(string); e == "" {
		t.Errorf("scout-usage.json must carry ended_at; got %s", data)
	}
}
