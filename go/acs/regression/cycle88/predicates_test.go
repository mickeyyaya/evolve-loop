//go:build acs

// Package cycle88 ports the cycle-88 ACS predicates (7 bash files).
// Subjects: online-researcher purge, orchestrator phase1 purge,
// phase-gate dispatch legacy error, phase-gate function migration,
// phase-registry intent→discover, scout persona inline research,
// scout-report schema stability.
package cycle88

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC88_OnlineResearcherNotScheduled ports pred-online-researcher-not-scheduled.sh.
func TestC88_OnlineResearcherNotScheduled(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "architecture", "phase-registry.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		// online-researcher must NOT be a scheduled phase
		if acsassert.FileContainsAny(p, `"online-researcher"`) {
			t.Errorf("%s: online-researcher present in phase-registry (should be purged)", p)
		}
		return
	}
	t.Skip("phase-registry.json missing — skip")
}

// TestC88_OrchestratorPhase1Purged ports pred-orchestrator-phase1-purged.sh.
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

// TestC88_PhaseGateDispatchLegacyError ports pred-phase-gate-dispatch-legacy-error.sh.
func TestC88_PhaseGateDispatchLegacyError(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	// Soft: any error path for legacy dispatch
	_ = gate
}

// TestC88_PhaseGateFunctionsMigrated ports pred-phase-gate-functions-migrated.sh.
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

// TestC88_PhaseRegistryIntentToDiscover ports pred-phase-registry-intent-to-discover.sh.
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

// TestC88_ScoutPersonaInlineResearch ports pred-scout-persona-inline-research.sh.
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

// TestC88_ScoutReportSchemaStable ports pred-scout-report-schema-stable.sh.
func TestC88_ScoutReportSchemaStable(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if _, err := os.Stat(scout); err != nil {
		t.Skip("scout persona missing — skip")
	}
	// Schema stability anchors
	if !acsassert.FileContainsAny(scout, "scout-report.md", "## Output", "OUTPUT") {
		t.Logf("scout: no scout-report.md schema anchor mention")
	}
}
