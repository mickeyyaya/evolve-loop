//go:build integration

package core

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Real filesystem/git closeout; runners replace external agents only.
func resumedLifecycleFixture(t *testing.T) (*Orchestrator, *fakeStorage, CycleRequest, *ResumePoint) {
	t.Helper()
	root := t.TempDir()
	initDossierRepo(t, root)
	ws := RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{state: State{LastCycleNumber: 7}, cycleState: CycleState{CycleID: 7, RunID: "original-run", WorkspacePath: ws, CompletedPhases: []string{"scout", "triage", "build"}}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	return o, st, CycleRequest{ProjectRoot: root, GoalHash: "recover-original-task"}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 7}
}

func TestResumeLifecycle_CloseoutRecordsLearningAndRejectsReplay(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	if err := os.WriteFile(filepath.Join(st.cycleState.WorkspacePath, "carryover-todos.json"), []byte(`[{"id":"follow-up","action":"verify recovered behavior"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dossier.CyclesDir(req.ProjectRoot), "cycle-7.json"))
	if err != nil {
		t.Fatalf("resumed terminal cycle has no durable dossier: %v", err)
	}
	d, err := dossier.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Cycle != 7 || d.RunID != "original-run" || d.FinalVerdict != result.FinalVerdict {
		t.Fatalf("closeout lost original identity/outcome: %+v", d)
	}
	if len(st.state.CarryoverTodos) != 1 || st.state.CarryoverTodos[0].ID != "follow-up" {
		t.Fatalf("resumed learning not merged: %+v", st.state.CarryoverTodos)
	}
	calls := len(o.runners[PhaseAudit].(*fakeRunner).requests)
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err == nil {
		t.Error("completed checkpoint must not dispatch again")
	}
	if got := len(o.runners[PhaseAudit].(*fakeRunner).requests); got != calls {
		t.Errorf("replay re-dispatched audit: %d -> %d", calls, got)
	}
	if len(st.state.CarryoverTodos) != 1 {
		t.Fatal("replay duplicated carryover")
	}
}

func TestResumeLifecycle_EmptyTriageStopsBeforeImplementation(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	ws := RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{
		state: State{LastCycleNumber: 7},
		cycleState: CycleState{
			CycleID:         7,
			RunID:           "original-run",
			WorkspacePath:   ws,
			CompletedPhases: []string{"scout"},
			FinalVerdict:    VerdictPASS,
		},
	}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[]}`}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	result, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "protected-task"}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 7})
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if result.FinalVerdict != CycleOutcomeSkippedUnknown {
		t.Errorf("FinalVerdict = %q, want %q", result.FinalVerdict, CycleOutcomeSkippedUnknown)
	}
	if result.TerminationReason != CycleTerminationTriageNoWork {
		t.Errorf("TerminationReason = %q, want %q", result.TerminationReason, CycleTerminationTriageNoWork)
	}
	if got, want := result.PhasesRun, []Phase{PhaseTriage}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("PhasesRun = %v, want %v", got, want)
	}
	for _, phase := range []Phase{PhaseTDD, PhaseBuild, PhaseAudit, PhaseShip} {
		if got := len(runners[phase].(*fakeRunner).requests); got != 0 {
			t.Errorf("%s dispatched %d time(s) after terminal Triage", phase, got)
		}
	}
}

func TestResumeLifecycle_LostLandingUsesSameTerminalFloor(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	if err := os.WriteFile(filepath.Join(st.cycleState.WorkspacePath, "ship-error.json"), []byte(`{"code":"GIT_FLEET_REBASE_NEEDED","class":"transient","message":"peer conflict"}`), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err != nil {
		t.Fatal(err)
	}
	if got.FinalVerdict != VerdictWARN || got.SystemFailure == nil || got.SystemFailure.Category != "landing-lost" {
		t.Fatalf("resume counted a lost landing as success: %+v", got)
	}
}

func TestResumeLifecycle_IterationExhaustionIsExplicitFailure(t *testing.T) {
	o, _, req, rp := resumedLifecycleFixture(t)
	WithMaxPhaseIterations(1)(o)
	got, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err == nil || !strings.Contains(err.Error(), "iteration") || got.FinalVerdict != VerdictFAIL {
		t.Fatalf("safety bound reported success: result=%+v err=%v", got, err)
	}
}

func TestResumeLifecycle_TerminalFailurePreservesContinuation(t *testing.T) {
	root, wt := initContinuationRepo(t, 7)
	ws := RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "feature.go"), []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{state: State{LastCycleNumber: 7}, cycleState: CycleState{CycleID: 7, RunID: "original-run", WorkspacePath: ws, ActiveWorktree: wt, WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD")}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &fakeRunner{name: "build", failErr: errors.New("deterministic build failure"), failUntil: 99}
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "recover"}, &ResumePoint{CycleID: 7, Phase: "build", WorktreePath: wt})
	if err == nil {
		t.Fatal("fixture should fail")
	}
	manifest, ok, err := continuation.ReadManifest(ws)
	if err != nil || !ok || manifest.Cycle != 7 {
		t.Fatalf("resumed failure lost continuation: %+v present=%v err=%v", manifest, ok, err)
	}
	if _, err := os.Stat(filepath.Join(dossier.CyclesDir(root), "cycle-7.json")); err != nil {
		t.Fatalf("resumed failure lost dossier: %v", err)
	}
}

