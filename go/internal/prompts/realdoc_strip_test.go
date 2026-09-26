package prompts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func TestRealDocOnDemandStrip(t *testing.T) {
	root := repoRoot(t)
	agentsDir := filepath.Join(root, "agents")

	// The floors only prove each marker section exists; what may be stripped is
	// governed by phasecoherence/persona_strip_operational_test.go.
	mustStrip := []struct {
		name    string
		minSave int
	}{
		{"evolve-auditor", 256},
		{"evolve-builder", 256},
		{"evolve-scout", 256},
		{"evolve-orchestrator", 512},
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
