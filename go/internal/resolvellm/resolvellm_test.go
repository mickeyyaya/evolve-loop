package resolvellm

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	_, err := Resolve("", Options{})
	if err == nil {
		t.Fatal("want error for empty role")
	}
}

func TestResolve_LLMConfigPhaseWithExactModel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"phases": map[string]any{
			"scout": map[string]any{"cli": "gemini", "model": "gemini-3-pro-preview"},
		},
	})

	r, err := Resolve("scout", Options{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.CLI != "gemini" || r.Model != "gemini-3-pro-preview" || r.Source != "llm_config" {
		t.Errorf("bad result: %+v", r)
	}
	want := `{"cli":"gemini","model":"gemini-3-pro-preview","source":"llm_config"}`
	if r.JSON() != want {
		t.Errorf("JSON parity:\n got=%s\nwant=%s", r.JSON(), want)
	}
}

func TestResolve_LLMConfigPhaseWithModelTier(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"phases": map[string]any{
			"auditor": map[string]any{"cli": "claude", "model_tier": "opus"},
		},
	})
	r, err := Resolve("auditor", Options{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.CLI != "claude" || r.ModelTier != "opus" || r.Source != "llm_config" {
		t.Errorf("bad result: %+v", r)
	}
	want := `{"cli":"claude","model_tier":"opus","source":"llm_config"}`
	if r.JSON() != want {
		t.Errorf("JSON parity:\n got=%s\nwant=%s", r.JSON(), want)
	}
}

func TestResolve_LLMConfigPhaseCLIOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"phases": map[string]any{
			"scout": map[string]any{"cli": "codex"},
		},
	})
	r, err := Resolve("scout", Options{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := `{"cli":"codex","model":"","source":"llm_config"}`
	if r.JSON() != want {
		t.Errorf("JSON parity:\n got=%s\nwant=%s", r.JSON(), want)
	}
}

func TestResolve_LLMConfigFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"_fallback": map[string]any{"cli": "claude", "model_tier": "haiku"},
	})
	r, err := Resolve("unmapped-role", Options{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := `{"cli":"claude","model_tier":"haiku","source":"llm_config_fallback"}`
	if r.JSON() != want {
		t.Errorf("JSON parity:\n got=%s\nwant=%s", r.JSON(), want)
	}
}

func TestResolve_LLMConfigFallbackDefaultsToSonnet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"_fallback": map[string]any{"cli": "claude"},
	})
	r, err := Resolve("any", Options{ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Cycle-124 followup: sentinel default migrated sonnet → balanced as part
	// of the abstract-vocabulary normalization. The realizer's fallback
	// ladder + parseManifest v1 shim keep legacy sonnet callers working.
	if r.ModelTier != "balanced" || r.Source != "llm_config_fallback" {
		t.Errorf("bad result: %+v", r)
	}
}

func TestResolve_InvalidJSONFallsThroughToProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("not json {{"), 0o644); err != nil {
		t.Fatal(err)
	}
	// also write a valid profile
	profDir := filepath.Join(dir, ".evolve", "profiles")
	writeJSON(t, filepath.Join(profDir, "scout.json"), map[string]any{
		"cli":                "claude",
		"model_tier_default": "haiku",
	})

	r, err := Resolve("scout", Options{ConfigPath: cfg, ProjectRoot: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Source != "profile" {
		t.Errorf("want source=profile, got %+v", r)
	}
	if r.ModelTier != "haiku" || r.CLI != "claude" {
		t.Errorf("bad profile read: %+v", r)
	}
}

func TestResolve_ProfileFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	profDir := filepath.Join(dir, ".evolve", "profiles")
	writeJSON(t, filepath.Join(profDir, "scout.json"), map[string]any{
		"cli":                "claude",
		"model_tier_default": "sonnet",
	})

	r, err := Resolve("scout", Options{
		ConfigPath:  filepath.Join(dir, "absent.json"),
		ProjectRoot: dir,
	})
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
	profDir := filepath.Join(dir, ".evolve", "profiles")
	writeJSON(t, filepath.Join(profDir, "scout.json"), map[string]any{
		"cli": "claude",
		// no model_tier_default — sentinel kicks in
	})

	r, err := Resolve("scout", Options{
		ConfigPath:  filepath.Join(dir, "absent.json"),
		ProjectRoot: dir,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Cycle-124 followup: profile-default sentinel migrated sonnet →
	// balanced as part of the abstract-vocabulary normalization.
	if r.ModelTier != "balanced" {
		t.Errorf("want balanced default, got %+v", r)
	}
}

func TestResolve_ProfileMissingCLI_IsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	profDir := filepath.Join(dir, ".evolve", "profiles")
	writeJSON(t, filepath.Join(profDir, "scout.json"), map[string]any{
		"model_tier_default": "sonnet",
	})
	_, err := Resolve("scout", Options{
		ConfigPath:  filepath.Join(dir, "absent.json"),
		ProjectRoot: dir,
	})
	if err == nil {
		t.Fatal("want error when profile.cli missing")
	}
}

func TestResolve_ProfileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := Resolve("nonexistent-role-xyz", Options{
		ConfigPath:  filepath.Join(dir, "absent.json"),
		ProjectRoot: dir,
		GitRoot:     dir,
	})
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

	r, err := Resolve("scout", Options{
		ConfigPath:  filepath.Join(project, "absent.json"),
		PluginRoot:  plugin,
		ProjectRoot: project,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// plugin wins
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
	r, err := Resolve("scout", Options{
		ConfigPath: filepath.Join(project, "absent.json"),
		Env:        env,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.CLI != "gemini" {
		t.Errorf("env-driven plugin root precedence broken: %+v", r)
	}
}

func TestResolve_DefaultConfigPathFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"phases": map[string]any{"scout": map[string]any{"cli": "claude", "model": "sonnet-4"}},
	})
	env := func(k string) string {
		if k == "EVOLVE_PROJECT_ROOT" {
			return dir
		}
		return ""
	}
	r, err := Resolve("scout", Options{Env: env})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Model != "sonnet-4" {
		t.Errorf("default config path resolution broken: %+v", r)
	}
}

func TestResolve_LLMConfigPhasePrecedesFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, ".evolve", "llm_config.json")
	writeJSON(t, cfg, map[string]any{
		"phases":    map[string]any{"scout": map[string]any{"cli": "codex", "model": "o1"}},
		"_fallback": map[string]any{"cli": "claude", "model_tier": "sonnet"},
	})
	r, _ := Resolve("scout", Options{ConfigPath: cfg})
	if r.Source != "llm_config" {
		t.Errorf("phase entry should win, got %+v", r)
	}
}

