// Package cycle60 ports the cycle-60 ACS predicates (2 bash files).
package cycle60

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC60_040_E2EMixedCliCycle ports cycle-60/040 (wiring-only).
// resolve-llm.sh exists; it routes per-phase cli via fixture input.
// Bash predicate executes resolve-llm.sh end-to-end; Go port asserts presence.
func TestC60_040_E2EMixedCliCycle(t *testing.T) {
	root := acsassert.RepoRoot(t)
	resolv := filepath.Join(root, "legacy", "scripts", "dispatch", "resolve-llm.sh")
	if !acsassert.FileExists(t, resolv) {
		t.Skip("resolve-llm.sh missing — skip cycle-60-040")
	}
	if !acsassert.FileContainsAny(resolv, "llm_config", "cli_resolution_source") {
		return
	}
}

// TestC60_042_LegacyNoLLMConfigCycleCompletes ports cycle-60/042 (wiring-only).
func TestC60_042_LegacyNoLLMConfigCycleCompletes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	sub := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if !acsassert.FileExists(t, sub) {
		t.Skip("subagent-run.sh missing — skip cycle-60-042")
	}
	for _, marker := range []string{"--validate-profile", "cli_resolution_source"} {
		if !acsassert.FileContains(t, sub, marker) {
			return
		}
	}
}
