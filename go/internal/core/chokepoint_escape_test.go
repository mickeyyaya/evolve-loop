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

func TestRunCycle_TransitionCycleGuard_RecordsChokepointEscape(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	// The spine needs far more than 2 dispatches, so the loop exits via the iteration bound.
	orch := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(nil),
		core.WithMaxPhaseIterations(2))
	res, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	if err != nil {
		t.Fatalf("RunCycle returned a batch-fatal error; the chokepoint escape must fail the cycle in-band: %v", err)
	}
	if res.FinalVerdict == core.VerdictPASS {
		t.Errorf("FinalVerdict = PASS — a transition-cycle escape must not classify as success")
	}

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
