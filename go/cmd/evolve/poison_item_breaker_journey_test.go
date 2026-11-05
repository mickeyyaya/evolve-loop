package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type laneRun struct {
	cycle    int
	decision map[string]any
	timing   []phasetiming.Entry
}

func writeJourneyJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, _ := json.Marshal(v)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runFailedLane(t *testing.T, root, id string, run laneRun, signals *signalcenter.Center) error {
	t.Helper()
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	if _, err := inboxmover.Claim(opts, id, strconv.Itoa(run.cycle)); err != nil {
		return err
	}
	ws := cycleWorkspace(root, run.cycle)
	writeJourneyJSON(t, filepath.Join(ws, "lane-scope.json"), map[string]any{"todo_ids": []string{id}, "goal_hash": "h"})
	writeJourneyJSON(t, filepath.Join(ws, "triage-decision.json"), run.decision)
	writeJourneyJSON(t, phasetiming.Path(ws), run.timing)
	result := core.CycleResult{Cycle: run.cycle, FinalVerdict: cyclestate.VerdictFAIL}
	if _, err := closeoutCycleOutcome(result, root, filepath.Join(root, ".evolve"), io.Discard, newFakeLedger(), signals); err != nil {
		t.Fatalf("closeout cycle %d: %v", run.cycle, err)
	}
	return nil
}

func seedJourneyItem(t *testing.T, root, id string) string {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeJourneyJSON(t, filepath.Join(inbox, "2026-09-14T00-00-00Z-"+id+".json"), map[string]any{"id": id, "title": "fixture " + id, "kind": "bug"})
	return inbox
}

func journeyItem(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(dir, id)
	if err != nil {
		t.Fatalf("%s not in %s: %v", id, dir, err)
	}
	body, _ := os.ReadFile(path)
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	return m
}

func TestPoisonItemJourney_TwoPreBuildFailuresParkTheItemAndTheNextLaneCannotClaimIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := "wave-orchestrator-o6-wire-oversight-into-the-loop"
	inbox := seedJourneyItem(t, root, id)
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	for _, cycle := range []int{2133, 2151} {
		run := laneRun{cycle: cycle,
			decision: map[string]any{"top_n": []map[string]string{{"id": id}}},
			timing: []phasetiming.Entry{
				{Phase: "scout", Verdict: cyclestate.VerdictPASS, AttemptCount: 1},
				{Phase: "triage", Verdict: cyclestate.VerdictFAIL, Diagnostics: []cyclestate.Diagnostic{{
					Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeTriageCommitmentInvalid, Message: "unified_commitment processing failed",
				}}},
			}}
		if err := runFailedLane(t, root, id, run, center); err != nil {
			t.Fatalf("lane of cycle %d could not claim: %v", cycle, err)
		}
	}

	rec := journeyItem(t, filepath.Join(inbox, "quarantine"), id)
	if got := rec["prebuild_failure_cycles"]; !sameCycles(got, 2133, 2151) {
		t.Errorf("prebuild_failure_cycles = %v, want [2133 2151]", got)
	}
	if !hasBreakerEvent(events, id) {
		t.Errorf("no INBOX_ITEM_PREBUILD_BREAKER inbox.warning names %s: %+v", id, events)
	}
	_, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, id, "2160")
	if !errors.Is(err, inboxmover.ErrNotFound) {
		t.Fatalf("the third lane claim = %v, want ErrNotFound: the parked item must never be re-picked", err)
	}
}

func TestProtectedSurfaceJourney_ASecondLaneNeverClaimsTheItemTriageRouted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := "edit-the-role-guard"
	inbox := seedJourneyItem(t, root, id)
	run := laneRun{cycle: 2140,
		decision: map[string]any{
			"top_n":          []map[string]string{{"id": "companion-card"}},
			"escalate_block": []map[string]string{{"task_id": id, "reason": "protected-surface: go/internal/guards/role.go — control-plane changes go through the console route (operator-gated), not lane top_n"}},
		},
		timing: []phasetiming.Entry{
			{Phase: "scout", Verdict: cyclestate.VerdictPASS, AttemptCount: 1},
			{Phase: "triage", Verdict: cyclestate.VerdictPASS, Diagnostics: []cyclestate.Diagnostic{{
				Severity: cyclestate.SeverityWarning, Code: cyclestate.DiagCodeTriageProtectedSurface, Subject: id,
			}}},
			{Phase: "tdd", Verdict: cyclestate.VerdictFAIL, Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "tdd wrote no predicate"}}},
		}}
	if err := runFailedLane(t, root, id, run, nil); err != nil {
		t.Fatalf("the first lane could not claim: %v", err)
	}

	if rec := journeyItem(t, inbox, id); rec["route"] != "console-manual" {
		t.Errorf("route = %v, want console-manual after triage's protected-surface rejection", rec["route"])
	}
	err := runFailedLane(t, root, id, laneRun{cycle: 2141, decision: run.decision, timing: run.timing}, nil)
	if !errors.Is(err, inboxmover.ErrConsoleRouted) {
		t.Fatalf("the second lane claim = %v, want ErrConsoleRouted", err)
	}
}

func sameCycles(got any, want ...int) bool {
	list, ok := got.([]any)
	if !ok || len(list) != len(want) {
		return false
	}
	for i, w := range want {
		if n, ok := list[i].(float64); !ok || int(n) != w {
			return false
		}
	}
	return true
}

func hasBreakerEvent(events []signalcenter.Event, id string) bool {
	for _, e := range events {
		if e.Code == "INBOX_ITEM_PREBUILD_BREAKER" && e.Kind == signalcenter.KindInboxWarning && e.Fields["task_id"] == id {
			return true
		}
	}
	return false
}