// TestResolve_ParityWithBash diffs Go output vs bash output across many
// scenarios. Skipped when bash or jq are not on PATH.
func TestResolve_ParityWithBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash parity skipped on windows")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not on PATH")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not on PATH")
	}

	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("git: %v", err)
	}
	repoRoot := strings.TrimSpace(string(out))
	script := filepath.Join(repoRoot, "legacy", "scripts", "dispatch", "resolve-llm.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("script missing")
	}

	cases := []struct {
		name      string
		llmConfig map[string]any
		profile   map[string]any
		role      string
	}{
		{
			"phase-exact-model",
			map[string]any{"phases": map[string]any{"scout": map[string]any{"cli": "gemini", "model": "gemini-3-pro-preview"}}},
			nil, "scout",
		},
		{
			"phase-tier-only",
			map[string]any{"phases": map[string]any{"auditor": map[string]any{"cli": "claude", "model_tier": "opus"}}},
			nil, "auditor",
		},
		{
			"fallback-block",
			map[string]any{"_fallback": map[string]any{"cli": "claude", "model_tier": "haiku"}},
			nil, "no-match",
		},
		{
			"profile-fallback",
			nil,
			map[string]any{"cli": "claude", "model_tier_default": "sonnet"},
			"scout",
		},
		{
			"profile-no-tier-defaults-sonnet",
			nil,
			map[string]any{"cli": "claude"},
			"scout",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "llm_config.json")
			if c.llmConfig != nil {
				writeJSON(t, cfgPath, c.llmConfig)
			}
			if c.profile != nil {
				writeJSON(t, filepath.Join(dir, ".evolve", "profiles", c.role+".json"), c.profile)
			}

			args := []string{script, c.role, cfgPath}
			cmd := exec.Command(bash, args...)
			cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+dir, "EVOLVE_PLUGIN_ROOT=")
			bashOut, berr := cmd.Output()
			bashStr := strings.TrimRight(string(bashOut), "\n")

			r, gerr := Resolve(c.role, Options{ConfigPath: cfgPath, ProjectRoot: dir})
			if berr == nil && gerr != nil {
				t.Fatalf("bash ok but Go err: %v (bash=%q)", gerr, bashStr)
			}
			if berr != nil && gerr == nil {
				t.Fatalf("Go ok (%s) but bash err: %v", r.JSON(), berr)
			}
			if berr == nil && gerr == nil {
				if r.JSON() != bashStr {
					t.Errorf("parity mismatch:\n bash=%s\n   go=%s", bashStr, r.JSON())
				}
			}
		})
	}
}
