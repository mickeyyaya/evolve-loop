package router

import (
	"fmt"
	"sort"
	"strings"
)

// RenderRecipeProjection renders the goal-type recipe table body from the recipe
// SSOT (config.RoutingConfig.GoalRecipes, passed as the bare map to keep router
// free of a config import). It is the generate side of the single-source recipe
// projection (ADR-0052 WS5, P10): one markdown row per goal type, sorted by goal
// type for determinism, recipe tokens joined with " → ". The persona's
// "Goal-Type Recipes" table is locked against this output (WS5-S2), and the
// RecipeVerifier reads the same source — ending the three-source recipe drift
// (the persona prose, the registry triggers, and per-phase metadata used to drift
// independently). A nil/empty map renders to the empty string (a clean no-op).
func RenderRecipeProjection(recipes map[string][]string) string {
	types := make([]string, 0, len(recipes))
	for t := range recipes {
		types = append(types, t)
	}
	sort.Strings(types)
	var b strings.Builder
	for _, t := range types {
		fmt.Fprintf(&b, "| %s | %s |\n", t, strings.Join(recipes[t], " → "))
	}
	return b.String()
}
