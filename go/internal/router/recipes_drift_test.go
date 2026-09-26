package router

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// The BEGIN marker is matched by prefix so the note inside it can change without breaking the lock.
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