func TestResumeLifecycle_RequestRetainsDifficultyAndRecurrence(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	rp.Phase = "build"
	req.Context = map[string]string{"fleet_scope": "task-a"}
	if err := os.WriteFile(filepath.Join(st.cycleState.WorkspacePath, "triage-report.md"), []byte("cycle_size_estimate: large\n"), 0644); err != nil {
		t.Fatal(err)
	}
	WithFailureCountReader(func(string) int { return 1 })(o)
	o.failurePolicy.Thresholds.BuildDeepEscalateAtFailures = 1
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatal(err)
	}
	got := o.runners[PhaseBuild].(*fakeRunner).requests[0]
	if got.BudgetScale != 1.5 || got.ModelRoutingTier != "deep" {
		t.Fatalf("resume dropped dispatch policy: scale=%v tier=%q", got.BudgetScale, got.ModelRoutingTier)
	}
}

func TestResumeLifecycle_QuotaExhaustionDefersWithoutTerminalDossier(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	o.retryConfig.RetryBackoffBaseS = 0
	o.runners[PhaseAudit] = &fakeRunner{name: "audit", failErr: wrapTransient(85), failUntil: 99}
	result, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("quota pause not typed as resumable: %+v %v", result, err)
	}
	if st.cycleState.Phase == "aborted" || len(st.state.FailedAt) != 0 {
		t.Fatalf("quota pause recorded terminal failure: %+v", st.cycleState)
	}
	if _, err := os.Stat(filepath.Join(dossier.CyclesDir(req.ProjectRoot), "cycle-7.json")); !os.IsNotExist(err) {
		t.Fatalf("quota pause wrote a terminal dossier: %v", err)
	}
}

func TestResumeLifecycle_CloseoutNeverRewindsConcurrentCycleCursor(t *testing.T) {
	st := &fakeUpdaterStorage{}
	st.mem.st = State{LastCycleNumber: 12, LastAllocatedCycleNumber: 13}
	o := &Orchestrator{storage: st}
	if err := o.persistCycleEndState(context.Background(), State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if st.mem.st.LastCycleNumber != 12 || st.mem.st.LastAllocatedCycleNumber != 13 {
		t.Fatalf("old resumed lane rewound newer completion: %+v", st.mem.st)
	}
}

func TestResumeLifecycle_FinalizeAccountingIsIdempotent(t *testing.T) {
	o, st, req, _ := resumedLifecycleFixture(t)
	st.state.TriageThroughput = []TriageThroughputEntry{{Cycle: 7, Floors: 3}}
	o.gitHEAD = func() (string, error) { return "new-head", nil }
	o.throughputRecorder = func(s *State, cycle int, _ string) {
		s.TriageThroughput = append(s.TriageThroughput, TriageThroughputEntry{Cycle: cycle, Floors: 3})
	}
	result := CycleResult{Cycle: 7, FinalVerdict: VerdictPASS}
	if _, err := o.finalizeCycle(context.Background(), st.cycleState, 7, "old-head", req.ProjectRoot, &result, &st.state, nil); err != nil {
		t.Fatal(err)
	}
	if len(st.state.TriageThroughput) != 1 {
		t.Fatalf("replayed closeout counted cycle twice: %+v", st.state.TriageThroughput)
	}
}

func TestResumeLifecycle_PostShipPauseRetainsThroughputBaseline(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	rp.Phase = string(PhaseRetro)
	st.cycleState.CompletedPhases = append(st.cycleState.CompletedPhases, "audit", "ship")
	st.cycleState.FinalVerdict = VerdictPASS // the completed Ship's persisted host disposition
	raw, err := json.Marshal(st.cycleState)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	saved["pre_cycle_head"] = "original-head"
	raw, err = json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &st.cycleState); err != nil {
		t.Fatal(err)
	}
	o.gitHEAD = func() (string, error) { return "already-shipped-head", nil }
	o.throughputRecorder = func(s *State, cycle int, _ string) {
		s.TriageThroughput = append(s.TriageThroughput, TriageThroughputEntry{Cycle: cycle, Floors: 3})
	}
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatal(err)
	}
	if len(st.state.TriageThroughput) != 1 || st.state.TriageThroughput[0].Cycle != 7 {
		t.Fatalf("post-ship resume lost observed throughput: %+v", st.state.TriageThroughput)
	}
}

