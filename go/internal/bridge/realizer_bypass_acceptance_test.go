package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRealizeFor_ClaudeProjectScopeRedeclaresTheBypassAcceptance(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	r := RealizeFor("claude-tmux", LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SettingsScope: "project", SessionMode: "ephemeral"})
	raw, ok := flagValue(r.LaunchFlags, "--settings")
	if !ok {
		t.Fatalf("a project-scoped claude seat drops the user settings that hold the bypass acceptance, so claude 2.1.316 stops on its bypass warning; want --settings in %v", r.LaunchFlags)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("--settings value %q is not JSON: %v", raw, err)
	}
	if settings["skipDangerousModePermissionPrompt"] != true {
		t.Fatalf("--settings %q must set skipDangerousModePermissionPrompt true", raw)
	}
	if v, _ := flagValue(r.LaunchFlags, "--setting-sources"); v != "project" {
		t.Fatalf("the user settings stay excluded: --setting-sources = %q, want project", v)
	}
}

func TestRealizeFor_ClaudeWithoutProjectScopeAddsNoFlagSettings(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	r := RealizeFor("claude-tmux", LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SessionMode: "ephemeral"})
	if containsToken(r.LaunchFlags, "--settings") {
		t.Fatalf("a seat that loads the user settings already has the acceptance; got %v", r.LaunchFlags)
	}
}

func TestLoadProfile_SettingsScopeReachesTheLaunchIntent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(path, []byte(`{"name":"router","settings_scope":"project"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prof, err := LoadProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := launchIntentFor(&Config{Model: "deep"}, prof, policy.EffortTable{}).SettingsScope; got != "project" {
		t.Fatalf("a profile's settings_scope must reach the launch intent; got %q", got)
	}
}

func TestRepoProfiles_DeclareTheSettingsScopeInsteadOfCopyingItsFlags(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", ".evolve", "profiles", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no repo profiles found: %v", err)
	}
	m, err := loadManifestRaw("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	scopes := m.Params["settings_scope"].Values
	for _, path := range paths {
		var prof struct {
			SettingsScope   string              `json:"settings_scope"`
			ExtraFlagsByCLI map[string][]string `json:"extra_flags_by_cli"`
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &prof); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for cli, flags := range prof.ExtraFlagsByCLI {
			if slices.Contains(flags, "--setting-sources") || slices.Contains(flags, "--settings") {
				t.Errorf("%s: extra_flags_by_cli[%s] copies a settings flag; declare \"settings_scope\" so the manifest owns the flags", filepath.Base(path), cli)
			}
		}
		if _, known := scopes[prof.SettingsScope]; prof.SettingsScope != "" && !known {
			t.Errorf("%s: settings_scope %q is not a scope of the claude-tmux manifest, so it realizes no flags", filepath.Base(path), prof.SettingsScope)
		}
		if slices.Contains(prof.ExtraFlagsByCLI["claude-tmux"], "--disable-slash-commands") && prof.SettingsScope != "project" {
			t.Errorf("%s: a clean-boot claude seat must declare settings_scope project, or the user settings reach the phase agent", filepath.Base(path))
		}
	}
}
