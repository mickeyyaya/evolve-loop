package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestCompleteCycle_AFailedToolOutputPruneWarnsAndStillSeals(t *testing.T) {
	ws := t.TempDir()
	busy := filepath.Join(ws, gcpolicy.ToolOutputFiles()[0])
	if err := os.MkdirAll(filepath.Join(busy, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
	cr := &cycleRun{
		ctx:    context.Background(),
		o:      o,
		cycle:  8,
		req:    CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "tool-output-retention"},
		cs:     CycleState{WorkspacePath: ws, Shipped: true},
		result: CycleResult{Cycle: 8, FinalVerdict: VerdictPASS, PhasesRun: []Phase{PhaseBuild, PhaseAudit, PhaseShip}},
	}
	var err error

	stderr := captureStderr(t, func() { err = cr.completeCycle() })

	if err != nil || !cr.cycleCompletedNormally {
		t.Fatalf("completeCycle = %v (completed %v), want a sealed cycle: the prune is non-fatal", err, cr.cycleCompletedNormally)
	}
	if !strings.Contains(stderr, "[orchestrator] WARN cycle 8: the raw tool output stays (non-fatal): delete raw tool output "+busy) {
		t.Errorf("stderr = %q, want the prune WARN naming %s", stderr, busy)
	}
}
