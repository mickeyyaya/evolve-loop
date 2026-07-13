package core

// Cycle-766 RED contract — fleet-lane-provisioning-split (inbox id
// fleet-lane-provisioning-split, cycle-640 incident: scout scouted lane A's
// goal while triage was handed lane B's fleet_scope, so the run had no
// coherent lane identity).
//
// Contract encoded here (Builder implements, must NOT modify these tests):
//
//  1. PIN: when `evolve fleet` provides EVOLVE_FLEET_SCOPE, the orchestrator
//     materializes <workspace>/lane-scope.json ({"todo_ids":[...],
//     "goal_hash":"..."}) BEFORE any phase runs.
//  2. INJECT: when <workspace>/lane-scope.json exists (supervisor- or
//     orchestrator-written), Context["fleet_scope"] handed to every phase is
//     derived from THAT file (comma-joined todo_ids) — authoritative over the
//     env snapshot, so cross-lane env drift can no longer split lane identity.
//     Absent file ⇒ legacy env fallback (sequential loop byte-identical).
//  3. COHERENCE: after scout completes and before triage runs, a scout-report
//     whose Decision Trace goal_hash differs from lane-scope.json's goal_hash
//     aborts the cycle with an explicit "lane-scope goal-hash mismatch" error
//     (no silent proceed). Missing report / missing goal_hash key / missing
//     lane-scope.json stay fail-open — a guard that false-aborts healthy
//     sequential cycles would recreate the cycle-760..762 destruction class.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// writeLaneScopeFixture writes <workspace>/lane-scope.json the way the fleet
// supervisor is contracted to (raw JSON on purpose — the fixture must pin the
// on-disk schema, not whatever helper the implementation ends up with).
func writeLaneScopeFixture(t *testing.T, workspace string, todoIDs []string, goalHash string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	b, err := json.Marshal(map[string]any{"todo_ids": todoIDs, "goal_hash": goalHash})
	if err != nil {
		t.Fatalf("marshal lane scope: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "lane-scope.json"), b, 0o644); err != nil {
		t.Fatalf("write lane-scope.json: %v", err)
	}
}

// scoutReportRunner is a scout fake that writes a scout-report.md carrying a
// Decision Trace goal_hash — the artifact the coherence gate must parse.
type scoutReportRunner struct {
	fakeRunner
	goalHash string // "" ⇒ write a report WITHOUT a goal_hash key
}

func (s *scoutReportRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return PhaseResponse{}, err
	}
	trace := `{"mode": "incremental"}`
	if s.goalHash != "" {
		trace = fmt.Sprintf(`{"mode": "incremental", "goal_hash": %q}`, s.goalHash)
	}
	report := "# Scout Report — test\n\n## Decision Trace\n\n```json\n" + trace + "\n```\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "scout-report.md"), []byte(report), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return s.fakeRunner.Run(ctx, req)
}

func phaseScopeSeen(t *testing.T, runners map[Phase]PhaseRunner, p Phase) string {
	t.Helper()
	fr, ok := runners[p].(*fakeRunner)
	if !ok {
		t.Fatalf("runner for %s is not *fakeRunner", p)
	}
	if len(fr.requests) == 0 {
		t.Fatalf("phase %s never ran", p)
	}
	return fr.requests[0].Context["fleet_scope"]
}

// INJECT: a supervisor-provisioned lane-scope.json is the fleet_scope source
// for every phase, with no EVOLVE_FLEET_SCOPE env needed at all.
func TestLaneScopePin_FileInjectsFleetScopeToPhases(t *testing.T) {
	root := t.TempDir()
	writeLaneScopeFixture(t, RunWorkspacePath(root, 1), []string{"todo-a", "todo-b"}, "goal-1")

	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal-1"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	for _, p := range []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuild} {
		if got := phaseScopeSeen(t, runners, p); got != "todo-a,todo-b" {
			t.Errorf("phase %s: Context[fleet_scope]=%q, want %q (from lane-scope.json)", p, got, "todo-a,todo-b")
		}
	}
}

// INJECT/negative: two lanes with distinct lane-scope.json each see ONLY their
// own scope even when the env snapshot carries the OTHER lane's scope — the
// exact cycle-640 cross-lane drift. lane-scope.json must win over env.
func TestLaneScopePin_TwoLanesSeeOnlyOwnScope(t *testing.T) {
	root := t.TempDir()
	lanes := []struct {
		lastCycle int
		cycle     int
		ownScope  string
		ownGoal   string
		driftEnv  string // the OTHER lane's scope, leaked via env
	}{
		{lastCycle: 10, cycle: 11, ownScope: "todo-lane-a", ownGoal: "goal-a", driftEnv: "todo-lane-b"},
		{lastCycle: 20, cycle: 21, ownScope: "todo-lane-b", ownGoal: "goal-b", driftEnv: "todo-lane-a"},
	}
	for _, lane := range lanes {
		writeLaneScopeFixture(t, RunWorkspacePath(root, lane.cycle), []string{lane.ownScope}, lane.ownGoal)
		runners := buildRunners(nil)
		o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: lane.lastCycle}}, &fakeLedger{}, runners)
		_, err := o.RunCycle(context.Background(), CycleRequest{
			ProjectRoot: root,
			GoalHash:    lane.ownGoal,
			Env:         map[string]string{ipcenv.FleetScopeKey: lane.driftEnv},
		})
		if err != nil {
			t.Fatalf("lane cycle %d RunCycle: %v", lane.cycle, err)
		}
		for _, p := range []Phase{PhaseScout, PhaseTriage, PhaseBuild} {
			if got := phaseScopeSeen(t, runners, p); got != lane.ownScope {
				t.Errorf("lane cycle %d phase %s: Context[fleet_scope]=%q, want %q (lane-scope.json must beat env drift %q)",
					lane.cycle, p, got, lane.ownScope, lane.driftEnv)
			}
		}
	}
}

