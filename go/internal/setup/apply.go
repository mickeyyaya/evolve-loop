package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var ErrRoutingTableDeclared = errors.New("this project declares a cli_routing table, which owns every route, so setup presets do not apply: inspect the routes with `evolve cli-routing show` and change one with `evolve cli-routing set agents.<role> <clis> [--model <tier>]`")

func Apply(rep DetectReport, cfg PresetConfig, presetName string, existingPolicyJSON []byte, profLoader *profiles.Loader) ([]byte, error) {
	if declaresRoutingTable(existingPolicyJSON) {
		return nil, fmt.Errorf("setup apply: %w", ErrRoutingTableDeclared)
	}
	preset, err := findPreset(Recommend(rep, cfg), presetName)
	if err != nil {
		return nil, err
	}
	pins, err := parseExistingPins(existingPolicyJSON)
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
		if !a.DiffersFromDefault {
			delete(pins, a.Role)
			continue
		}
		pin := policy.Pin{CLI: a.CLI, Model: a.Tier}
		if verr := policy.ValidatePin(a.Role, pin, &prof); verr != nil {
			return nil, fmt.Errorf("setup apply: emitted pin for %q breaches floor: %w", a.Role, verr)
		}
		pins[a.Role] = pin
	}
	return policy.PatchBlocks(existingPolicyJSON, map[string]any{"pins": pinsBlock(pins)})
}

func pinsBlock(pins map[string]policy.Pin) any {
	if len(pins) == 0 {
		return nil
	}
	return pins
}

func findPreset(rr RecommendReport, presetName string) (*Preset, error) {
	for i := range rr.Presets {
		if rr.Presets[i].Name == presetName {
			return &rr.Presets[i], nil
		}
	}
	return nil, fmt.Errorf("setup apply: unknown preset %q (have %s)", presetName, presetNamesOf(rr))
}

func parseExistingPins(existingPolicyJSON []byte) (map[string]policy.Pin, error) {
	obj := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(existingPolicyJSON))) > 0 {
		if err := json.Unmarshal(existingPolicyJSON, &obj); err != nil {
			return nil, fmt.Errorf("setup apply: existing policy.json is malformed (%w); refusing to clobber", err)
		}
	}
	pins := map[string]policy.Pin{}
	if raw, ok := obj["pins"]; ok {
		if err := json.Unmarshal(raw, &pins); err != nil {
			return nil, fmt.Errorf("setup apply: existing policy.json pins block is malformed (%w); refusing to clobber", err)
		}
	}
	return pins, nil
}

func presetNamesOf(rr RecommendReport) string {
	names := make([]string, 0, len(rr.Presets))
	for _, p := range rr.Presets {
		names = append(names, p.Name)
	}
	return strings.Join(names, "|")
}

func declaresRoutingTable(policyJSON []byte) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(policyJSON, &top) != nil {
		return false
	}
	for key := range top {
		if strings.EqualFold(key, "cli_routing") {
			return true
		}
	}
	return false
}
