// Package phaseorder lists the pipeline phases in order from the phase registry, with a hardcoded fallback.
// See docs/architecture/packages/internal-phaseorder.md.
package phaseorder

import (
	"encoding/json"
	"fmt"
	"os"
)

var HardcodedOrder = []string{
	"intent", "scout", "triage", "plan-review",
	"tdd", "build-planner", "build", "tester",
	"audit", "ship", "retrospective", "memo",
}

func List(registryPath string) ([]string, error) {
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
