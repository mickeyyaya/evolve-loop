package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
)

func TestResumeDispatch_NonCanonicalVerdict_RecordsPhaseOutcome(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 644},
		cycleState: CycleState{CycleID: 644, WorkspacePath: ws, CompletedPhases: []string{string(PhaseScout), string(PhaseTriage), string(PhaseBuild)}},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	o.resumeDispatch = func(_ *cycleRun, next Phase, _ PhaseRequest, _ retryOpts) (map[string]bool, PhaseResponse, int, error) {
		return nil, PhaseResponse{Phase: string(next), Verdict: "BOGUS"}, 1, nil
	}

	res, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "test-goal"},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 644})
	if err == nil {
		t.Fatal("non-canonical verdict from resumeDispatch must return an error")
	}
	if want := []Phase{PhaseAudit, PhaseRetro}; !slices.Equal(res.PhasesRun, want) {
		t.Errorf("PhasesRun = %v, want %v: the audit outcome, then the retrospective", res.PhasesRun, want)
	}
	sidecarRaw, err := os.ReadFile(outcome.UsageSidecarPath(ws, string(PhaseAudit)))
	if err != nil {
		t.Fatalf("read usage sidecar: %v", err)
	}
	var sc outcome.UsageSidecar
	if err := json.Unmarshal(sidecarRaw, &sc); err != nil {
		t.Fatalf("decode usage sidecar: %v", err)
	}
	if sc.AbortReason == "" {
		t.Errorf("usage sidecar for %s has empty abort_reason: %+v", PhaseAudit, sc)
	}
	timingRaw, err := os.ReadFile(filepath.Join(ws, "phase-timing.json"))
	if err != nil {
		t.Fatalf("read phase-timing.json: %v", err)
	}
	if len(timingRaw) == 0 {
		t.Error("phase-timing.json is empty")
	}
}
