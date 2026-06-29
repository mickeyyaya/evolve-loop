package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

// compaction_coverage_test.go — RED contract for cycle-416 task prompt-compaction-coverage-gate.
//
// Regression guard: every per-cycle phase agent must have a ## Reference Index heading so
// StripOnDemandSections fires on every cycle dispatch. Fails loudly naming any agent whose
// marker is missing or whose body is not strictly shortened.
//
// RED state (before builder): evolve-intent.md has no ## Reference Index heading;
// StripOnDemandSections returns its body unchanged — len(stripped) == len(body) → FAIL for intent.

// TestAllPerCycleAgentsStrictlyCompact asserts that every per-cycle phase agent's prompt body
// is strictly shortened by StripOnDemandSections (marker present + tail non-empty).
// A regression: if any agent loses its ## Reference Index heading, this test fails loudly
// naming the specific agent, preventing silent per-cycle token re-inflation.
// RED: evolve-intent has no heading → stripped==body → len(stripped) < len(body) is false → FAIL.
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

// TestCompactionCoverage_MarkerlessBodyUnchanged_Negative asserts that a body with no
// ## Reference Index heading is returned byte-for-byte unchanged by StripOnDemandSections.
// This confirms the gate logic: only bodies WITH a marker are shortened; a markerless body
// reaching this function means the agent was not updated — the test above will catch it.
// Pre-existing GREEN: StripOnDemandSections already handles this correctly.
func TestCompactionCoverage_MarkerlessBodyUnchanged_Negative(t *testing.T) {
	body := "# Agent\n\nOperational content.\n\nMore rules.\n"
	stripped := StripOnDemandSections(body)
	if stripped != body {
		t.Errorf("markerless body was modified by StripOnDemandSections (body=%d stripped=%d) — gate logic broken", len(body), len(stripped))
	}
}
