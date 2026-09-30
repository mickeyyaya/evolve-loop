package setup

import (
	"path/filepath"
	"testing"
)

func TestLoadPresets_EmbeddedDefault(t *testing.T) {
	cfg, err := LoadPresets(t.TempDir())
	if err != nil {
		t.Fatalf("LoadPresets default: %v", err)
	}
	if cfg.Default != "recommended" {
		t.Errorf("default = %q, want recommended", cfg.Default)
	}
	byName := map[string]string{}
	for _, p := range cfg.Presets {
		byName[p.Name] = p.TierBias
	}
	want := map[string]string{"recommended": "default", "economy": "down", "max-quality": "max"}
	for name, bias := range want {
		if byName[name] != bias {
			t.Errorf("preset %q bias = %q, want %q", name, byName[name], bias)
		}
	}
}

func TestLoadPresets_OverrideWins(t *testing.T) {
	evolveDir := t.TempDir()
	writeFile(t, filepath.Join(evolveDir, "setup-presets.json"), `{
	  "default": "thrifty",
	  "presets": [{"name":"thrifty","description":"min everywhere","tier_bias":"min"}]
	}`)
	cfg, err := LoadPresets(evolveDir)
	if err != nil {
		t.Fatalf("LoadPresets override: %v", err)
	}
	if cfg.Default != "thrifty" || len(cfg.Presets) != 1 || cfg.Presets[0].TierBias != "min" {
		t.Errorf("override not applied: %+v", cfg)
	}
}

func TestLoadPresets_MalformedOverride_Errors(t *testing.T) {
	evolveDir := t.TempDir()
	writeFile(t, filepath.Join(evolveDir, "setup-presets.json"), `{not json`)
	if _, err := LoadPresets(evolveDir); err == nil {
		t.Error("malformed override should error")
	}
}

func TestLoadPresets_InvalidDefaultName_Errors(t *testing.T) {
	evolveDir := t.TempDir()
	writeFile(t, filepath.Join(evolveDir, "setup-presets.json"), `{
	  "default": "ghost",
	  "presets": [{"name":"real","tier_bias":"default"}]
	}`)
	if _, err := LoadPresets(evolveDir); err == nil {
		t.Error("default naming a missing preset should error")
	}
}

func TestLoadPresets_EmptyPresets_Errors(t *testing.T) {
	evolveDir := t.TempDir()
	writeFile(t, filepath.Join(evolveDir, "setup-presets.json"), `{"default":"","presets":[]}`)
	if _, err := LoadPresets(evolveDir); err == nil {
		t.Error("empty presets should error")
	}
}

func TestLoadPresets_UnknownTierBias_Errors(t *testing.T) {
	evolveDir := t.TempDir()
	writeFile(t, filepath.Join(evolveDir, "setup-presets.json"), `{
	  "default":"x","presets":[{"name":"x","tier_bias":"bogus"}]
	}`)
	if _, err := LoadPresets(evolveDir); err == nil {
		t.Error("unknown tier_bias should be rejected at load")
	}
}

func TestBuiltinPresetsValid(t *testing.T) {
	cfg, err := LoadPresets("")
	if err != nil {
		t.Fatalf("embedded default invalid: %v", err)
	}
	if len(cfg.Presets) < 1 {
		t.Error("embedded default has no presets")
	}
}
