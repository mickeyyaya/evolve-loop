package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var errUnknownCycle = errors.New("no recorded state")

var routingPlanAdvisor = wireRoutingPlanAdvisor

func wireRoutingPlanAdvisor(projectRoot string, console io.Writer) (*core.PhaseAdvisor, error) {
	evolveDir := paths.EvolveDirOf(projectRoot)
	bridge.SetModelCatalogDirFn(func() string { return evolveDir })
	prm := cmdutil.NewPromptsLoader(projectRoot)
	catalog, _, _ := loadPhaseCatalog(projectRoot, config.RegistryPath(projectRoot), prm)
	cliRouter, err := wireCLIRouter(routerSite{root: projectRoot, catalog: catalog}, routingRun{env: filterEvolveEnv(os.Environ()), role: roleCycle}, console)
	if err != nil {
		return nil, err
	}
	walked := bridgechain.New(bridge.NewDefault(projectRoot, nil), bridgechain.DefaultPlanResolver(cliRouter))
	return wireRouterAdvisor(routerAdvisorWiring{router: cliRouter, root: projectRoot, bridge: walked, prompts: prm})
}

func routingPlanInput(projectRoot string, cycle int, goalText, workspace string) (router.RouteInput, error) {
	recorded := filepath.Join(cycleWorkspace(projectRoot, cycle), "cycle-state.json")
	raw, err := os.ReadFile(recorded)
	if errors.Is(err, os.ErrNotExist) {
		return router.RouteInput{}, fmt.Errorf("cycle %d: %w at %s", cycle, errUnknownCycle, recorded)
	}
	if err != nil {
		return router.RouteInput{}, err
	}
	var state cyclestate.CycleState
	if err := json.Unmarshal(raw, &state); err != nil {
		return router.RouteInput{}, fmt.Errorf("parse %s: %w", recorded, err)
	}
	if state.CycleID != cycle {
		return router.RouteInput{}, fmt.Errorf("cycle %d: %w: %s records cycle %d", cycle, errUnknownCycle, recorded, state.CycleID)
	}
	if goalText == "" {
		goalText = state.GoalText
	}
	in := router.RouteInput{
		Workspace:   workspace,
		ProjectRoot: projectRoot,
		Cycle:       cycle,
		GoalText:    goalText,
	}
	cat, err := phasespec.Load(config.RegistryPath(projectRoot))
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return router.RouteInput{}, err
	default:
		in.Catalog, in.OnDemandPhases = core.AdvisorCatalog(cat)
	}
	return in, nil
}

func runRoutingPlan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("routing plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cycle := fs.Int("cycle", 0, "cycle whose recorded state seeds the plan (required, >=1)")
	root := fs.String("project-root", "", "project root (default: cwd or EVOLVE_PROJECT_ROOT)")
	goalText := fs.String("goal-text", "", "goal to plan for (default: the goal recorded for the cycle)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if *cycle <= 0 {
		fmt.Fprintln(stderr, "evolve routing plan: --cycle N is required (N>=1)")
		return 10
	}
	projectRoot, err := routingProjectRoot(*root, routingGetwd)
	if err != nil {
		fmt.Fprintf(stderr, "evolve routing plan: %v\n", err)
		return 1
	}
	scratch, err := os.MkdirTemp("", "evolve-routing-plan-")
	if err != nil {
		fmt.Fprintf(stderr, "evolve routing plan: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	in, err := routingPlanInput(projectRoot, *cycle, *goalText, scratch)
	if err != nil {
		fmt.Fprintf(stderr, "evolve routing plan: %v\n", err)
		return 1
	}
	adv, err := routingPlanAdvisor(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve routing plan: advisor not wired: %v\n", err)
		return 2
	}
	plan, err := adv.Plan(in)
	if err != nil {
		fmt.Fprintf(stderr, "evolve routing plan: advisor launch failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "Advisor plan — cycle %d (nothing written under runs/cycle-%d)\n\n", *cycle, *cycle)
	writePlanEntries(stdout, plan.Entries)
	return 0
}
