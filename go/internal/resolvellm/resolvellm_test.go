package resolvellm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve_EmptyRole(t *testing.T) {
	t.Parallel()
	if _, err := Resolve("", Options{}); err == nil {
		t.Fatal("want error for empty role")
	}
}

func TestResolve_ProfileFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli":                "claude",
		"model_tier_default": "sonnet",
	})
	r, err := Resolve("scout", Options{ProjectRoot: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := `{"cli":"claude","model_tier":"sonnet","source":"profile"}`
	if r.JSON() != want {
		t.Errorf("JSON parity:\n got=%s\nwant=%s", r.JSON(), want)
	}
}

func TestResolve_ProfileDefaultsTierToBalanced(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli": "claude", // no model_tier_default — sentinel kicks in
	})
	r, err := Resolve("scout", Options{ProjectRoot: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.ModelTier != "balanced" {
		t.Errorf("want balanced default, got %+v", r)
	}
}

func TestResolve_ProfileMissingCLI_IsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".evolve", "profiles", "scout.json"), map[string]any{
		"model_tier_default": "sonnet",
	})
	if _, err := Resolve("scout", Options{ProjectRoot: dir}); err == nil {
		t.Fatal("want error when profile.cli missing")
	}
}

func TestResolve_ProfilePathIsDirectory_ReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A directory where the profile file should be → findProfile's Stat sees it
	// exist, then os.ReadFile fails → the read-error branch (not ErrProfileNotFound).
	if err := os.MkdirAll(filepath.Join(dir, ".evolve", "profiles", "scout.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve("scout", Options{ProjectRoot: dir})
	if err == nil || errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("want a profile read error, got %v", err)
	}
}

func TestResolve_ProfileInvalidJSON_ParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, ".evolve", "profiles", "scout.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("not json {{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve("scout", Options{ProjectRoot: dir})
	if err == nil || errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("want a profile parse error, got %v", err)
	}
}

func TestResolve_ProfileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := Resolve("nonexistent-role-xyz", Options{ProjectRoot: dir, GitRoot: dir})
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("want ErrProfileNotFound, got %v", err)
	}
}

func TestResolve_PluginRootFirst(t *testing.T) {
	t.Parallel()
	plugin := t.TempDir()
	project := t.TempDir()
	writeJSON(t, filepath.Join(plugin, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli": "gemini", "model_tier_default": "opus",
	})
	writeJSON(t, filepath.Join(project, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli": "claude", "model_tier_default": "sonnet",
	})
	r, err := Resolve("scout", Options{PluginRoot: plugin, ProjectRoot: project})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.CLI != "gemini" || r.ModelTier != "opus" {
		t.Errorf("plugin precedence broken: %+v", r)
	}
}

func TestResolve_EnvDrivenRoots(t *testing.T) {
	t.Parallel()
	plugin := t.TempDir()
	project := t.TempDir()
	writeJSON(t, filepath.Join(plugin, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli": "gemini", "model_tier_default": "opus",
	})
	writeJSON(t, filepath.Join(project, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli": "claude", "model_tier_default": "sonnet",
	})
	env := func(k string) string {
		switch k {
		case "EVOLVE_PLUGIN_ROOT":
			return plugin
		case "EVOLVE_PROJECT_ROOT":
			return project
		}
		return ""
	}
	r, err := Resolve("scout", Options{Env: env})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.CLI != "gemini" {
		t.Errorf("env-driven plugin root precedence broken: %+v", r)
	}
}

// TestResolve_IgnoresLLMConfig is the Step-9 behavior-change proof: a
// .evolve/llm_config.json present on disk is now IGNORED — resolution comes
// solely from the profile. (Before Step 9 the llm_config phase entry would have
// won.) See docs/architecture/step9-llm-config-removal.md.
func TestResolve_IgnoresLLMConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// An llm_config that, pre-Step-9, would have forced cli=codex/model=gpt-5.5.
	writeJSON(t, filepath.Join(dir, ".evolve", "llm_config.json"), map[string]any{
		"phases":    map[string]any{"scout": map[string]any{"cli": "codex", "model": "gpt-5.5"}},
		"_fallback": map[string]any{"cli": "gemini", "model_tier": "deep"},
	})
	writeJSON(t, filepath.Join(dir, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli": "claude", "model_tier_default": "balanced",
	})
	r, err := Resolve("scout", Options{ProjectRoot: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Source != "profile" || r.CLI != "claude" || r.ModelTier != "balanced" {
		t.Errorf("llm_config must be ignored — profile must win; got %+v", r)
	}
}
