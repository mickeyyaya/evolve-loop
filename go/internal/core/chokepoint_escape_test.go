// chokepoint_escape_test.go — ADR-0044 C1 invariant regression (inbox
// cycle-terminal-path-escapes-c1-chokepoint, weight 0.98; the cycle-492 escape).
//
// Contract: when RunCycle's bounded dispatch loop exhausts its iteration budget
// without reaching PhaseEnd (a transition-table cycle keeps re-selecting phases),
// the cycle MUST record an explicit terminal abort so cyclehealth.ClassifyOutcome
// classifies it FAILED_EXPLAINED — never the FAILED_UNEXPLAINED alarm bucket. The
// escape is a CYCLE-level failure (loud + diagnosable), never batch-fatal.
//
// Driven deterministically: WithMaxPhaseIterations caps the loop below the spine
// length, so an all-PASS cycle exits via the iteration bound (the exact escape
// path) instead of reaching ship→end. Shares the core_test harness (recStorage,
// fakeLedger, newRunners) from orchestrator_recovery_test.go.
package core_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
)

// TestRunCycle_TransitionCycleGuard_RecordsChokepointEscape pins the C1
// invariant: the bounded-loop escape is recorded as a terminal abort, so the
// cycle never classifies FAILED_UNEXPLAINED.
func TestRunCycle_TransitionCycleGuard_RecordsChokepointEscape(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	// Cap at 2: the spine (scout→triage→tdd→…→ship→end) needs far more than 2
	// dispatches to reach PhaseEnd, so the loop exits via the iteration bound —
	// exactly the transition-cycle escape path the guard must catch.
	orch := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil),
		core.WithMaxPhaseIterations(2))
	res, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	// The escape is CYCLE-level: RunCycle returns without a batch-fatal error.
	if err != nil {
		t.Fatalf("RunCycle returned a batch-fatal error; the chokepoint escape must fail the cycle in-band: %v", err)
	}
	// A transition-cycle escape must never look like success.
	if res.FinalVerdict == core.VerdictPASS {
		t.Errorf("FinalVerdict = PASS — a transition-cycle escape must not classify as success")
	}

	// The real contract: the cycle workspace's phase-timing.json carries a
	// terminal abort, so ClassifyOutcome does NOT page FAILED_UNEXPLAINED.
	ws := findCycleWorkspace(t, projectRoot)
	outcome, detail := cyclehealth.ClassifyOutcome(ws)
	if outcome == cyclehealth.OutcomeFailedUnexplained {
		t.Fatalf("cycle classified FAILED_UNEXPLAINED — the C1 chokepoint escaped; detail=%q", detail)
	}
	if outcome != cyclehealth.OutcomeFailedExplained {
		t.Errorf("outcome = %q, want FAILED_EXPLAINED (%q)", outcome, cyclehealth.OutcomeFailedExplained)
	}
	if !strings.Contains(detail, "transition-table cycle") {
		t.Errorf("abort detail = %q, want it to name the transition-table cycle (the diagnosable reason)", detail)
	}
}

// findCycleWorkspace returns the cycle-N run dir that holds a phase-timing.json.
func findCycleWorkspace(t *testing.T, projectRoot string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(projectRoot, ".evolve", "runs", "cycle-*"))
	if err != nil {
		t.Fatalf("glob cycle workspaces under %s: %v", projectRoot, err)
	}
	for _, m := range matches {
		if _, err := os.Stat(filepath.Join(m, "phase-timing.json")); err == nil {
			return m
		}
	}
	t.Fatalf("no cycle workspace with phase-timing.json under %s (matches=%v)", projectRoot, matches)
	return ""
}
