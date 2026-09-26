package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllPerCycleAgentsStrictlyCompact(t *testing.T) {
	root := repoRoot(t)
	perCycleAgents := []string{
		"evolve-scout",
		"evolve-builder",
		"evolve-auditor",
		"evolve-orchestrator",
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-intent",
	}
	for _, name := range perCycleAgents {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, "agents", name+".md"))
			if err != nil {
				t.Fatalf("read %s.md: %v", name, err)
			}
			_, body, err := ParseFrontmatter(string(raw))
			if err != nil {
				t.Fatalf("parse frontmatter for %s.md: %v", name, err)
			}
			stripped := StripOnDemandSections(body)
			if len(stripped) >= len(body) {
				t.Errorf("RED: %s.md is NOT compacted — StripOnDemandSections returned body unchanged (body=%d stripped=%d); add ## Reference Index heading with on-demand tail", name, len(body), len(stripped))
			}
		})
	}
}

func TestCompactionCoverage_MarkerlessBodyUnchanged_Negative(t *testing.T) {
	body := "# Agent\n\nOperational content.\n\nMore rules.\n"
	stripped := StripOnDemandSections(body)
	if stripped != body {
		t.Errorf("markerless body was modified by StripOnDemandSections (body=%d stripped=%d) — gate logic broken", len(body), len(stripped))
	}
}
