package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

func TestRunCycleFromPhase_EmitsPhaseOutputsSurvey(t *testing.T) {
	ws := t.TempDir()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 7},
		cycleState: CycleState{CycleID: 7, WorkspacePath: ws, CompletedPhases: []string{"scout", "build"}},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 7}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("resume finalize emitted nothing to the unified stream: %v", err)
	}
	if !strings.Contains(string(raw), string(dispatchevents.EventPhaseOutputsSurveyed)) {
		t.Fatalf("no %s event on the resume topology: %s", dispatchevents.EventPhaseOutputsSurveyed, raw)
	}
}
