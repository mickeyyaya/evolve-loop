package prompts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot resolves the repository root from this test file's path.
// go/internal/prompts/ is three levels below the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// TestRealDocOnDemandStrip loads the actual shipped agent docs and asserts that
// StripOnDemandSections correctly removes the "## Reference Index (Layer 3, on-demand)"
// tail — the production heading that exact-equality matching previously missed.
// This is the regression guard for the strip-ondemand-heading-prefix-match fix.
func TestRealDocOnDemandStrip(t *testing.T) {
	root := repoRoot(t)
	agentsDir := filepath.Join(root, "agents")

	// Docs with a reference tail: must shrink by at least minSave bytes.
	mustStrip := []struct {
		name    string
		minSave int
	}{
		{"evolve-auditor", 4096},      // ~70 % tail (≈12 657 B)
		{"evolve-builder", 4096},      // ~35 % tail (≈6 944 B)
		{"evolve-scout", 2048},        // ~25 % tail (≈3 717 B)
		{"evolve-orchestrator", 512},  // ~5 % tail (≈993 B)
		{"evolve-tdd-engineer", 1500}, // cycle-415: predicate quality + failure modes tail
		{"evolve-triage", 1200},       // cycle-415: inbox pre-check algorithms + reflection tail
	}

	for _, tc := range mustStrip {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(agentsDir, tc.name+".md"))
			if err != nil {
				t.Fatalf("read %s.md: %v", tc.name, err)
			}
			_, body, err := ParseFrontmatter(string(raw))
			if err != nil {
				t.Fatalf("parse %s.md: %v", tc.name, err)
			}
			stripped := StripOnDemandSections(body)
			if strings.Contains(stripped, "## Reference Index") {
				t.Errorf("%s: stripped body still contains '## Reference Index'; heading not matched", tc.name)
			}
			if len(stripped) >= len(body) {
				t.Errorf("%s: strip did not shrink body (before=%d after=%d)", tc.name, len(body), len(stripped))
			}
			saved := len(body) - len(stripped)
			if saved < tc.minSave {
				t.Errorf("%s: saved only %d bytes (want ≥%d)", tc.name, saved, tc.minSave)
			}
		})
	}
}
