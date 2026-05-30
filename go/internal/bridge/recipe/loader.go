package recipe

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// recipes/*.json are the embedded recipe definitions, making the engine
// self-contained. The operator override directory (recipeDir) is consulted
// first so a project can ship or tweak recipes without a rebuild — the exact
// precedence LoadManifest uses for manifests.
//
//go:embed recipes/*.json
var embeddedRecipes embed.FS

// recipeSource is the test-swappable embedded source. The embed.FS satisfies
// it in production; tests inject a fake to drive ReadFile/ReadDir error
// branches the always-valid embedded set cannot reach.
type recipeSource interface {
	ReadFile(name string) ([]byte, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

var recipeFS recipeSource = embeddedRecipes

// recipeDir is the writable override directory consulted before the embedded
// set: EVOLVE_BRIDGE_RECIPE_DIR, else .evolve/bridge-recipes.
func recipeDir() string {
	if d := os.Getenv("EVOLVE_BRIDGE_RECIPE_DIR"); d != "" {
		return d
	}
	return filepath.Join(".evolve", "bridge-recipes")
}

// LoadRecipe reads, parses, and validates the recipe named name (no .json
// suffix). Override directory wins over the embedded set.
func LoadRecipe(name string) (Recipe, error) {
	if name == "" {
		return Recipe{}, fmt.Errorf("recipe: empty name")
	}
	if data, err := os.ReadFile(filepath.Join(recipeDir(), name+".json")); err == nil {
		return parseRecipe(name, data)
	}
	data, err := recipeFS.ReadFile("recipes/" + name + ".json")
	if err != nil {
		return Recipe{}, fmt.Errorf("recipe: not found: %s", name)
	}
	return parseRecipe(name, data)
}

// RecipeNames returns the sorted set of embedded recipe names.
func RecipeNames() []string {
	entries, err := recipeFS.ReadDir("recipes")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".json") {
			out = append(out, strings.TrimSuffix(n, ".json"))
		}
	}
	sort.Strings(out)
	return out
}

// parseRecipe unmarshals + validates recipe bytes.
func parseRecipe(name string, data []byte) (Recipe, error) {
	var r Recipe
	if err := json.Unmarshal(data, &r); err != nil {
		return Recipe{}, fmt.Errorf("recipe: invalid JSON for %s: %w", name, err)
	}
	if r.Name == "" {
		r.Name = name
	}
	if err := r.validate(); err != nil {
		return Recipe{}, err
	}
	return r, nil
}

// validate rejects malformed recipes at load time: a recipe must declare at
// least one step arm, and every step must have a non-empty body, a valid send
// kind, a positive timeout, and an await that compiles.
func (r Recipe) validate() error {
	if len(r.Steps) == 0 && len(r.PerCLI) == 0 {
		return fmt.Errorf("%w: recipe %s has no steps", ErrInvalidRecipe, r.Name)
	}
	if err := validateSteps(r.Name, "", r.Steps); err != nil {
		return err
	}
	for cli, steps := range r.PerCLI {
		if err := validateSteps(r.Name, cli, steps); err != nil {
			return err
		}
	}
	return nil
}

func validateSteps(recipe, cli string, steps []Step) error {
	where := recipe
	if cli != "" {
		where = recipe + " (cli=" + cli + ")"
	}
	for i, s := range steps {
		if strings.TrimSpace(s.Send.Body) == "" {
			return fmt.Errorf("%w: %s step %d has empty send body", ErrInvalidRecipe, where, i)
		}
		if s.Send.Kind != KindCommand && s.Send.Kind != KindKeys {
			return fmt.Errorf("%w: %s step %d invalid send kind %q", ErrInvalidRecipe, where, i, s.Send.Kind)
		}
		if s.Await.TimeoutS <= 0 {
			return fmt.Errorf("%w: %s step %d await timeout_s must be > 0", ErrInvalidRecipe, where, i)
		}
		if _, err := s.Await.compile(); err != nil {
			return fmt.Errorf("%s step %d: %w", where, i, err)
		}
		if s.OnTimeout != "" && s.OnTimeout != OnTimeoutAbort && s.OnTimeout != OnTimeoutContinue {
			return fmt.Errorf("%w: %s step %d invalid on_timeout %q", ErrInvalidRecipe, where, i, s.OnTimeout)
		}
	}
	return nil
}
