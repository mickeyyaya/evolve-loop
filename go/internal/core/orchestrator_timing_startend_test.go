package core

// RED tests for PR1/A1 (phase-timing start/end wall-clock evidence).
//
// Today phase-timing.json records only a relative duration_ms; the per-phase
// wall-clock anchors (started_at/ended_at) are captured at dispatch
// (cyclerun_dispatch.go: PhaseStartedAt) but thrown away into transient
// cycle-state — never persisted. These white-box tests (package core, reusing
// the orchestrator_test.go harness) pin the contract: EVERY recorded phase
// carries an RFC3339 started_at and ended_at, ended_at is not before started_at,
// and an advancing clock proves ended_at is captured strictly after started_at
// (i.e. the orchestrator brackets the dispatch, it does not stamp one instant
// twice). They reference only already-public symbols, so the core test binary
// still COMPILES at the pre-implementation baseline and fails at RUNTIME (keys
// absent) — the correct RED signal.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// readTimings unmarshals <workspace>/phase-timing.json (local helper — the
// integration-tagged orchestrator_phaseoutcome_test.go has equivalents under a
// build tag this default-suite file cannot see).
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

// timingEntry returns the single entry for phase, failing on absence/duplicates.
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

// advancingClock returns a monotonically increasing clock: each call returns a
// timestamp `step` later than the previous. Mutex-guarded so a concurrent
// observer probe cannot race the dispatch under `go test -race`.
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

// AC-1: a fully-PASS cycle persists started_at + ended_at on every phase entry,
// both RFC3339-parseable, ended_at not before started_at. The per-phase
// presence (not a single cycle-level pair) is the anti-no-op guard.
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

// AC-2: an advancing clock proves ended_at is strictly after started_at — the
// orchestrator captured end AFTER start with real clock progression, not the
// same instant stamped twice.
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

// AC-3: a phase that exhausts its retries and aborts the cycle must STILL carry
// started_at/ended_at (the deferred writer flushes on the abort path too).
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

// A2: every timing entry carries the phase's config-driven archetype
// (plan/build/evaluate/control) so the evidence roll-up can bucket cycle time
// into productive vs checking vs planning vs recovery WITHOUT a hand-maintained
// phase list. Reuses the existing phasespec taxonomy (single source), so the
// classification matches the inventory exactly. RED until recordPhaseOutcome
// stamps the archetype.
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

// AC-4: the <phase>-usage.json sidecar carries the same start/end anchors, so
// the per-phase evidence reads uniformly across both timing surfaces.
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
