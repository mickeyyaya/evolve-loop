package setup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func Apply(rep DetectReport, cfg PresetConfig, presetName string, existingPolicyJSON []byte, profLoader *profiles.Loader) ([]byte, error) {
	preset, err := findPreset(Recommend(rep, cfg), presetName)
	if err != nil {
		return nil, err
	}
	obj, pins, err := parseExistingPolicy(existingPolicyJSON)
	if err != nil {
		return nil, err
	}

	for _, a := range preset.Assignments {
		if a.Warning != "" {
			return nil, fmt.Errorf("setup apply: preset %q is degraded for phase %q (%s); refusing to write an unsatisfiable pin",
				presetName, a.Role, a.Warning)
		}
		prof, perr := profLoader.Get(a.Role)
		if perr != nil {
			continue
		}
		if a.DiffersFromDefault {
			pin := policy.Pin{CLI: a.CLI, Model: a.Tier}
			if verr := policy.ValidatePin(a.Role, pin, &prof); verr != nil {
				return nil, fmt.Errorf("setup apply: emitted pin for %q breaches floor: %w", a.Role, verr)
			}
			pins[a.Role] = pin
		} else {
			delete(pins, a.Role)
		}
	}

	return encodePolicy(obj, pins)
}

func findPreset(rr RecommendReport, presetName string) (*Preset, error) {
	for i := range rr.Presets {
		if rr.Presets[i].Name == presetName {
			return &rr.Presets[i], nil
		}
	}
	return nil, fmt.Errorf("setup apply: unknown preset %q (have %s)", presetName, presetNamesOf(rr))
}

func parseExistingPolicy(existingPolicyJSON []byte) (map[string]json.RawMessage, map[string]policy.Pin, error) {
	obj := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(existingPolicyJSON))) > 0 {
		if err := json.Unmarshal(existingPolicyJSON, &obj); err != nil {
			return nil, nil, fmt.Errorf("setup apply: existing policy.json is malformed (%w); refusing to clobber", err)
		}
	}
	pins := map[string]policy.Pin{}
	if raw, ok := obj["pins"]; ok {
		if err := json.Unmarshal(raw, &pins); err != nil {
			return nil, nil, fmt.Errorf("setup apply: existing policy.json pins block is malformed (%w); refusing to clobber", err)
		}
	}
	return obj, pins, nil
}

func encodePolicy(obj map[string]json.RawMessage, pins map[string]policy.Pin) ([]byte, error) {
	if len(pins) == 0 {
		delete(obj, "pins")
	} else {
		pinsRaw, err := json.Marshal(pins)
		if err != nil {
			return nil, fmt.Errorf("setup apply: encoding pins: %w", err)
		}
		obj["pins"] = pinsRaw
	}
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("setup apply: encoding policy: %w", err)
	}
	return append(out, '\n'), nil
}

func presetNamesOf(rr RecommendReport) string {
	names := make([]string, 0, len(rr.Presets))
	for _, p := range rr.Presets {
		names = append(names, p.Name)
	}
	return strings.Join(names, "|")
}
