package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type routerDecisionType int

const (
	decisionPlan routerDecisionType = iota
	decisionRePlan
	decisionPropose
	decisionJudge
)

const (
	routerAgent        = "router"
	routerDefaultModel = "opus"
)

func routerDecision(r *cliroute.Router, projectRoot string) (cliroute.Decision, error) {
	return r.Resolve(cliroute.Request{
		Agent: routerAgent, Launch: cliroute.LaunchAdvisor, ProjectRoot: projectRoot,
		DefaultModel: routerDefaultModel, Env: filterEvolveEnv(os.Environ()),
	})
}

func decisionModel(base string, dt routerDecisionType, rc policy.RouterPolicy) string {
	switch {
	case (dt == decisionPlan || dt == decisionRePlan) && rc.PlanModel != "":
		return rc.PlanModel
	case (dt == decisionPropose || dt == decisionJudge) && rc.ProposeModel != "":
		return rc.ProposeModel
	}
	return base
}

type routerDispatch struct {
	cli     string
	model   string
	healthy bool
	walk    llmroute.Plan
}

func resolveRouterDispatchHealthy(r *cliroute.Router, projectRoot string, dt routerDecisionType, benched map[string]bool) (routerDispatch, error) {
	d, err := routerDecision(r, projectRoot)
	if err != nil {
		return routerDispatch{}, err
	}
	model := decisionModel(d.Plan.Model, dt, r.Policy().RouterConfig())
	walk, healthy, err := d.WalkAt(model, func(cli string) bool { return benched[llmroute.Family(cli)] })
	if err != nil {
		return routerDispatch{}, err
	}
	return routerDispatch{cli: walk.Candidates[0], model: model, healthy: healthy, walk: walk}, nil
}

func routerAdvisorOptions(rd routerDispatch) []core.PhaseAdvisorOption {
	return []core.PhaseAdvisorOption{core.WithProposerCLI(rd.cli), core.WithProposerModel(rd.model), core.WithProposerWalk(rd.walk)}
}

func benchedFamilies(projectRoot string) map[string]bool {
	out := map[string]bool{}
	for family := range clihealth.NewStore(projectRoot, nil).Active() {
		out[family] = true
	}
	return out
}

type routerAdvisorWiring struct {
	router  *cliroute.Router
	root    string
	bridge  core.Bridge
	prompts *prompts.Loader
	signals *signalcenter.Center
}

func wireRouterAdvisor(w routerAdvisorWiring) (*core.PhaseAdvisor, error) {
	rd, err := resolveRouterDispatchHealthy(w.router, w.root, decisionPlan, benchedFamilies(w.root))
	if err != nil {
		return nil, fmt.Errorf("the routing advisor has no route: %w", err)
	}
	if !rd.healthy {
		fmt.Fprintf(os.Stderr, "[router] WARN router family and the claude fallback are both benched — advisor will degrade to the static spine\n")
	}
	var persona string
	if rp, err := w.prompts.Agent("evolve-router"); err == nil {
		persona = rp.Body
	} else {
		fmt.Fprintf(os.Stderr, "[router] WARN persona evolve-router.md not loaded (%v); advisor uses legacy inline framing\n", err)
	}
	return core.NewPhaseAdvisor(w.bridge, append(routerAdvisorOptions(rd),
		core.WithPersona(persona),
		core.WithDepthCheck(core.AdvisorDepthExceeded),
		core.WithAdvisorSignals(w.signals),
	)...), nil
}