// Legacy edge: no lane-scope.json ⇒ the env snapshot still feeds fleet_scope
// (pre-existing behavior, cyclerun.go env→ctx bridge; regression guard so the
// pin cannot break sequential / older-supervisor runs).
func TestLaneScopePin_AbsentFileFallsBackToEnv(t *testing.T) {
	root := t.TempDir()
	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "g",
		Env:         map[string]string{ipcenv.FleetScopeKey: "todo-x,todo-y"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := phaseScopeSeen(t, runners, PhaseTriage); got != "todo-x,todo-y" {
		t.Errorf("Context[fleet_scope]=%q, want env fallback %q", got, "todo-x,todo-y")
	}
}

// PIN: an env-scoped run materializes lane-scope.json into the run workspace
// so the lane identity is on disk before any phase output exists.
func TestLaneScopePin_MaterializedFromEnvBeforePhases(t *testing.T) {
	root := t.TempDir()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root,
		GoalHash:    "goal-pin",
		Env:         map[string]string{ipcenv.FleetScopeKey: "todo-a,todo-b"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(RunWorkspacePath(root, 1), "lane-scope.json"))
	if err != nil {
		t.Fatalf("lane-scope.json not materialized: %v", err)
	}
	var got struct {
		TodoIDs  []string `json:"todo_ids"`
		GoalHash string   `json:"goal_hash"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("lane-scope.json malformed: %v\n%s", err, b)
	}
	if len(got.TodoIDs) != 2 || got.TodoIDs[0] != "todo-a" || got.TodoIDs[1] != "todo-b" {
		t.Errorf("todo_ids=%v, want [todo-a todo-b]", got.TodoIDs)
	}
	if got.GoalHash != "goal-pin" {
		t.Errorf("goal_hash=%q, want goal-pin", got.GoalHash)
	}
}

// COHERENCE: scout-report goal_hash ≠ lane-scope.json goal_hash must abort the
// cycle at the scout→triage transition with an explicit reason — triage must
// never run on an incoherent lane identity.
func TestLaneScopePin_ScoutGoalHashMismatchAbortsBeforeTriage(t *testing.T) {
	root := t.TempDir()
	writeLaneScopeFixture(t, RunWorkspacePath(root, 1), []string{"todo-a"}, "goal-1")

	runners := buildRunners(nil)
	runners[PhaseScout] = &scoutReportRunner{fakeRunner: fakeRunner{name: string(PhaseScout)}, goalHash: "goal-OTHER-lane"}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal-1"})
	if err == nil {
		t.Fatal("RunCycle succeeded; want abort on lane-scope goal-hash mismatch")
	}
	if !strings.Contains(err.Error(), "lane-scope goal-hash mismatch") {
		t.Errorf("abort reason %q does not name the lane-scope goal-hash mismatch", err.Error())
	}
	if tr := runners[PhaseTriage].(*fakeRunner); tr.calls != 0 {
		t.Errorf("triage ran %d time(s) after an incoherent scout; want 0 (fail fast, no silent proceed)", tr.calls)
	}
}

// COHERENCE/negative: a MATCHING goal hash must proceed normally — the gate
// must not turn into a blanket abort.
func TestLaneScopePin_ScoutGoalHashMatchProceeds(t *testing.T) {
	root := t.TempDir()
	writeLaneScopeFixture(t, RunWorkspacePath(root, 1), []string{"todo-a"}, "goal-1")

	runners := buildRunners(nil)
	runners[PhaseScout] = &scoutReportRunner{fakeRunner: fakeRunner{name: string(PhaseScout)}, goalHash: "goal-1"}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal-1"})
	if err != nil {
		t.Fatalf("RunCycle aborted on a MATCHING goal hash: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	if tr := runners[PhaseTriage].(*fakeRunner); tr.calls != 1 {
		t.Errorf("triage calls=%d, want 1", tr.calls)
	}
}

// COHERENCE/fail-open edge: a scout-report with NO goal_hash key (legacy or
// malformed Decision Trace) must proceed — an over-strict gate that destroys
// healthy cycles would recreate the cycle-760..762 abort class.
func TestLaneScopePin_ScoutReportWithoutGoalHashProceeds(t *testing.T) {
	root := t.TempDir()
	writeLaneScopeFixture(t, RunWorkspacePath(root, 1), []string{"todo-a"}, "goal-1")

	runners := buildRunners(nil)
	runners[PhaseScout] = &scoutReportRunner{fakeRunner: fakeRunner{name: string(PhaseScout)}} // no goal_hash key
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal-1"})
	if err != nil {
		t.Fatalf("RunCycle aborted on a goal_hash-less scout report (must fail open): %v", err)
	}
	if tr := runners[PhaseTriage].(*fakeRunner); tr.calls != 1 {
		t.Errorf("triage calls=%d, want 1", tr.calls)
	}
}
