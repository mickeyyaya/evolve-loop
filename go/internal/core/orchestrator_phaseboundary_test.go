package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// seedCycleStateFile seeds a pre-existing cycle-state.json so the additive
// checkpoint splice has a file to merge into and preserved fields can be checked.
func seedCycleStateFile(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	path := filepath.Join(dir, "cycle-state.json")
	if err := os.WriteFile(path, []byte(`{"cycle_id":1,"phase":"scout","custom_field":"preserve-me"}`), 0o644); err != nil {
		t.Fatalf("seed cycle-state.json: %v", err)
	}
	return path
}

func readCheckpointBlock(t *testing.T, path string) (map[string]any, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("parse %s: %v\nbody: %s", path, err, raw)
	}
	cp, _ := state["checkpoint"].(map[string]any)
	return cp, state
}

func completedPhasesOf(cp map[string]any) []string {
	raw, _ := cp["completedPhases"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// checkpointProbeRunner runs as the TRIAGE phase and snapshots the on-disk
// checkpoint block after scout completes but before anything later, pinning
// "written at the phase BOUNDARY" (an end-of-cycle-only write can't satisfy this).
type checkpointProbeRunner struct {
	name      string
	statePath string
	sawBlock  map[string]any
}

func (p *checkpointProbeRunner) Name() string { return p.name }
func (p *checkpointProbeRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	raw, err := os.ReadFile(p.statePath)
	if err == nil {
		var state map[string]any
		if json.Unmarshal(raw, &state) == nil {
			p.sawBlock, _ = state["checkpoint"].(map[string]any)
		}
	}
	return core.PhaseResponse{Phase: p.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestOrchestrator_PhaseBoundaryCheckpoint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	statePath := seedCycleStateFile(t, root)

	probe := &checkpointProbeRunner{name: "triage", statePath: statePath}
	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: probe,
	}))
	_, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if probe.sawBlock == nil {
		t.Fatal("no checkpoint block on disk when triage ran — phase boundary after scout did not write one")
	}
	if got := probe.sawBlock["reason"]; got != "phase-complete" {
		t.Errorf("mid-cycle checkpoint reason = %v, want \"phase-complete\"", got)
	}
	if enabled, _ := probe.sawBlock["enabled"].(bool); !enabled {
		t.Error("mid-cycle checkpoint enabled = false, want true")
	}
	midPhases := completedPhasesOf(probe.sawBlock)
	if !containsStr(midPhases, "scout") {
		t.Errorf("mid-cycle completedPhases = %v, want to include \"scout\" (the just-completed phase)", midPhases)
	}

	cp, state := readCheckpointBlock(t, statePath)
	if cp == nil {
		t.Fatal("no checkpoint block in cycle-state.json after the cycle")
	}
	if got := cp["reason"]; got != "phase-complete" {
		t.Errorf("final checkpoint reason = %v, want \"phase-complete\"", got)
	}
	finalPhases := completedPhasesOf(cp)
	if !containsStr(finalPhases, "ship") {
		t.Errorf("final completedPhases = %v, want to include \"ship\"", finalPhases)
	}

	if got := state["custom_field"]; got != "preserve-me" {
		t.Errorf("custom_field = %v, want \"preserve-me\" (checkpoint write must be additive, not clobbering)", got)
	}
}

type alwaysErrRunner struct{ name string }

func (r *alwaysErrRunner) Name() string { return r.name }
func (r *alwaysErrRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, errors.New("synthetic hard failure in " + r.name)
}

type recordingRetroRunner struct {
	name     string
	calls    int
	requests []core.PhaseRequest
}

func (r *recordingRetroRunner) Name() string { return r.name }
func (r *recordingRetroRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	r.requests = append(r.requests, req)
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestOrchestrator_FailedPhase_NoSuccessCheckpoint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	statePath := seedCycleStateFile(t, root)

	retro := &recordingRetroRunner{name: "retro"}
	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  retro,
	}))
	_, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	if err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	cp, _ := readCheckpointBlock(t, statePath)
	if cp == nil {
		t.Fatal("scout completed before the crash — its boundary checkpoint must exist (this is what --resume consumes)")
	}
	phases := completedPhasesOf(cp)
	if !containsStr(phases, "scout") {
		t.Errorf("completedPhases = %v, want to include \"scout\" (completed before the crash)", phases)
	}
	if containsStr(phases, "triage") {
		t.Errorf("completedPhases = %v — must NOT include \"triage\": the phase FAILED, recording it as complete corrupts resume", phases)
	}
	if retro.calls != 1 {
		t.Fatalf("retro calls = %d, want 1: hard mid-cycle failures must trigger learning before returning", retro.calls)
	}
	if got := retro.requests[0].Context["failed_phase"]; got != "triage" {
		t.Errorf("retro failed_phase context = %q, want triage", got)
	}
}

func TestOrchestrator_FailedPhase_QueuesNextCycleTodo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedCycleStateFile(t, root)

	retro := &recordingRetroRunner{name: "retro"}
	orch, st, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  retro,
	}))
	res, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	if err == nil {
		t.Fatal("triage hard failure must still surface after learning runs")
	}
	if len(res.PhasesRun) == 0 || res.PhasesRun[len(res.PhasesRun)-1] != core.PhaseRetro {
		t.Fatalf("last phase = %v, want retro to run on the error path", res.PhasesRun)
	}
	if len(st.state.CarryoverTodos) != 1 {
		t.Fatalf("carryover todos = %+v, want one failure-learning todo", st.state.CarryoverTodos)
	}
	todo := st.state.CarryoverTodos[0]
	if todo.ID != "cycle-1-failed-triage" {
		t.Errorf("todo id = %q, want cycle-1-failed-triage", todo.ID)
	}
	if todo.Priority != "P0" || !strings.Contains(todo.Action, "triage") {
		t.Errorf("todo = %+v, want P0 action mentioning triage", todo)
	}
	if len(st.state.FailedAt) != 1 || !st.state.FailedAt[0].Retrospected {
		t.Fatalf("failedApproaches = %+v, want one retrospected failure record", st.state.FailedAt)
	}
	if got := retro.requests[0].Context["next_cycle_todo_id"]; got != todo.ID {
		t.Errorf("retro next_cycle_todo_id = %q, want %q", got, todo.ID)
	}
}
