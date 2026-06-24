package core

// orchestrator_ghostphase_test.go — cycle-265 incident RED tests: the routing
// surface (registry order + catalog) can know phases the DISPATCH surface
// cannot run. Live: registry-listed `memo` had no .evolve/phases config, so
// no specrunner was registered; after the post-ship optional phases the
// static order walked into it and `no runner registered for phase memo`
// killed a batch whose cycle had already PASSED and shipped.
//
// Kernel floor: a selected-but-unregistered OPTIONAL phase is skipped loudly
// (WARN; the order walk continues; it never appears in PhasesRun — nothing
// dispatched). A missing MANDATORY runner remains a fatal wiring bug.

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// intoUserPhaseStrategy routes scout → target (a catalog user phase), then
// leaves the static walk alone.
type intoUserPhaseStrategy struct{ target string }

func (s intoUserPhaseStrategy) Decide(in router.RouteInput) router.RouterDecision {
	if in.Current == string(PhaseScout) {
		return router.RouterDecision{NextPhase: s.target, Reason: "test: enter user-phase region"}
	}
	return router.RouterDecision{}
}
func (intoUserPhaseStrategy) Recover(router.RouteInput) router.RouterDecision {
	return router.RouterDecision{}
}

func TestRunCycle_UnregisteredOptionalPhase_SkippedLoudly(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	// user-a IS dispatchable; ghost-phase is catalog/order-known but has NO
	// runner — the cycle-265 memo shape.
	runners[Phase("user-a")] = &fakeRunner{name: "user-a"}

	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "user-a", Optional: true},
		{Name: "ghost-phase", Optional: true},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	cfg := dynamicCfg()
	cfg.Order = []string{"scout", "user-a", "ghost-phase", "build", "audit", "ship"}

	o := NewOrchestrator(st, led, runners,
		WithRouting(cfg, intoUserPhaseStrategy{target: "user-a"}),
		WithPlanner(shipPlanner{}),
		WithCatalog(cat))
	res, rerr := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	if rerr != nil {
		t.Fatalf("an unregistered OPTIONAL phase must not kill the cycle (cycle-265: memo killed a PASSING batch); got %v", rerr)
	}
	if indexOfPhase(res.PhasesRun, "user-a") < 0 {
		t.Errorf("user-a should have dispatched; PhasesRun=%v", res.PhasesRun)
	}
	if indexOfPhase(res.PhasesRun, "ghost-phase") >= 0 {
		t.Errorf("ghost-phase never dispatched — it must not appear in PhasesRun; got %v", res.PhasesRun)
	}
	if indexOfPhase(res.PhasesRun, "ship") < 0 {
		t.Errorf("the walk must continue past the ghost to ship; PhasesRun=%v", res.PhasesRun)
	}
}

// A missing MANDATORY runner stays fatal — that is a wiring bug, not a
// routing-surface drift.
func TestRunCycle_UnregisteredMandatoryPhase_StillFatal(t *testing.T) {
	t.Parallel()
	runners := buildRunners(nil)
	delete(runners, PhaseBuild)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err == nil || !strings.Contains(err.Error(), "no runner registered") {
		t.Fatalf("a missing mandatory runner must abort loudly; got %v", err)
	}
}
