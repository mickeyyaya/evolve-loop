package core

import (
	"context"
	"testing"
)

type driverRecordingRunner struct {
	drivers []string
}

func (r *driverRecordingRunner) Name() string { return string(PhaseBuild) }

func (r *driverRecordingRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.drivers = append(r.drivers, req.Driver.String())
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestReviewWithCorrections_TheEscalatedDriverEndsWithTheLadder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Reason: "[missing_section] no Verdict", Blocks: 1},
		{Reason: "[missing_section] no Verdict", Blocks: 2},
		{Approve: true},
	}}
	runner := &driverRecordingRunner{}
	cr, dr := correctionRunE4(t, rev, &fakeLedger{}, root, runner)
	cr.retryConfig.ContractCorrectionRetries = 3
	var sink injectedStderr
	cr.o.stderr = &sink

	if act, err := cr.reviewWithCorrections(PhaseBuild, dr); act != loopNext || err != nil {
		t.Fatalf("act=%v err=%v, want the approved correction to advance", act, err)
	}
	if len(runner.drivers) != 2 || runner.drivers[1] != "codex-tmux" {
		t.Fatalf("correction dispatches ran on %v, want the second one escalated to codex-tmux", runner.drivers)
	}
	if !dr.phaseReq.Driver.IsZero() {
		t.Fatalf("the escalated driver %q outlived the ladder: the escalation applies to the re-dispatch only", dr.phaseReq.Driver)
	}
}
