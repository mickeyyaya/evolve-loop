package recipe

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// fakeSource drives the ReadFile/ReadDir error branches the always-valid
// embedded FS cannot reach.
type fakeSource struct {
	files   map[string][]byte
	readErr error
	dirErr  error
}

func (f fakeSource) ReadFile(name string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	if b, ok := f.files[name]; ok {
		return b, nil
	}
	return nil, fs.ErrNotExist
}

func (f fakeSource) ReadDir(string) ([]fs.DirEntry, error) {
	if f.dirErr != nil {
		return nil, f.dirErr
	}
	return nil, nil
}

func withRecipeFS(t *testing.T, s recipeSource) {
	t.Helper()
	orig := recipeFS
	recipeFS = s
	t.Cleanup(func() { recipeFS = orig })
}

func TestLoadRecipe_Embedded(t *testing.T) {
	// The real embedded plugin-install.json must load + validate.
	r, err := LoadRecipe("plugin-install")
	if err != nil {
		t.Fatalf("LoadRecipe: %v", err)
	}
	if r.Name != "plugin-install" {
		t.Errorf("name=%q", r.Name)
	}
	if _, err := r.stepsFor("claude-tmux"); err != nil {
		t.Errorf("claude-tmux arm: %v", err)
	}
	if _, err := r.stepsFor("ollama-tmux"); !errors.Is(err, ErrUnsupportedCLI) {
		t.Errorf("ollama should be unsupported, err=%v", err)
	}
}

func TestLoadRecipe_EmptyName(t *testing.T) {
	if _, err := LoadRecipe(""); err == nil {
		t.Fatal("want error for empty name")
	}
}

func TestLoadRecipe_OverrideDirWins(t *testing.T) {
	dir := t.TempDir()
	override := `{"name":"plugin-install","steps":[{"name":"ov","send":{"kind":"command","body":"/override"},"await":{"kind":"prompt_marker","timeout_s":5}}]}`
	if err := os.WriteFile(filepath.Join(dir, "plugin-install.json"), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_BRIDGE_RECIPE_DIR", dir)
	r, err := LoadRecipe("plugin-install")
	if err != nil {
		t.Fatalf("LoadRecipe: %v", err)
	}
	steps, _ := r.stepsFor("anything")
	if len(steps) != 1 || steps[0].Send.Body != "/override" {
		t.Fatalf("override not applied: %+v", steps)
	}
}

func TestLoadRecipe_NotFound(t *testing.T) {
	withRecipeFS(t, fakeSource{files: map[string][]byte{}})
	if _, err := LoadRecipe("does-not-exist"); err == nil {
		t.Fatal("want not-found error")
	}
}

func TestLoadRecipe_InvalidJSON(t *testing.T) {
	withRecipeFS(t, fakeSource{files: map[string][]byte{"recipes/bad.json": []byte("{not json")}})
	if _, err := LoadRecipe("bad"); err == nil {
		t.Fatal("want JSON error")
	}
}

func TestParseRecipe_NameFallback(t *testing.T) {
	r, err := parseRecipe("fallback", []byte(`{"steps":[{"send":{"kind":"command","body":"/x"},"await":{"kind":"prompt_marker","timeout_s":5}}]}`))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if r.Name != "fallback" {
		t.Errorf("name=%q want fallback", r.Name)
	}
}

func TestValidate_Rejections(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"no steps at all", `{"name":"e"}`},
		{"empty body", `{"name":"e","steps":[{"send":{"kind":"command","body":"  "},"await":{"kind":"prompt_marker","timeout_s":5}}]}`},
		{"bad send kind", `{"name":"e","steps":[{"send":{"kind":"shout","body":"/x"},"await":{"kind":"prompt_marker","timeout_s":5}}]}`},
		{"non-positive timeout", `{"name":"e","steps":[{"send":{"kind":"command","body":"/x"},"await":{"kind":"prompt_marker","timeout_s":0}}]}`},
		{"bad await kind", `{"name":"e","steps":[{"send":{"kind":"command","body":"/x"},"await":{"kind":"weird","timeout_s":5}}]}`},
		{"uncompilable regex", `{"name":"e","steps":[{"send":{"kind":"command","body":"/x"},"await":{"kind":"regex","regex":"([","timeout_s":5}}]}`},
		{"bad on_timeout", `{"name":"e","steps":[{"send":{"kind":"command","body":"/x"},"await":{"kind":"prompt_marker","timeout_s":5},"on_timeout":"explode"}]}`},
		{"per_cli arm invalid", `{"name":"e","per_cli":{"claude-tmux":[{"send":{"kind":"command","body":""},"await":{"kind":"prompt_marker","timeout_s":5}}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseRecipe("e", []byte(tc.json)); !errors.Is(err, ErrInvalidRecipe) {
				t.Fatalf("err=%v want ErrInvalidRecipe", err)
			}
		})
	}
}

func TestValidate_OnTimeoutValuesAccepted(t *testing.T) {
	for _, ot := range []string{"", "abort", "continue"} {
		j := `{"name":"e","steps":[{"send":{"kind":"command","body":"/x"},"await":{"kind":"prompt_marker","timeout_s":5},"on_timeout":"` + ot + `"}]}`
		if _, err := parseRecipe("e", []byte(j)); err != nil {
			t.Errorf("on_timeout=%q rejected: %v", ot, err)
		}
	}
}

func TestRecipeNames(t *testing.T) {
	names := RecipeNames()
	found := false
	for _, n := range names {
		if n == "plugin-install" {
			found = true
		}
	}
	if !found {
		t.Errorf("plugin-install not in %v", names)
	}
}

func TestRecipeNames_DirError(t *testing.T) {
	withRecipeFS(t, fakeSource{dirErr: errors.New("boom")})
	if names := RecipeNames(); names != nil {
		t.Errorf("want nil on ReadDir error, got %v", names)
	}
}
