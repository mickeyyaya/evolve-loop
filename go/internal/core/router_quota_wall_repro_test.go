package core

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type walledPlanner struct{ err error }

func (p walledPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) { return nil, p.err }

func TestRunCycle_ARouterPlanEndingAtAQuotaWallPausesTheCycle(t *testing.T) {
	t.Parallel()
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	walled := WalkError{Wall: Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"}), Err: wrapTransient(85)}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}), WithPlanner(walledPlanner{err: walled}))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the quota pause: a router launch that ends at a usage-limit wall must not degrade to the static spine", err)
	}
}
