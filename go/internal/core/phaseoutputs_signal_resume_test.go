package core

// phaseoutputs_signal_resume_test.go — the RESUME-topology wiring proof for
// the survey signal. The RunCycle proof alone left this line deletable with a
// green suite (review finding): dropping the resume defer's call would
// recreate the 1452/1453 silent-non-reporting class on exactly the topology
// that runs after a crash — when the accounting matters most.

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
