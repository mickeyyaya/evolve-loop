package core_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

func TestRecordFailureLearning_CarryoverTodoStampsExpiresAt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedCycleStateFile(t, root)

	retro := &recordingRetroRunner{name: "retro"}
	orch, st, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  retro,
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	if len(st.state.CarryoverTodos) != 1 {
		t.Fatalf("carryover todos = %+v, want exactly one failure-learning todo", st.state.CarryoverTodos)
	}
	todo := st.state.CarryoverTodos[0]
	if todo.ExpiresAt == "" {
		t.Fatal("recordFailureLearning must stamp ExpiresAt on the todo it creates so " +
			"failurelog.PruneExpiredCarryoverTodos can age it out (state.go:87-91 documents " +
			"this contract, but the creation site never assigns it — the live bug this test pins)")
	}
	expires, err := time.Parse(time.RFC3339, todo.ExpiresAt)
	if err != nil {
		t.Fatalf("ExpiresAt = %q must be RFC3339, got parse error: %v", todo.ExpiresAt, err)
	}
	if !expires.After(time.Now().UTC()) {
		t.Errorf("ExpiresAt = %s must be in the future immediately after creation", todo.ExpiresAt)
	}
}

func TestRecordFailureLearning_FailedRecordStampsExpiresAt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedCycleStateFile(t, root)

	orch, st, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  &alwaysErrRunner{name: "retro"},
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	if len(st.state.FailedAt) != 1 {
		t.Fatalf("FailedAt = %+v, want exactly one cycle-mid-execution-fail record", st.state.FailedAt)
	}
	rec := st.state.FailedAt[0]
	if rec.ExpiresAt == "" {
		t.Fatal("recordFailureLearning must stamp the FailedRecord's ExpiresAt")
	}
	expires, err := time.Parse(time.RFC3339, rec.ExpiresAt)
	if err != nil {
		t.Fatalf("ExpiresAt = %q must be RFC3339, got parse error: %v", rec.ExpiresAt, err)
	}
	if !expires.After(time.Now().UTC()) {
		t.Errorf("ExpiresAt = %s must be in the future immediately after creation", rec.ExpiresAt)
	}
}

func TestRecordFailureLearning_CreatedTodoSurvivesImmediatePrune(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedCycleStateFile(t, root)

	retro := &recordingRetroRunner{name: "retro"}
	orch, st, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  retro,
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}
	if len(st.state.CarryoverTodos) != 1 {
		t.Fatalf("carryover todos = %+v, want exactly one failure-learning todo", st.state.CarryoverTodos)
	}

	statePath := filepath.Join(t.TempDir(), "state.json")
	raw, err := json.Marshal(st.state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		t.Fatalf("write state fixture: %v", err)
	}

	result, err := failurelog.PruneExpiredCarryoverTodos(statePath, time.Now().UTC())
	if err != nil {
		t.Fatalf("PruneExpiredCarryoverTodos: %v", err)
	}
	if result.Removed != 0 {
		t.Errorf("a todo created THIS SECOND must not be pruned yet (result=%+v) — an unstamped "+
			"ExpiresAt is kept forever by prune's own legacy rule, so a non-zero Removed here means "+
			"something ELSE is wrong; a zero-but-empty stamp would also incorrectly show Removed=0, "+
			"which is why the positive tests above assert the stamp is non-empty first", result)
	}

	after, rerr := os.ReadFile(statePath)
	if rerr != nil {
		t.Fatalf("read pruned state: %v", rerr)
	}
	var decoded map[string]any
	if err := json.Unmarshal(after, &decoded); err != nil {
		t.Fatalf("parse pruned state: %v", err)
	}
	todos, _ := decoded["carryoverTodos"].([]any)
	if len(todos) != 1 {
		t.Fatalf("carryoverTodos after prune = %v, want the fresh todo to survive", todos)
	}
	survivor, _ := todos[0].(map[string]any)
	expiresAt, _ := survivor["expiresAt"].(string)
	if expiresAt == "" {
		// Surviving with Removed==0 alone is not sufficient proof: an unstamped
		// todo also survives, by prune's own "age unknown, never delete" rule.
		t.Fatal("surviving todo has no expiresAt — it survived by the legacy no-stamp rule, " +
			"not because a real TTL stamp hasn't elapsed yet; recordFailureLearning must stamp ExpiresAt")
	}
}
