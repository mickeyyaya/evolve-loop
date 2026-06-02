//go:build legacy

// Package cycle61 ports the cycle-61 ACS predicates (1 bash file).
package cycle61

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC61_043_GeminiNativeMode ports cycle-61/043.
// gemini.capabilities.json has non_interactive_prompt: true; gemini.sh
// has the native-invocation log line.
func TestC61_043_GeminiNativeMode(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cap := filepath.Join(root, "legacy", "scripts", "cli_adapters", "gemini.capabilities.json")
	sh := filepath.Join(root, "legacy", "scripts", "cli_adapters", "gemini.sh")
	if !fixtures.FilePresent(cap) {
		t.Skip("gemini.capabilities.json missing — skip cycle-61-043")
	}
	if !fixtures.FilePresent(sh) {
		t.Skip("gemini.sh missing — skip cycle-61-043")
	}
	if !acsassert.FileContains(t, cap, `"non_interactive_prompt": true`) {
		return
	}
	if !acsassert.FileContains(t, sh, "invoking gemini binary directly") {
		return
	}
}
