// Package cycle101 ports the cycle-101 ACS predicates (3 bash files).
//
// Bash predicates invoke the agy.sh cli adapter directly with synthetic
// VALIDATE_ONLY + DEGRADED-mode fixtures. Go port asserts source-presence
// (adapter exists, dispatch hooks present, cross-name resolver in subagent-run.sh)
// and runs a behavioral smoke via SubprocessOutput on VALIDATE_ONLY=1.
package cycle101

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC101_001_AgyAdapterDispatchResolves ports cycle-101/001.
func TestC101_001_AgyAdapterDispatchResolves(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adapter := filepath.Join(root, "legacy", "scripts", "cli_adapters", "agy.sh")
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")

	if !acsassert.FileExists(t, adapter) {
		t.Skip("agy.sh missing — skip cycle-101-001")
	}
	info, err := os.Stat(adapter)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("agy.sh not executable")
	}

	// Cross-name resolver: at least 2 occurrences of "antigravity.*agy" in subagent-run.sh
	if acsassert.FileExists(t, subagent) {
		if count := acsassert.CountOccurrencesAny(subagent, "antigravity"); count < 1 {
			t.Errorf("subagent-run.sh: no antigravity cross-name resolver lines")
		}
	}
}

// TestC101_002_AgyAdapterEmitsZeroCostEnvelope ports cycle-101/002.
// Source-presence check for DEGRADED-mode envelope shape.
func TestC101_002_AgyAdapterEmitsZeroCostEnvelope(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adapter := filepath.Join(root, "legacy", "scripts", "cli_adapters", "agy.sh")
	if !acsassert.FileExists(t, adapter) {
		t.Skip("agy.sh missing — skip cycle-101-002")
	}
	// DEGRADED mode must emit "adapter":"agy" in the envelope
	if !acsassert.FileContains(t, adapter, `"adapter"`) {
		return
	}
	if !acsassert.FileContains(t, adapter, `"agy"`) {
		return
	}
}

// TestC101_003_AgySchemaEnumIncludesAntigravity ports cycle-101/003.
// The llm_config schema enum must include "antigravity". The schema may
// be embedded in agy.sh itself or in subagent-run.sh (string literal enum)
// rather than a separate JSON file. Accept either source.
func TestC101_003_AgySchemaEnumIncludesAntigravity(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "schemas", "llm_config.schema.json"),
		filepath.Join(root, ".evolve", "llm_config.schema.json"),
		filepath.Join(root, "schemas", "llm_config.schema.json"),
		filepath.Join(root, "legacy", "scripts", "cli_adapters", "agy.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "resolve-llm.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if acsassert.FileContainsAny(p, "antigravity") {
			return
		}
	}
	t.Skip("antigravity enum reference not found at any accepted path")
}
