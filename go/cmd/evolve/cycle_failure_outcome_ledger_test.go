package main

// cycle_failure_outcome_ledger_test.go — ADR-0101 S4a: the failed-cycle inbox
// walk (cycleoutcome.ApplyFailure) appends its lifecycle lines through the
// ROOT's ledger — the one the Signal Center observes — on every path that
// reaches it: the cycle-run root and both sequential loop paths. Before this
// the inbox mover built its own unobserved FileLedger over the same file
// (S4a architecture review HIGH-1).

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// seedFailedCycleInbox writes one inbox item and the cycle workspace's
// triage-decision.json committing it, and returns (evolveDir, workspace).
func seedFailedCycleInbox(t *testing.T, root, id string, cycle int) (string, string) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"id": id, "title": "fixture " + id, "kind": "bug"})
	if err := os.WriteFile(filepath.Join(inbox, "2026-09-13T00-00-00Z-"+id+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	ws := cycleWorkspace(root, cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	dec, _ := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": id}}})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), dec, 0o644); err != nil {
		t.Fatal(err)
	}
	return evolveDir, ws
}

func assertLifecycleWentThroughTheRootLedger(t *testing.T, evolveDir string, fake *fakeLedgerNoAppend) {
	t.Helper()
	if len(fake.lifecycle) == 0 {
		t.Fatal("the failure walk's lifecycle lines must go through the root's ledger")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "ledger.jsonl")); !os.IsNotExist(err) {
		t.Errorf("a self-constructed ledger wrote the lifecycle beside the root's (stat err=%v)", err)
	}
}

func TestApplyCycleFailureOutcome_AppendsLifecycleThroughTheGivenLedger(t *testing.T) {
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
	fake := newFakeLedger()
	if err := applyCycleFailureOutcome(root, evolveDir, 7, io.Discard, fake); err != nil {
		t.Fatalf("applyCycleFailureOutcome: %v", err)
	}
	assertLifecycleWentThroughTheRootLedger(t, evolveDir, fake)
}

func TestCompleteSequentialCycle_FailureWalkGoesThroughTheRootLedger(t *testing.T) {
	root := t.TempDir()
	evolveDir, ws := seedFailedCycleInbox(t, root, "poison", 7)
	fake := newFakeLedger()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: fake}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	state := &sequentialBatchState{maxConsecutiveFails: 5, stallCfg: goalStallConfig{threshold: 100, nonprogressThreshold: 100}}
	b.completeSequentialCycle(0, sequentialCycle{cycle: 7, workspace: ws, result: core.CycleResult{Cycle: 7, FinalVerdict: core.VerdictFAIL}}, state)
	assertLifecycleWentThroughTheRootLedger(t, evolveDir, fake)
}

func TestHandleCycleError_FailureWalkGoesThroughTheRootLedger(t *testing.T) {
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
	fake := newFakeLedger()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: fake}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	cycleErr := &core.ErrCycleLevelFailure{Phase: "build", Cause: errors.New("builder crashed")}
	if d := b.handleCycleError(core.CycleResult{Cycle: 7}, cycleErr); d.flow != batchNextIteration {
		t.Fatalf("a cycle-level failure continues the batch: %+v", d)
	}
	assertLifecycleWentThroughTheRootLedger(t, evolveDir, fake)
}

// The two voices of a failed walk: the cycle-run root's and the loop's. Both
// WARN the seam's error and never change their caller's flow.
func TestWarnCycleFailureOutcome_SpeaksOnlyOnAnError(t *testing.T) {
	var stderr bytes.Buffer
	warnCycleFailureOutcome(&stderr, 7, nil)
	if stderr.Len() != 0 {
		t.Errorf("a clean walk prints nothing: %q", stderr.String())
	}
	warnCycleFailureOutcome(&stderr, 7, errors.New("inbox unreadable"))
	if want := "evolve cycle run: WARN: could not apply cycle 7 failure outcome to the inbox: inbox unreadable\n"; stderr.String() != want {
		t.Errorf("the cycle root's voice:\n got %q\nwant %q", stderr.String(), want)
	}
}

func TestLoopApplyCycleFailureOutcome_WarnsInTheLoopsVoiceOnAFailedWalk(t *testing.T) {
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
	// An inbox root that is a file, not a directory: the walk cannot move
	// the committed item and reports it.
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.RemoveAll(inbox); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inbox, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: newFakeLedger()}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	b.applyCycleFailureOutcome(7)
	if !strings.Contains(stderr.String(), "[loop] WARN: could not apply cycle 7 failure outcome to the inbox: ") {
		t.Errorf("the loop's voice on a failed walk: %q", stderr.String())
	}
}
