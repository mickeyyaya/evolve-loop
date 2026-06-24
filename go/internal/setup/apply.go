package setup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Apply merges the chosen preset into an existing policy.json as per-phase pins
// and returns the new policy bytes. It is pure given its inputs (the cmd layer
// does the disk read/write), and it is the GATE that refuses to persist an
// illegal config: a degraded preset, a malformed existing policy, or a pin that
// breaches the floor all return an error with NO bytes.
//
// Merge semantics:
//   - Lossless: the existing policy is round-tripped through a raw map, so foreign
//     top-level keys (floor, cli_health, version, …) and foreign pins survive.
//   - Minimal: a pin is emitted ONLY where the assignment differs from the profile
//     default; a phase that matches its default has any stale Role pin removed
//     (so re-applying a lighter preset cleanly drops a heavier one's upgrades).
//   - Pins store the abstract TIER (fast|balanced|deep), never the native model id
//     — policy.ValidatePin ranks pin.Model via TierRank; a native id ranks 0 and
//     would silently skip the envelope floor.
//   - A phase with no profile is skipped (we never pin what we cannot validate).
func Apply(rep DetectReport, cfg PresetConfig, presetName string, existingPolicyJSON []byte, profLoader *profiles.Loader) ([]byte, error) {
	rr := Recommend(rep, cfg)
	var preset *Preset
	for i := range rr.Presets {
		if rr.Presets[i].Name == presetName {
			preset = &rr.Presets[i]
			break
		}
	}
	if preset == nil {
		return nil, fmt.Errorf("setup apply: unknown preset %q (have %s)", presetName, presetNamesOf(rr))
	}

	// Parse the existing policy losslessly (raw map — never the typed Policy,
	// which would reorder keys and drop any unmodeled future key).
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

	for _, a := range preset.Assignments {
		if a.Warning != "" {
			return nil, fmt.Errorf("setup apply: preset %q is degraded for phase %q (%s); refusing to write an unsatisfiable pin",
				presetName, a.Role, a.Warning)
		}
		prof, perr := profLoader.Get(a.Role)
		if perr != nil {
			continue // no profile → leave any existing pin untouched; never pin what we can't validate
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
