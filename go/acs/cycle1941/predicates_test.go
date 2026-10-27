//go:build acs

package cycle1941

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var strategyOptionsDocumentPhases = []string{"premise-challenge", "forces-analysis", "market-sizing", "scope-baseline"}

func realRoutingConfig(t *testing.T, root string) config.RoutingConfig {
	t.Helper()
	registry := config.RegistryPath(root)
	cfg, _ := config.Load(registry)
	builtin, err := phasespec.Load(registry)
	if err != nil {
		t.Fatalf("load builtin catalog: %v", err)
	}
	specs, _ := phasespec.DiscoverUserSpecs(filepath.Join(root, ".evolve", "phases"))
	phasespec.ApplyUserRouting(&cfg, specs, builtin)
	return cfg
}

func recipePhases(recipe []string) []string {
	var phases []string
	for _, step := range recipe {
		if !strings.HasPrefix(step, "[") {
			phases = append(phases, step)
		}
	}
	return phases
}

func documentLaneSignals(goalType string) router.RoutingSignals {
	return router.RoutingSignals{
		Scout:  router.ScoutSignals{Present: true, GoalType: goalType, DeliverableKind: router.DeliverableKindDocument, ItemCount: 1},
		Triage: router.TriageSignals{Present: true, CycleSize: "small", DeliverableKind: router.DeliverableKindDocument},
	}
}

func routeRecipeLane(cfg config.RoutingConfig, goalType string, recipe []string) router.RouterDecision {
	var entries []router.PhasePlanEntry
	for _, phase := range append(append([]string{"scout", "triage"}, recipe...), "build", "audit", "ship") {
		entries = append(entries, router.PhasePlanEntry{Phase: phase, Run: true, Justification: "goal recipe " + goalType})
	}
	return router.Route(router.RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Completed: []string{"scout", "triage"},
		Signals:   documentLaneSignals(goalType),
		Cfg:       cfg,
		Plan:      &router.PhasePlan{Entries: entries},
	}, nil)
}

func phasesGatedByTheirOwnTrigger(dec router.RouterDecision, phases []string) []string {
	var gated []string
	for _, c := range dec.Clamps {
		if c.Rule != router.RuleInsertWhenGatesPlan {
			continue
		}
		phase := strings.TrimSuffix(c.Forced, "=skip")
		if slices.Contains(phases, phase) {
			gated = append(gated, phase)
		}
	}
	return gated
}

func admitsOnlyByGoalType(block config.RoutingBlock) bool {
	for _, c := range block.InsertWhen {
		if c.Field != "scout.goal_type" {
			return false
		}
	}
	return len(block.InsertWhen) > 0
}

func TestC1941_001_StrategyOptionsDocumentLanePlanRunsItsRecipePhases(t *testing.T) {
	cfg := realRoutingConfig(t, acsassert.RepoRoot(t))
	recipe := recipePhases(cfg.GoalRecipes["strategy-options"])
	for _, phase := range strategyOptionsDocumentPhases {
		if !slices.Contains(recipe, phase) {
			t.Fatalf("goal_recipes.strategy-options = %v, want it to name %s", recipe, phase)
		}
	}
	dec := routeRecipeLane(cfg, "strategy-options", recipe)
	for _, phase := range strategyOptionsDocumentPhases {
		if slices.Contains(dec.SkipPhases, phase) {
			t.Errorf("RED: a strategy-options document lane skips %s (SkipPhases=%v)", phase, dec.SkipPhases)
		}
	}
	if gated := phasesGatedByTheirOwnTrigger(dec, strategyOptionsDocumentPhases); len(gated) > 0 {
		t.Errorf("RED: insert-when-gates-plan clamps strategy-options recipe phases %v", gated)
	}
	if !slices.Contains(strategyOptionsDocumentPhases, dec.NextPhase) {
		t.Errorf("RED: NextPhase after triage = %q, want one of the recipe phases %v", dec.NextPhase, strategyOptionsDocumentPhases)
	}
}

func TestC1941_002_NoGoalRecipePhaseIsGatedOffByItsOwnInsertWhen(t *testing.T) {
	cfg := realRoutingConfig(t, acsassert.RepoRoot(t))
	if len(cfg.GoalRecipes) == 0 {
		t.Fatal("RED: the real registry loaded no goal_recipes")
	}
	var gated []string
	for goalType, recipe := range cfg.GoalRecipes {
		phases := recipePhases(recipe)
		for _, phase := range phasesGatedByTheirOwnTrigger(routeRecipeLane(cfg, goalType, phases), phases) {
			if admitsOnlyByGoalType(cfg.Triggers[phase]) {
				gated = append(gated, goalType+" -> "+phase)
			}
		}
	}
	slices.Sort(gated)
	if len(gated) > 0 {
		t.Errorf("RED: goal recipe phases whose insert_when never admits the recipe goal type: %v", gated)
	}
}

func TestC1941_003_TheSweepDetectsARecipePhaseGatedByAnotherGoalType(t *testing.T) {
	gatedByAnotherGoalType := config.RoutingConfig{
		Stage:         config.StageAdvisory,
		Mandatory:     []string{"scout", "build", "audit"},
		MaxInsertions: 10,
		Order:         []string{"scout", "triage", "forces-analysis", "build", "audit", "ship"},
		Triggers: map[string]config.RoutingBlock{
			"forces-analysis": {InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "==", Value: "business-strategy"}}},
		},
	}
	dec := routeRecipeLane(gatedByAnotherGoalType, "strategy-options", []string{"forces-analysis"})
	if !admitsOnlyByGoalType(gatedByAnotherGoalType.Triggers["forces-analysis"]) {
		t.Errorf("a goal-type-only trigger was not classified as one")
	}
	gatedBySize := config.RoutingBlock{InsertWhen: []config.Condition{{Field: "scout.cycle_size", Op: "==", Value: "large"}}}
	if admitsOnlyByGoalType(gatedBySize) || admitsOnlyByGoalType(config.RoutingBlock{}) {
		t.Errorf("a size trigger or an empty trigger was classified as goal-type-only")
	}
	if gated := phasesGatedByTheirOwnTrigger(dec, []string{"forces-analysis"}); !slices.Equal(gated, []string{"forces-analysis"}) {
		t.Errorf("the sweep missed a recipe phase gated by another goal type: gated=%v clamps=%+v", gated, dec.Clamps)
	}
	admitted := gatedByAnotherGoalType
	admitted.Triggers = map[string]config.RoutingBlock{
		"forces-analysis": {InsertWhen: []config.Condition{
			{Field: "scout.goal_type", Op: "==", Value: "business-strategy"},
			{Field: "scout.goal_type", Op: "==", Value: "strategy-options"},
		}},
	}
	if gated := phasesGatedByTheirOwnTrigger(routeRecipeLane(admitted, "strategy-options", []string{"forces-analysis"}), []string{"forces-analysis"}); len(gated) > 0 {
		t.Errorf("the sweep flagged a phase whose insert_when admits the recipe goal type: %v", gated)
	}
}

func TestC1941_004_RouterRegistryTestPinsGoalRecipeTriggers(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", "GoalRecipe", "./internal/router")
	if err != nil || code != 0 {
		t.Fatalf("RED: go test -run GoalRecipe ./internal/router exit=%d err=%v\n%s\n%s", code, err, out, stderr)
	}
	if !strings.Contains(out, "--- PASS: TestGoalRecipe") {
		t.Errorf("RED: no TestGoalRecipe* registry test ran in ./internal/router:\n%s", out)
	}
}
