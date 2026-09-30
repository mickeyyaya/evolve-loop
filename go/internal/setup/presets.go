package setup

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PresetSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TierBias    string `json:"tier_bias"`
}

type PresetConfig struct {
	Default string       `json:"default"`
	Presets []PresetSpec `json:"presets"`
}

//go:embed presets.json
var presetsDefaultJSON []byte

var builtinPresets = mustBuiltinPresets()

func mustBuiltinPresets() PresetConfig {
	cfg, err := parsePresets(presetsDefaultJSON)
	if err != nil {
		panic("setup: embedded presets.json is invalid: " + err.Error())
	}
	return cfg
}

const presetOverrideFile = "setup-presets.json"

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
