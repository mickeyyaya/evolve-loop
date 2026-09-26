package router

import (
	"strings"
	"testing"
)

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

func TestRenderRecipeProjection_EmptyIsEmpty(t *testing.T) {
	if got := RenderRecipeProjection(nil); got != "" {
		t.Errorf("empty recipes should render empty, got %q", got)
	}
}
