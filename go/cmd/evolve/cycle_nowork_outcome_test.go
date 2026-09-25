package main

// cycle_nowork_outcome_test.go — F30's loop breaker at the cycle-run root.
// Every fleet lane runs `evolve cycle run` as a subprocess and the parent wave
// loop's fleet.Result carries neither the lane's cycle number nor its
// workspace, so a lane's closeout happens HERE. closeoutCycleOutcome is the
// one post-result closeout: the failure walk for a FAIL, the planned-no-work
// hand-off for a lane that answered for its scope without committing, nothing
// for anything else.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// seedNoWorkLane writes a lane scoped to one inbox item whose triage dropped
// it with a reason (the cycle-1682 shape) and returns the .evolve dir.
func seedNoWorkLane(t *testing.T, root string, cycle int) string {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	ws := cycleWorkspace(root, cycle)
	for _, dir := range []string{inbox, ws} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(inbox, "2026-09-26T00-00-00Z-answered.json"): `{"id":"answered","title":"fixture","kind":"bug"}`,
		filepath.Join(ws, "lane-scope.json"):                       `{"todo_ids":["answered"],"goal_hash":"g"}`,
		filepath.Join(ws, "triage-decision.json"):                  `{"top_n":[],"dropped":[{"id":"answered","reason":"already-shipped-cycle-1679"}]}`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return evolveDir
}

func plannedNoWork(cycle int) core.CycleResult {
	return core.CycleResult{
		Cycle:             cycle,
		FinalVerdict:      core.CycleOutcomeSkippedUnknown,
		TerminationReason: core.CycleTerminationTriageNoWork,
		PhasesRun:         []core.Phase{core.PhaseScout, core.PhaseTriage},
	}
}

func routeOf(t *testing.T, evolveDir, id string) string {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(filepath.Join(evolveDir, "inbox"), id)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	body, _ := os.ReadFile(path)
	var item struct {
		Route string `json:"route"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item.Route
}

func TestCloseoutCycleOutcome_PlannedNoWorkHandsTheLanesAnsweredItemToTheConsole(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	fake := newFakeLedger()
	var stderr bytes.Buffer

	closeoutCycleOutcome(plannedNoWork(7), root, evolveDir, &stderr, fake, nil)

	if got := routeOf(t, evolveDir, "answered"); got != "console-manual" {
		t.Fatalf("route = %q — the lane's answered item goes to the console, or the next wave draws it into another no-work lane", got)
	}
	if len(fake.lifecycle) == 0 {
		t.Error("the hand-off's lifecycle line goes through the root's ledger (ADR-0101 S4a)")
	}
}

// TestCloseoutCycleOutcome_OnlyAPlannedNoWorkHandsOver: a PASS never hands its
// scope to the console, and neither does a SKIPPED that is not the planned
// no-work disposition (IsTriageNoWorkResult is the one authority).
func TestCloseoutCycleOutcome_OnlyAPlannedNoWorkHandsOver(t *testing.T) {
	notNoWork := plannedNoWork(7)
	notNoWork.TerminationReason = "audit-advisory"
	for _, result := range []core.CycleResult{{Cycle: 7, FinalVerdict: core.VerdictPASS}, notNoWork} {
		root := t.TempDir()
		evolveDir := seedNoWorkLane(t, root, 7)
		closeoutCycleOutcome(result, root, evolveDir, io.Discard, newFakeLedger(), nil)
		if got := routeOf(t, evolveDir, "answered"); got != "" {
			t.Errorf("verdict %q / reason %q routed the scope to %q", result.FinalVerdict, result.TerminationReason, got)
		}
	}
}

// TestCloseoutCycleOutcome_ReportsWhatItApplied: the one closeout names what
// it applied so each root WARNs in its own voice; nothing applies to a PASS.
func TestCloseoutCycleOutcome_ReportsWhatItApplied(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	if applied, err := closeoutCycleOutcome(plannedNoWork(7), root, evolveDir, io.Discard, newFakeLedger(), nil); applied != "no-work hand-off" || err != nil {
		t.Errorf("planned no-work: applied=%q err=%v", applied, err)
	}
	if applied, err := closeoutCycleOutcome(core.CycleResult{Cycle: 7, FinalVerdict: core.VerdictPASS}, root, evolveDir, io.Discard, newFakeLedger(), nil); applied != "" || err != nil {
		t.Errorf("a PASS applies nothing: applied=%q err=%v", applied, err)
	}
}

// TestCompleteSequentialCycle_PlannedNoWorkHandsTheLaneOver (F30 architecture
// review M1): the sequential loop makes the same one closeout, so a lane pin
// reaching it — however it got there — is handed over like the cycle-run
// root's.
func TestCompleteSequentialCycle_PlannedNoWorkHandsTheLaneOver(t *testing.T) {
	root := t.TempDir()
	evolveDir := seedNoWorkLane(t, root, 7)
	fake := newFakeLedger()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: fake}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	state := &sequentialBatchState{maxConsecutiveFails: 5, stallCfg: goalStallConfig{threshold: 100, nonprogressThreshold: 100}}
	b.completeSequentialCycle(0, sequentialCycle{cycle: 7, workspace: cycleWorkspace(root, 7), result: plannedNoWork(7)}, state)
	if got := routeOf(t, evolveDir, "answered"); got != "console-manual" {
		t.Fatalf("the sequential loop hands a planned-no-work lane's answered item over: route=%q stderr=%s", got, stderr.String())
	}
	if len(fake.lifecycle) == 0 {
		t.Error("through the root's ledger")
	}
}
