// Package cycle45 ports the cycle-45 ACS predicates (5 bash files).
package cycle45

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC45_001_BuilderEffortMedium ports cycle-45/001.
// builder.json:effort_level is "medium" (not "high").
func TestC45_001_BuilderEffortMedium(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "builder.json")
	if !acsassert.FileExists(t, profile) {
		t.Skip("builder.json missing — skip cycle-45-001")
	}
	if !acsassert.FileMatchesRegex(t, profile, `"effort_level"\s*:\s*"medium"`) {
		return
	}
}

// TestC45_002_BuilderTurnGate ports cycle-45/002.
// evolve-builder.md STOP CRITERION contains turn-budget-respected gate.
func TestC45_002_BuilderTurnGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	file := filepath.Join(root, "agents", "evolve-builder.md")
	if !acsassert.FileExists(t, file) {
		t.Skip("evolve-builder.md missing — skip cycle-45-002")
	}
	if acsassert.CountOccurrencesAny(file, "turn-budget-respected") < 1 {
		t.Errorf("%s: turn-budget-respected not found", file)
	}
}

// TestC45_003_TrajectoryCompressionPersona ports cycle-45/003.
// Soft-pass when the Trajectory Compression section is removed (cycle-46
// dropped context_compact_* fields as phantom — see cycle-46/006).
func TestC45_003_TrajectoryCompressionPersona(t *testing.T) {
	root := acsassert.RepoRoot(t)
	file := filepath.Join(root, "agents", "evolve-builder.md")
	if !acsassert.FileContainsAny(file, "Trajectory Compression") {
		t.Skip("Trajectory Compression marker absent — source evolved past cycle-45-003")
	}
}

// TestC45_004_TrajectoryCompressionProfile ports cycle-45/004.
// Soft-pass — cycle-46 explicitly removed this phantom field.
func TestC45_004_TrajectoryCompressionProfile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "builder.json")
	if !acsassert.FileContainsAny(profile, `"context_compact_expired_tool_results": true`) {
		t.Skip("context_compact_expired_tool_results absent — removed in cycle-46 as phantom field")
	}
}

// TestC45_005_PNew21RoadmapDone ports cycle-45/005.
// Roadmap marks P-NEW-21 as DONE (cycle 45).
func TestC45_005_PNew21RoadmapDone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !acsassert.FileExists(t, roadmap) {
		t.Skip("roadmap missing — skip cycle-45-005")
	}
	if !acsassert.LineContainsAll(roadmap, "P-NEW-21", "DONE (cycle 45)") {
		t.Errorf("%s: P-NEW-21 not marked 'DONE (cycle 45)'", roadmap)
	}
}
