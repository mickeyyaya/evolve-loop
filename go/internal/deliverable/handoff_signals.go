package deliverable

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var signalsSentinel = regexp.MustCompile(`(?m)^<!--\s*evolve-signals:\s*(\{.*\})\s*-->\s*$`)

func declaredSignals(report []byte, declared []string) map[string]any {
	match := signalsSentinel.FindSubmatch(report)
	if match == nil {
		return nil
	}
	var reported map[string]any
	if err := json.Unmarshal(match[1], &reported); err != nil {
		return nil
	}
	kept := map[string]any{}
	for key, value := range reported {
		if slices.Contains(declared, key) {
			kept[key] = value
		}
	}
	return kept
}

func (h *HostEffects) writeHandoffSignals(c phasecontract.Contract, roots phasecontract.Roots) error {
	if h.spec == nil {
		return nil
	}
	spec, ok := h.spec(c.Phase)
	if !ok || len(spec.Outputs.Signals) == 0 {
		return nil
	}
	report, err := os.ReadFile(c.ArtifactPath(roots))
	if err != nil {
		return nil
	}
	signals := declaredSignals(report, spec.Outputs.Signals)
	if len(signals) == 0 {
		return nil
	}
	out, _ := json.Marshal(map[string]any{"signals": signals})
	return atomicwrite.Bytes(filepath.Join(roots.Workspace, "handoff-"+c.Phase+".json"), out)
}
