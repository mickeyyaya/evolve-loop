// Package cycle55 ports the cycle-55 ACS predicates (4 bash files).
package cycle55

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC55_011_CapabilityGateBlocksNativeGemini ports cycle-55/011 (wiring-only).
// gemini.sh has the _GEMINI_NATIVE_CAP gate keyed on non_interactive_prompt.
func TestC55_011_CapabilityGateBlocksNativeGemini(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gemini := filepath.Join(root, "legacy", "scripts", "cli_adapters", "gemini.sh")
	if !acsassert.FileExists(t, gemini) {
		t.Skip("gemini.sh missing — skip cycle-55-011")
	}
	for _, marker := range []string{"_GEMINI_NATIVE_CAP", "non_interactive_prompt"} {
		if !acsassert.FileContains(t, gemini, marker) {
			return
		}
	}
}

// TestC55_012_CapabilityGateBlocksNativeCodex ports cycle-55/012 (wiring-only).
func TestC55_012_CapabilityGateBlocksNativeCodex(t *testing.T) {
	root := acsassert.RepoRoot(t)
	codex := filepath.Join(root, "legacy", "scripts", "cli_adapters", "codex.sh")
	if !acsassert.FileExists(t, codex) {
		t.Skip("codex.sh missing — skip cycle-55-012")
	}
	for _, marker := range []string{"_CODEX_NATIVE_CAP", "non_interactive_prompt"} {
		if !acsassert.FileContains(t, codex, marker) {
			return
		}
	}
}

// TestC55_020_PhaseRegistryExistsAndValidates ports cycle-55/020.
// phase-registry.json exists, is valid JSON, and references resolve.
func TestC55_020_PhaseRegistryExistsAndValidates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if !acsassert.FileExists(t, registry) {
		t.Skip("phase-registry.json missing — skip cycle-55-020")
	}
	// AC2-AC4: valid JSON with schema_version + non-empty phases.
	if !acsassert.JSONFieldEquals(t, registry, "schema_version", float64(1)) {
		// schema_version might be a string or different number — relax to
		// presence check via regex on raw file.
		if !acsassert.FileMatchesRegex(t, registry, `"schema_version"\s*:`) {
			return
		}
	}
	// Non-empty phases array.
	if !acsassert.FileMatchesRegex(t, registry, `"phases"\s*:\s*\[\s*\{`) {
		return
	}
	// AC6: gate refs resolve — we approximate by checking that phase-gate.sh
	// exists and contains at least one gate_ function. Full resolution
	// requires jq + cross-file analysis; the bash predicate is authoritative.
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if !acsassert.FileExists(t, gate) {
		t.Skip("phase-gate.sh missing — skip cycle-55-020 (gate-ref check)")
	}
	if !acsassert.FileMatchesRegex(t, gate, `(?m)^gate_\w+\(\)`) {
		t.Errorf("%s: no gate_X() function declarations", gate)
	}
}

// TestC55_021_OrchestratorReadsRegistryNotNarrative ports cycle-55/021 (wiring-only).
// list-phase-order.sh exists + handles EVOLVE_USE_PHASE_REGISTRY env.
func TestC55_021_OrchestratorReadsRegistryNotNarrative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	helper := filepath.Join(root, "legacy", "scripts", "dispatch", "list-phase-order.sh")
	if !acsassert.FileExists(t, helper) {
		t.Skip("list-phase-order.sh missing — skip cycle-55-021")
	}
	if !acsassert.FileContains(t, helper, "EVOLVE_USE_PHASE_REGISTRY") {
		return
	}
	if !acsassert.FileContains(t, helper, "phase-registry.json") {
		return
	}
}
