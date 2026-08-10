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
		// Auditor floor recalibrated 2026-08-10: the old 4096 floor ("~70 % tail")
		// fossilized a mid-file marker that stripped the verdict rules, STOP
		// criterion, and MANDATORY disposition contract from every dispatched
		// audit (cycles 1390-1429, 15/30 FAILs). The marker now sits at EOF; what
		// may be stripped is governed by phasecoherence/persona_strip_operational_test.go.
		{"evolve-auditor", 256},
		// Builder/scout/tdd/triage floors recalibrated 2026-08-10 alongside the
		// auditor's (persona-strip lobotomy incident): the old floors fossilized
		// operational tails (STOP CRITERION, POSTHOC, predicate-quality rules,
		// inbox ingestion) below mid-file markers. Markers now sit at EOF; the
		// keep-guard phasecoherence/persona_strip_operational_test.go governs
		// what may be stripped.
		{"evolve-builder", 256},
		{"evolve-scout", 256},
		{"evolve-orchestrator", 512}, // ~5 % tail (≈993 B)
		{"evolve-tdd-engineer", 64},
		{"evolve-triage", 64},
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
