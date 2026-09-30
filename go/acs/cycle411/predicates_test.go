//go:build acs

package cycle411

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func countHeaderLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}
	return count
}

func countCodeSpanLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.ContainsRune(line, '`') {
			count++
		}
	}
	return count
}

func TestC411_001_AuditorByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-auditor.md: %v", err)
	}
	const maxBytes = 18815
	if len(raw) > maxBytes {
		t.Errorf("RED: agents/evolve-auditor.md has %d bytes — must be < 18816 (≥15%% cut from baseline 22137).\n"+
			"Builder must apply TSC: remove grammar glue (articles/auxiliaries/fillers/modal-padding)\n"+
			"while preserving all section headers, backtick spans, and gate anchors.\n"+
			"An unmodified file or marker-only edit cannot satisfy this predicate.",
			len(raw))
	}
}

// acs-predicate: config-check
func TestC411_002_AuditorTSCMarkerPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	if !acsassert.FileContains(t, path, "<!-- TSC applied") {
		t.Errorf("RED: agents/evolve-auditor.md is missing the TSC marker.\n" +
			"Builder must add: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// acs-predicate: config-check
func TestC411_003_AuditorHeaderCountExact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-auditor.md: %v", err)
	}
	got := countHeaderLines(string(raw))
	const want = 25
	if got != want {
		t.Errorf("agents/evolve-auditor.md has %d '## ' headers — expected exactly %d.\n"+
			"TSC must not remove section headers; only prose glue within sections may be deleted.",
			got, want)
	}
}

func TestC411_004_AuditorCodeSpanLinesFloor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-auditor.md: %v", err)
	}
	got := countCodeSpanLines(string(raw))
	const minLines = 74
	if got < minLines {
		t.Errorf("agents/evolve-auditor.md has %d code-span lines — must be ≥%d.\n"+
			"TSC must preserve code examples; deleting them to reduce byte count is forbidden.",
			got, minLines)
	}
}

// acs-predicate: config-check
func TestC411_005_AuditorGateAnchorsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	anchors := []string{
		"challenge-token",
		"EGPS Verdict Computation",
		"STOP CRITERION",
		"handoff-auditor.json",
		"acs-verdict.json",
	}
	for _, anchor := range anchors {
		if !acsassert.FileContains(t, path, anchor) {
			t.Errorf("gate anchor %q was removed from agents/evolve-auditor.md — TSC must preserve all gate-critical anchors", anchor)
		}
	}
}

func TestC411_006_TDDByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tdd-engineer.md: %v", err)
	}
	const maxBytes = 21711
	if len(raw) > maxBytes {
		t.Errorf("RED: agents/evolve-tdd-engineer.md has %d bytes — must be < 21712 (≥15%% cut from baseline 25544).\n"+
			"Builder must apply TSC: remove grammar glue while preserving all section headers,\n"+
			"backtick spans, and EGPS/ACS anchors.",
			len(raw))
	}
}

// acs-predicate: config-check
func TestC411_007_TDDTSCMarkerPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	if !acsassert.FileContains(t, path, "<!-- TSC applied") {
		t.Errorf("RED: agents/evolve-tdd-engineer.md is missing the TSC marker.\n" +
			"Builder must add: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// acs-predicate: config-check
func TestC411_008_TDDHeaderCountExact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tdd-engineer.md: %v", err)
	}
	got := countHeaderLines(string(raw))
	const want = 17
	if got != want {
		t.Errorf("agents/evolve-tdd-engineer.md has %d '## ' headers — expected exactly %d.\n"+
			"TSC must not remove section headers.", got, want)
	}
}

func TestC411_009_TDDCodeSpanLinesFloor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tdd-engineer.md: %v", err)
	}
	got := countCodeSpanLines(string(raw))
	const minLines = 85
	if got < minLines {
		t.Errorf("agents/evolve-tdd-engineer.md has %d code-span lines — must be ≥%d.\n"+
			"TSC must preserve code examples; deletion to reduce byte count is forbidden.",
			got, minLines)
	}
}

// acs-predicate: config-check
func TestC411_010_TDDACSAnchorsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	anchors := []string{
		"EGPS",
		"predicates_test.go",
		"go:build acs",
	}
	for _, anchor := range anchors {
		if !acsassert.FileContains(t, path, anchor) {
			t.Errorf("EGPS/ACS anchor %q was removed from agents/evolve-tdd-engineer.md — TSC must preserve these anchors", anchor)
		}
	}
}

func TestC411_011_OrchestratorByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-orchestrator.md: %v", err)
	}
	const maxBytes = 20944
	if len(raw) > maxBytes {
		t.Errorf("RED: agents/evolve-orchestrator.md has %d bytes — must be < 20945 (≥15%% cut from baseline 24642).\n"+
			"Builder must apply TSC: remove grammar glue while preserving all section headers,\n"+
			"backtick spans, and phase/guard anchors.",
			len(raw))
	}
}

// acs-predicate: config-check
func TestC411_012_OrchestratorTSCMarkerPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if !acsassert.FileContains(t, path, "<!-- TSC applied") {
		t.Errorf("RED: agents/evolve-orchestrator.md is missing the TSC marker.\n" +
			"Builder must add: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// acs-predicate: config-check
func TestC411_013_OrchestratorHeaderCountExact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-orchestrator.md: %v", err)
	}
	got := countHeaderLines(string(raw))
	const want = 26
	if got != want {
		t.Errorf("agents/evolve-orchestrator.md has %d '## ' headers — expected exactly %d.\n"+
			"TSC must not remove section headers.", got, want)
	}
}

func TestC411_014_OrchestratorCodeSpanLinesFloor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-orchestrator.md: %v", err)
	}
	got := countCodeSpanLines(string(raw))
	const minLines = 99
	if got < minLines {
		t.Errorf("agents/evolve-orchestrator.md has %d code-span lines — must be ≥%d.\n"+
			"TSC must preserve code examples; deletion to reduce byte count is forbidden.",
			got, minLines)
	}
}

// acs-predicate: config-check
func TestC411_015_OrchestratorPhaseGuardAnchorsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	anchors := []string{
		"STOP CRITERION",
		"Phase Loop",
		"evolve guard phase",
		"acs-verdict.json",
		"phase-gate-precondition",
	}
	for _, anchor := range anchors {
		if !acsassert.FileContains(t, path, anchor) {
			t.Errorf("phase/guard anchor %q was removed from agents/evolve-orchestrator.md — TSC must preserve these anchors", anchor)
		}
	}
}
