package router

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

const (
	recipeBeginMarker = "<!-- GENERATED:goal-recipes BEGIN"
	recipeEndMarker   = "<!-- GENERATED:goal-recipes END -->"
)

func TestRouterPersonaRecipeTable_NoDrift(t *testing.T) {
	const personaPath = "../../../agents/evolve-router.md"
	const registryPath = "../../../docs/architecture/phase-registry.json"

	raw, err := os.ReadFile(personaPath)
	if err != nil {
		t.Fatalf("read persona %s: %v", personaPath, err)
	}
	persona := string(raw)

	begin := strings.Index(persona, recipeBeginMarker)
	if begin < 0 {
		t.Fatalf("BEGIN marker %q not found in %s", recipeBeginMarker, personaPath)
	}
	nl := strings.IndexByte(persona[begin:], '\n')
	if nl < 0 {
		t.Fatalf("BEGIN marker line has no terminating newline in %s", personaPath)
	}
	blockStart := begin + nl + 1

	end := strings.Index(persona[blockStart:], recipeEndMarker)
	if end < 0 {
		t.Fatalf("END marker %q not found after BEGIN in %s", recipeEndMarker, personaPath)
	}
	extracted := persona[blockStart : blockStart+end]

	cfg, _ := config.Load(registryPath, map[string]string{})
	if len(cfg.GoalRecipes) == 0 {
		t.Fatalf("config.goal_recipes is empty in %s — the recipe SSOT must be present", registryPath)
	}
	want := RenderRecipeProjection(cfg.GoalRecipes)

	if extracted != want {
		t.Errorf("persona recipe table has drifted from the SSOT (config.goal_recipes).\n"+
			"Regenerate the block between the GENERATED markers in %s with the output of\n"+
			"router.RenderRecipeProjection(config.Load(%q).GoalRecipes).\n\n--- persona (extracted) ---\n%s\n--- want (projection) ---\n%s",
			personaPath, registryPath, extracted, want)
	}
}

func recipeStepPhases(recipe []string) []string {
	var phases []string
	for _, step := range recipe {
		if !strings.HasPrefix(step, "[") {
			phases = append(phases, step)
		}
	}
	return phases
}

func gatedByGoalTypeOnly(block config.RoutingBlock) bool {
	if !block.TriggerIsTheWholeRule() {
		return false
	}
	for _, c := range block.InsertWhen {
		if c.Field != "scout.goal_type" {
			return false
		}
	}
	return true
}

func TestGoalRecipePhases_GoalTypeTriggersAdmitTheRecipeGoalType(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	if len(cfg.GoalRecipes) == 0 {
		t.Fatal("the real registry loaded no goal_recipes")
	}
	var gated []string
	for goalType, recipe := range cfg.GoalRecipes {
		for _, phase := range recipeStepPhases(recipe) {
			block := cfg.Triggers[phase]
			if gatedByGoalTypeOnly(block) && !triggerFires(documentLaneSignals(goalType), block) {
				gated = append(gated, goalType+" -> "+phase)
			}
		}
	}
	slices.Sort(gated)
	if len(gated) > 0 {
		t.Errorf("goal_recipes name phases whose insert_when never admits the recipe goal type: %v", gated)
	}
}

func TestGoalRecipeStrategyOptions_DocumentLaneRunsItsRecipePhases(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	want := []string{"premise-challenge", "forces-analysis", "market-sizing", "scope-baseline"}
	var entries []PhasePlanEntry
	for _, phase := range append(append([]string{"scout", "triage"}, want...), "build", "audit", "ship") {
		entries = append(entries, PhasePlanEntry{Phase: phase, Run: true, Justification: "strategy-options recipe"})
	}
	dec := Route(RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Completed: []string{"scout", "triage"},
		Signals:   documentLaneSignals("strategy-options"),
		Cfg:       cfg,
		Plan:      &PhasePlan{Entries: entries},
	}, nil)
	for _, phase := range want {
		if slices.Contains(dec.SkipPhases, phase) {
			t.Errorf("SkipPhases = %v, want %s run on a strategy-options document lane", dec.SkipPhases, phase)
		}
		if clampForces(dec, RuleInsertWhenGatesPlan, phase+"=skip") {
			t.Errorf("Clamps = %+v, want no %s on %s", dec.Clamps, RuleInsertWhenGatesPlan, phase)
		}
	}
	if !slices.Contains(want, dec.NextPhase) {
		t.Errorf("NextPhase = %q, want one of the recipe phases %v", dec.NextPhase, want)
	}
}

func TestGoalRecipeTriggerSweep_FlagsOnlyGoalTypeOnlyTriggers(t *testing.T) {
	cases := []struct {
		name  string
		block config.RoutingBlock
		want  bool
	}{
		{"goal type", config.RoutingBlock{InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "==", Value: "business-strategy"}}}, true},
		{"size", config.RoutingBlock{InsertWhen: []config.Condition{{Field: "scout.cycle_size", Op: "==", Value: "large"}}}, false},
		{"mixed", config.RoutingBlock{InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "==", Value: "x"}, {Field: "scout.cycle_size", Op: "==", Value: "large"}}}, false},
		{"rubric hint", config.RoutingBlock{InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "==", Value: "x"}}, RubricHint: []string{"h"}}, false},
		{"empty", config.RoutingBlock{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gatedByGoalTypeOnly(tc.block); got != tc.want {
				t.Errorf("gatedByGoalTypeOnly = %v, want %v", got, tc.want)
			}
		})
	}
}
