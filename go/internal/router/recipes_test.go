package router

import (
	"strings"
	"testing"
)

// WS5-S1 (ADR-0052): RenderRecipeProjection is the generate side of the
// single-source goal-type recipe projection (P10). It renders the recipe SSOT
// (cfg.GoalRecipes) into the persona's "Goal-Type Recipes" table body — one row
// per goal type, sorted for determinism, tokens joined with " → ". The persona
// table is drift-locked against this output (WS5-S2) and the RecipeVerifier
// reads the same source, killing the three-source recipe drift (gap #3).

func TestRenderRecipeProjection_FromConfig(t *testing.T) {
	recipes := map[string][]string{
		"bugfix":       {"fault-localization", "bug-reproduction", "[tdd, build]", "coverage-gate"},
		"docs/trivial": {"[spine only]"},
	}
	got := RenderRecipeProjection(recipes)
	want := "| bugfix | fault-localization → bug-reproduction → [tdd, build] → coverage-gate |\n" +
		"| docs/trivial | [spine only] |\n"
	if got != want {
		t.Errorf("RenderRecipeProjection mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderRecipeProjection_Deterministic(t *testing.T) {
	recipes := map[string][]string{
		"zeta":  {"z1", "z2"},
		"alpha": {"a1"},
		"mid":   {"m1", "m2"},
	}
	first := RenderRecipeProjection(recipes)
	for i := 0; i < 20; i++ {
		if got := RenderRecipeProjection(recipes); got != first {
			t.Fatalf("non-deterministic output on run %d:\nfirst=%q\ngot=%q", i, first, got)
		}
	}
	ai, mi, zi := strings.Index(first, "| alpha |"), strings.Index(first, "| mid |"), strings.Index(first, "| zeta |")
	if !(ai >= 0 && ai < mi && mi < zi) {
		t.Errorf("rows not sorted by goal type: alpha=%d mid=%d zeta=%d\n%s", ai, mi, zi, first)
	}
}

// An empty recipe set renders to the empty string (no rows) — a clean no-op so
// the projection is safe before any recipes are wired.
func TestRenderRecipeProjection_EmptyIsEmpty(t *testing.T) {
	if got := RenderRecipeProjection(nil); got != "" {
		t.Errorf("empty recipes should render empty, got %q", got)
	}
}
