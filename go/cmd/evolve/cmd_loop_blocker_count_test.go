package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
)

func TestCompleteSequentialCycle_AProvenBlockerDoesNotAdvanceTheConsecutiveFailCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir, ws := seedFailedCycleInbox(t, root, "poison", 7)
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: newFakeLedger()}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	state := &sequentialBatchState{maxConsecutiveFails: 2, consecutiveFails: 1, stallCfg: goalStallConfig{threshold: 100, nonprogressThreshold: 100}}
	blocked := core.CycleResult{Cycle: 7, FinalVerdict: core.VerdictFAIL, TerminationReason: core.CycleTerminationBlockedNeedsConsole}

	d := b.completeSequentialCycle(0, sequentialCycle{cycle: 7, workspace: ws, result: blocked}, state)

	if d.flow == batchStopIterations || state.consecutiveFails != 1 {
		t.Errorf("flow=%v streak=%d; a proven blocker is not a code FAIL, so a streak of 1 of 2 must stay 1 and the batch must go on", d.flow, state.consecutiveFails)
	}
	if b.result.ContinuedFailures != 0 || !strings.Contains(stderr.String(), "blocked: needs console") {
		t.Errorf("continued=%d stderr=%q", b.result.ContinuedFailures, stderr.String())
	}

	d = b.completeSequentialCycle(0, sequentialCycle{cycle: 8, workspace: ws, result: core.CycleResult{Cycle: 8, FinalVerdict: core.VerdictFAIL}}, state)
	if d.flow != batchStopIterations || state.consecutiveFails != 2 {
		t.Errorf("flow=%v streak=%d; a code FAIL after the blocker still reaches the ceiling", d.flow, state.consecutiveFails)
	}
}

func TestCompleteSequentialCycle_TheBudgetAndStallBranchesAfterAShip(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		stage    cyclebudget.Stage
		state    string
		verdict  string
		wantStop bool
		wantErr  string
	}{
		"no state file goes on":                  {cyclebudget.Enforce, "", core.VerdictPASS, false, ""},
		"an advisory stop only warns":            {cyclebudget.Advisory, `{"carryoverTodos":[]}`, core.VerdictPASS, false, "cycle-budget ADVISORY"},
		"an enforced stop ends the batch":        {cyclebudget.Enforce, `{"carryoverTodos":[]}`, core.VerdictPASS, true, "cycle-budget: stopping (goal_complete)"},
		"a backlog under the cap goes on":        {cyclebudget.Enforce, `{"carryoverTodos":[{}]}`, core.VerdictPASS, false, ""},
		"an empty cycle at the threshold stalls": {cyclebudget.Off, "", core.CycleOutcomeSkippedUnknown, false, "consecutive"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			evolveDir := filepath.Join(root, ".evolve")
			if err := os.MkdirAll(evolveDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.state != "" {
				if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(tc.state), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir, GoalHash: "g"}, deps: orchDeps{Ledger: newFakeLedger()}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
			state := &sequentialBatchState{maxConsecutiveFails: 3, budgetStage: tc.stage, effectiveMax: 10, stallCfg: goalStallConfig{threshold: 1, nonprogressThreshold: 100}}

			d := b.completeSequentialCycle(0, sequentialCycle{cycle: 7, workspace: t.TempDir(), result: core.CycleResult{Cycle: 7, FinalVerdict: tc.verdict}}, state)

			if (d.flow == batchStopIterations) != tc.wantStop || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Errorf("flow=%v stderr=%q; want stop=%v and %q", d.flow, stderr.String(), tc.wantStop, tc.wantErr)
			}
		})
	}
}
