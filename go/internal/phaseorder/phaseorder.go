// Package phaseorder ports legacy/scripts/dispatch/list-phase-order.sh.
//
// Reads docs/architecture/phase-registry.json and emits the phases in
// order. Falls back to a hardcoded canonical order when:
//   - EVOLVE_USE_PHASE_REGISTRY=0
//   - registry file is missing
//   - registry parse fails (and registry is enabled)
package phaseorder

import (
	"encoding/json"
	"fmt"
	"os"
)

// HardcodedOrder is the canonical phase sequence used as fallback.
var HardcodedOrder = []string{
	"intent", "scout", "triage", "plan-review",
	"tdd", "build-planner", "build", "tester",
	"audit", "ship", "retrospective", "memo",
}

// List returns the phase names in order. registryPath should be the
// absolute path to phase-registry.json. If useRegistry is false or
// registryPath does not exist, returns HardcodedOrder. Returns an
// error only when the registry exists but cannot be parsed (matches
// bash exit-code 1 contract).
func List(registryPath string, useRegistry bool) ([]string, error) {
	if !useRegistry {
		return HardcodedOrder, nil
	}
	if _, err := os.Stat(registryPath); err != nil {
		return HardcodedOrder, nil
	}
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", registryPath, err)
	}
	var doc struct {
		Phases []struct {
			Name string `json:"name"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", registryPath, err)
	}
	if len(doc.Phases) == 0 {
		return nil, fmt.Errorf("%s: phases array empty", registryPath)
	}
	out := make([]string, 0, len(doc.Phases))
	for _, p := range doc.Phases {
		if p.Name != "" {
			out = append(out, p.Name)
		}
	}
	return out, nil
}
