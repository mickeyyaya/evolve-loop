package setup

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PresetSpec is one named preset's definition from the public preset config
// (presets.json / .evolve/setup-presets.json). TierBias is a generic strategy
// the recommender interprets — "default" (profile default), "down" (one tier
// cheaper), "up" (one tier richer), "min"/"max" (envelope floor/ceiling) — so
// preset behavior is data, never hardcoded in Go.
type PresetSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TierBias    string `json:"tier_bias"`
}

// PresetConfig is the public, exposed preset-definition file. Default names the
// preset the UI pre-selects.
type PresetConfig struct {
	Default string       `json:"default"`
	Presets []PresetSpec `json:"presets"`
}

//go:embed presets.json
var presetsDefaultJSON []byte

// builtinPresets is the shipped default, parsed once at init. A malformed
// embedded default is a build-time programming error, so it panics (fail loud).
var builtinPresets = mustBuiltinPresets()

func mustBuiltinPresets() PresetConfig {
	cfg, err := parsePresets(presetsDefaultJSON)
	if err != nil {
		panic("setup: embedded presets.json is invalid: " + err.Error())
	}
	return cfg
}

// presetOverrideFile is the per-repo override the user may drop to customize
// presets without editing the shipped default.
const presetOverrideFile = "setup-presets.json"

// LoadPresets resolves the active preset config: the per-repo override
// .evolve/setup-presets.json when present + valid, else the shipped default.
// A present-but-malformed/invalid override is an error (never silently ignored).
func LoadPresets(evolveDir string) (PresetConfig, error) {
	if evolveDir == "" {
		return builtinPresets, nil
	}
	path := filepath.Join(evolveDir, presetOverrideFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return builtinPresets, nil
		}
		return PresetConfig{}, fmt.Errorf("setup presets: reading %s: %w", path, err)
	}
	cfg, perr := parsePresets(b)
	if perr != nil {
		return PresetConfig{}, fmt.Errorf("setup presets: %s: %w", path, perr)
	}
	return cfg, nil
}

// parsePresets unmarshals + validates a preset config (shared by the embedded
// default and the override path).
func parsePresets(b []byte) (PresetConfig, error) {
	var cfg PresetConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return PresetConfig{}, fmt.Errorf("malformed preset JSON: %w", err)
	}
	if err := validatePresetConfig(cfg); err != nil {
		return PresetConfig{}, err
	}
	return cfg, nil
}

// knownTierBias is the generic strategy vocabulary the recommender interprets.
// Empty means "default" (profile default). An override using anything else is a
// typo and is rejected at load rather than silently treated as "default".
var knownTierBias = map[string]bool{
	"": true, "default": true, "down": true, "up": true, "min": true, "max": true,
}

func validatePresetConfig(cfg PresetConfig) error {
	if len(cfg.Presets) == 0 {
		return fmt.Errorf("no presets defined")
	}
	names := map[string]bool{}
	for _, p := range cfg.Presets {
		if p.Name == "" {
			return fmt.Errorf("preset with empty name")
		}
		if !knownTierBias[p.TierBias] {
			return fmt.Errorf("preset %q: unknown tier_bias %q (want default|down|up|min|max)", p.Name, p.TierBias)
		}
		names[p.Name] = true
	}
	if cfg.Default != "" && !names[cfg.Default] {
		return fmt.Errorf("default %q names no defined preset", cfg.Default)
	}
	return nil
}
