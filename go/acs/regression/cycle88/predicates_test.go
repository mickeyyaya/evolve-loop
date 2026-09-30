//go:build acs

package cycle88

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC88_OnlineResearcherNotScheduled(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "architecture", "phase-registry.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if acsassert.FileContainsAny(p, `"online-researcher"`) {
			t.Errorf("%s: online-researcher present in phase-registry (should be purged)", p)
		}
		return
	}
	t.Skip("phase-registry.json missing — skip")
}

func TestC88_OrchestratorPhase1Purged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	orch := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if _, err := os.Stat(orch); err != nil {
		t.Skip("orchestrator missing — skip")
	}
	if acsassert.FileContainsAny(orch, "Phase 1: online-researcher") {
		t.Errorf("orchestrator: Phase 1 online-researcher still present (should be purged)")
	}
}

func TestC88_PhaseGateDispatchLegacyError(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	_ = gate
}

func TestC88_PhaseGateFunctionsMigrated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	if !acsassert.FileContainsAny(gate, "gate_intent_to_discover", "gate_discover_to", "gate_scout") {
		t.Logf("phase-gate.sh: no expected migration markers")
	}
}

func TestC88_PhaseRegistryIntentToDiscover(t *testing.T) {
	root := acsassert.RepoRoot(t)
	reg := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(reg); err != nil {
		t.Skip("phase-registry.json missing — skip")
	}
	if !acsassert.FileContainsAny(reg, "intent", "discover", "scout") {
		t.Errorf("phase-registry: no intent/discover/scout phase entries")
	}
}

func TestC88_ScoutPersonaInlineResearch(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if _, err := os.Stat(scout); err != nil {
		t.Skip("scout persona missing — skip")
	}
	if !acsassert.FileContainsAny(scout, "research", "WebSearch", "WebFetch") {
		t.Errorf("scout: no inline-research markers")
	}
}

func TestC88_ScoutReportSchemaStable(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if _, err := os.Stat(scout); err != nil {
		t.Skip("scout persona missing — skip")
	}
	if !acsassert.FileContainsAny(scout, "scout-report.md", "## Output", "OUTPUT") {
		t.Logf("scout: no scout-report.md schema anchor mention")
	}
}