func TestResumeLifecycle_AuditRoundPropagatesToAuditAndShip(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	st.cycleState.AuditDispatches = 2
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []Phase{PhaseAudit, PhaseShip} {
		raw, _ := json.Marshal(o.runners[phase].(*fakeRunner).requests[0])
		var got map[string]any
		_ = json.Unmarshal(raw, &got)
		if got["audit_round"] != float64(3) {
			t.Errorf("%s request dropped audit round: %v", phase, got["audit_round"])
		}
	}
}

func TestResumeLifecycle_RestoresOriginalGoalAndRejectsReplacement(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore", true: "reject-replacement"}[replacement], func(t *testing.T) {
			o, st, req, rp := resumedLifecycleFixture(t)
			rp.Phase = "build"
			req.GoalHash = ""
			raw, _ := json.Marshal(st.cycleState)
			var state map[string]any
			_ = json.Unmarshal(raw, &state)
			state["goal_hash"], state["goal_text"] = "original-hash", "repair the original task"
			raw, _ = json.Marshal(state)
			if err := json.Unmarshal(raw, &st.cycleState); err != nil {
				t.Fatal(err)
			}
			if replacement {
				req.GoalHash = "replacement-hash"
			}
			_, err := o.RunCycleFromPhase(context.Background(), req, rp)
			if replacement {
				if err == nil {
					t.Fatal("resumed checkpoint accepted a different goal")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := o.runners[PhaseBuild].(*fakeRunner).requests[0]
			if got.GoalHash != "original-hash" || got.Context["goal"] != "repair the original task" {
				t.Fatalf("original task lost on resume: hash=%q goal=%q", got.GoalHash, got.Context["goal"])
			}
		})
	}
}

func TestResumeLifecycle_FreshCyclePersistsOriginalGoal(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	o.gitHEAD = func() (string, error) { return "original-head", nil }
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "original-hash", Context: map[string]string{"goal": "original task"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(st.cycleState)
	var state map[string]any
	_ = json.Unmarshal(raw, &state)
	if state["goal_hash"] != "original-hash" || state["goal_text"] != "original task" || state["pre_cycle_head"] != "original-head" {
		t.Fatalf("fresh cycle has no durable goal: %v", state)
	}
}

func TestResumeLifecycle_RecurrenceUsesPersistedLaneScope(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	rp.Phase = "build"
	materializeLaneScope(st.cycleState.WorkspacePath, "task-a", req.GoalHash)
	WithFailureCountReader(func(id string) int {
		if id == "task-a" {
			return 1
		}
		return 0
	})(o)
	o.failurePolicy.Thresholds.BuildDeepEscalateAtFailures = 1
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatal(err)
	}
	got := o.runners[PhaseBuild].(*fakeRunner).requests[0]
	if got.ModelRoutingTier != "deep" || got.Context["fleet_scope"] != "task-a" {
		t.Fatalf("persisted task scope lost: tier=%q scope=%q", got.ModelRoutingTier, got.Context["fleet_scope"])
	}
}

func TestResumeLifecycle_FreshQuotaPauseIsNotTerminalFailure(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(85), failUntil: 99}
	st := &fakeStorage{}
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	o.retryConfig.RetryBackoffBaseS = 0
	got, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "original"})
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("fixture must pause on quota: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dossier.CyclesDir(root), "cycle-1.json")); !os.IsNotExist(err) {
		t.Fatalf("resource pause incorrectly became a terminal dossier: %v", err)
	}
	if st.cycleState.Phase == "aborted" || len(got.SkippedPhases) != 0 {
		t.Fatalf("resource pause sealed as failed: %+v %+v", st.cycleState, got)
	}
}

func TestResumeLifecycle_PausedAllocatedCycleUsesHostLeaseIdentity(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	st.state.LastCycleNumber = 6
	st.state.LastAllocatedCycleNumber = 7
	got, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err != nil {
		t.Fatalf("host allocated cycle could not resume after quota pause: %v", err)
	}
	if got.Cycle != 7 || st.state.LastCycleNumber != 7 || st.state.LastAllocatedCycleNumber != 7 {
		t.Fatalf("resume changed allocated identity: %+v %+v", got, st.state)
	}
}
