package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func TestLaneScopePin_ScoutGoalHashMismatchNormalizesAndProceeds(t *testing.T) {
	root := t.TempDir()
	ws := RunWorkspacePath(root, 1)
	// pinGoalHash / misGoalHash (defined in lanescope_normalize_test.go) mirror
	// a one-digit transcription flip, the shape the machine-stamp guard reconciles.
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)

	runners := buildRunners(nil)
	runners[PhaseScout] = &scoutReportRunner{fakeRunner: fakeRunner{name: string(PhaseScout)}, goalHash: misGoalHash}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: pinGoalHash})
	if err != nil {
		t.Fatalf("a mis-echoed goal_hash must normalize + proceed, not abort: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS (lane reconciled, cycle continues)", res.FinalVerdict)
	}
	if tr := runners[PhaseTriage].(*fakeRunner); tr.calls != 1 {
		t.Errorf("triage calls=%d, want 1 (proceeds on the machine-stamped lane, no false abort)", tr.calls)
	}
	if got := scoutReportGoalHash(ws); got != pinGoalHash {
		t.Errorf("report Decision Trace goal_hash=%q, want the machine-stamped pin %q", got, pinGoalHash)
	}
}

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
