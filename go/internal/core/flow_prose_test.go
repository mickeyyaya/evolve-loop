package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestReportProseFallbacks_EmitsEachFallbackOncePerCycle(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	cr := &cycleRun{o: &Orchestrator{signals: c}, cycle: 12, current: PhaseTriage}
	sig := router.RoutingSignals{ProseFallbacks: []router.ProseFallback{
		{Seat: "scout", Path: "/ws/scout-report.md", Stage: phasecontract.FlowStageLegacy},
		{Seat: "triage", Path: "/ws/triage-report.md", Stage: phasecontract.FlowStageEnforce, Refused: true},
	}}
	cr.reportProseFallbacks(sig)
	cr.reportProseFallbacks(sig)
	if len(*got) != 2 {
		t.Fatalf("each fallback is reported once per cycle, got %+v", *got)
	}
	scout, triage := (*got)[0], (*got)[1]
	if scout.Fields["action"] != "logged" || scout.Fields["rule"] != phasecontract.FlowRuleProseFallback || scout.Fields["cell"] != "scout" ||
		scout.Fields["target"] != "/ws/scout-report.md" || scout.Cycle != 12 || scout.Phase != "triage" || scout.Origin != "cycleRun.reportProseFallbacks" {
		t.Errorf("scout event %+v", scout)
	}
	if triage.Fields["action"] != "refused" || triage.Fields["stage"] != "enforce" {
		t.Errorf("triage event %+v", triage)
	}
	cr.reportProseFallbacks(router.RoutingSignals{})
	if len(*got) != 2 {
		t.Fatalf("no fallback, no event: %+v", *got)
	}
}

type proseScout struct{ fakeRunner }

func (s *proseScout) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(filepath.Join(req.Workspace, "scout-report.md"), []byte("goal_type: bugfix\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return s.fakeRunner.Run(ctx, req)
}

func TestRunCycle_AScoutReportWithNoHandoffEmitsTheProseFallbackOnce(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	runners := buildRunners(nil)
	runners[PhaseScout] = &proseScout{fakeRunner{name: string(PhaseScout)}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithRouting(shadowCfg(config.StageAdvisory), router.StaticPreset{}), WithSignalCenter(c))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "prose"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	var scout []string
	for _, e := range *got {
		if e.Code == isolation.ViolationCode(8) && e.Fields["cell"] == "scout" {
			scout = append(scout, e.Fields["action"]+" "+e.Fields["rule"])
		}
	}
	if len(scout) != 1 || scout[0] != "logged "+phasecontract.FlowRuleProseFallback {
		t.Fatalf("the routing digest read the scout prose: want one logged prose fallback, got %v", scout)
	}
}
