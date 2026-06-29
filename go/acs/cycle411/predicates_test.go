//go:build acs

// Package cycle411 materializes the cycle-411 acceptance criteria for three TSC tasks:
//   - tsc-compress-auditor-prompt (agents/evolve-auditor.md, baseline 22137 bytes)
//   - tsc-compress-tdd-engineer-prompt (agents/evolve-tdd-engineer.md, baseline 25544 bytes)
//   - tsc-compress-orchestrator-prompt (agents/evolve-orchestrator.md, baseline 24642 bytes)
//
// Telegraphic Semantic Compression (TSC) removes grammar glue while preserving every
// section header, code-span line, and gate anchor.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	tsc-compress-auditor-prompt:
//	  AC1 bytes < 18816 (≥15% cut from 22137)                    → C411_001 (RED)
//	  AC2 TSC marker present (<!-- TSC applied)                   → C411_002 (RED)
//	  AC3 header count == 25 exact (anti-gaming: no deletion)     → C411_003 (pre-existing GREEN, config-check)
//	  AC4 code-span lines ≥ 74 (anti-gaming floor, ~90% of 82)   → C411_004 (pre-existing GREEN)
//	  AC5 5 gate anchors intact                                   → C411_005 (pre-existing GREEN, config-check)
//
//	tsc-compress-tdd-engineer-prompt:
//	  AC1 bytes < 21712 (≥15% cut from 25544)                    → C411_006 (RED)
//	  AC2 TSC marker present                                      → C411_007 (RED)
//	  AC3 header count == 17 exact                                → C411_008 (pre-existing GREEN, config-check)
//	  AC4 code-span lines ≥ 85 (anti-gaming floor, ~89% of 95)   → C411_009 (pre-existing GREEN)
//	  AC5 3 EGPS/ACS anchors intact                              → C411_010 (pre-existing GREEN, config-check)
//
//	tsc-compress-orchestrator-prompt:
//	  AC1 bytes < 20945 (≥15% cut from 24642)                    → C411_011 (RED)
//	  AC2 TSC marker present                                      → C411_012 (RED)
//	  AC3 header count == 26 exact                                → C411_013 (pre-existing GREEN, config-check)
//	  AC4 code-span lines ≥ 99 (anti-gaming floor, ~90% of 110)  → C411_014 (pre-existing GREEN)
//	  AC5 phase/guard anchors intact                             → C411_015 (pre-existing GREEN, config-check)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: unmodified file (above byte threshold) → C411_001/006/011 (RED)
//	Edge/OOD: file with code examples deleted → C411_004/009/014 (code-span floor)
//	Semantic:  TSC marker absent vs byte count are distinct failure modes
//
// Deferred (zero predicates per R9.3): D1 CompactPrompts strip path,
// D2 report-size tightening, D3 router/sidecars TSC.
package cycle411

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// countHeaderLines returns the number of lines starting with "## " (matching grep -c '^## ').
func countHeaderLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			count++
		}
	}
	return count
}

// countCodeSpanLines returns the number of lines containing at least one backtick.
// This matches the scout-report baseline metric: grep -c '`' file = 82/95/110.
func countCodeSpanLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.ContainsRune(line, '`') {
			count++
		}
	}
	return count
}

// ── Task 1: tsc-compress-auditor-prompt ──────────────────────────────────────

// TestC411_001_AuditorByteCountReduced asserts that agents/evolve-auditor.md
// has been compressed to < 18816 bytes (≥15% cut from baseline 22137 bytes).
//
// BEHAVIORAL: reads the file and checks len(raw). Adding a magic string cannot
// satisfy this — only genuine TSC prose compression cuts ≥3321 bytes.
//
// NEGATIVE (adversarial): an unmodified file (22137 bytes) fails here.
// This is the primary anti-no-op signal for tsc-compress-auditor-prompt.
//
// RED: currently 22137 bytes — above the 18816 threshold.
func TestC411_001_AuditorByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-auditor.md: %v", err)
	}
	const maxBytes = 18815 // strictly < 18816
	if len(raw) > maxBytes {
		t.Errorf("RED: agents/evolve-auditor.md has %d bytes — must be < 18816 (≥15%% cut from baseline 22137).\n"+
			"Builder must apply TSC: remove grammar glue (articles/auxiliaries/fillers/modal-padding)\n"+
			"while preserving all section headers, backtick spans, and gate anchors.\n"+
			"An unmodified file or marker-only edit cannot satisfy this predicate.",
			len(raw))
	}
}

// TestC411_002_AuditorTSCMarkerPresent asserts that agents/evolve-auditor.md
// contains the TSC applied marker (<!-- TSC applied).
//
// acs-predicate: config-check
//
// RED: the marker is absent in the baseline file.
// GREEN: only after Builder adds <!-- TSC applied --> near the top.
func TestC411_002_AuditorTSCMarkerPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-auditor.md")
	if !acsassert.FileContains(t, path, "<!-- TSC applied") {
		t.Errorf("RED: agents/evolve-auditor.md is missing the TSC marker.\n" +
			"Builder must add: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// TestC411_003_AuditorHeaderCountExact asserts exactly 25 '## ' section headers
// (the baseline count) in agents/evolve-auditor.md.
//
// acs-predicate: config-check
//
// ANTI-GAMING (edge): if Builder deleted a section to hit the byte target, this fails.
// TSC must reduce bytes by compressing prose within sections, not by removing them.
//
// Pre-existing GREEN: currently 25 headers. Must remain 25 after TSC.
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

// TestC411_004_AuditorCodeSpanLinesFloor asserts ≥74 lines containing at least one
// backtick in agents/evolve-auditor.md (~90% of baseline 82 code-span lines).
//
// BEHAVIORAL: counts lines with backticks in the file content. Deleting code examples
// or fenced code blocks drives this below 74 and fails this predicate.
//
// EDGE: a file with all code examples deleted (0 code-span lines) fails here —
// the strongest guard against "shrink by deleting code examples" gaming.
//
// Pre-existing GREEN: currently 82 code-span lines (above the floor of 74).
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

// TestC411_005_AuditorGateAnchorsPresent asserts that all five gate-critical
// anchors in agents/evolve-auditor.md survive TSC compression.
//
// acs-predicate: config-check
//
// The five anchors are required by the audit gate protocol:
//   - challenge-token (auth anchor used by the challenge/response gate)
//   - EGPS Verdict Computation (scoring section that drives the EGPS verdict)
//   - STOP CRITERION (halt condition — must not be elided)
//   - handoff-auditor.json (structured output artifact contract)
//   - acs-verdict.json (the verdict-of-record output file)
//
// Pre-existing GREEN: all 5 anchors present in baseline file.
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

// ── Task 2: tsc-compress-tdd-engineer-prompt ─────────────────────────────────

// TestC411_006_TDDByteCountReduced asserts that agents/evolve-tdd-engineer.md
// has been compressed to < 21712 bytes (≥15% cut from baseline 25544 bytes).
//
// BEHAVIORAL: reads the file and checks len(raw).
//
// NEGATIVE: an unmodified file (25544 bytes) fails here — primary anti-no-op
// signal for tsc-compress-tdd-engineer-prompt.
//
// RED: currently 25544 bytes — above the 21712 threshold.
func TestC411_006_TDDByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tdd-engineer.md: %v", err)
	}
	const maxBytes = 21711 // strictly < 21712
	if len(raw) > maxBytes {
		t.Errorf("RED: agents/evolve-tdd-engineer.md has %d bytes — must be < 21712 (≥15%% cut from baseline 25544).\n"+
			"Builder must apply TSC: remove grammar glue while preserving all section headers,\n"+
			"backtick spans, and EGPS/ACS anchors.",
			len(raw))
	}
}

// TestC411_007_TDDTSCMarkerPresent asserts that agents/evolve-tdd-engineer.md
// contains the TSC applied marker (<!-- TSC applied).
//
// acs-predicate: config-check
//
// RED: the marker is absent in the baseline file.
func TestC411_007_TDDTSCMarkerPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	if !acsassert.FileContains(t, path, "<!-- TSC applied") {
		t.Errorf("RED: agents/evolve-tdd-engineer.md is missing the TSC marker.\n" +
			"Builder must add: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// TestC411_008_TDDHeaderCountExact asserts exactly 17 '## ' section headers
// in agents/evolve-tdd-engineer.md (the baseline count).
//
// acs-predicate: config-check
//
// ANTI-GAMING: deleting sections to hit the byte target fails here.
//
// Pre-existing GREEN: currently 17 headers.
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

// TestC411_009_TDDCodeSpanLinesFloor asserts ≥85 lines containing at least one
// backtick in agents/evolve-tdd-engineer.md (~89% of baseline 95 code-span lines).
//
// BEHAVIORAL: counts lines with backticks.
//
// EDGE: a file with all code examples deleted (0 code-span lines) fails here.
//
// Pre-existing GREEN: currently 95 code-span lines (above the floor of 85).
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

// TestC411_010_TDDACSAnchorsPresent asserts that the three EGPS/ACS anchors
// in agents/evolve-tdd-engineer.md survive TSC compression.
//
// acs-predicate: config-check
//
// The three anchors form the EGPS predicate lane contract:
//   - EGPS (EGPS gate reference, appears in multiple places)
//   - predicates_test.go (canonical predicate file path)
//   - go:build acs (the build tag all predicates must carry)
//
// Pre-existing GREEN: all 3 anchors present in baseline.
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

// ── Task 3: tsc-compress-orchestrator-prompt ──────────────────────────────────

// TestC411_011_OrchestratorByteCountReduced asserts that agents/evolve-orchestrator.md
// has been compressed to < 20945 bytes (≥15% cut from baseline 24642 bytes).
//
// BEHAVIORAL: reads the file and checks len(raw).
//
// NEGATIVE: an unmodified file (24642 bytes) fails here — primary anti-no-op
// signal for tsc-compress-orchestrator-prompt.
//
// RED: currently 24642 bytes — above the 20945 threshold.
func TestC411_011_OrchestratorByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-orchestrator.md: %v", err)
	}
	const maxBytes = 20944 // strictly < 20945
	if len(raw) > maxBytes {
		t.Errorf("RED: agents/evolve-orchestrator.md has %d bytes — must be < 20945 (≥15%% cut from baseline 24642).\n"+
			"Builder must apply TSC: remove grammar glue while preserving all section headers,\n"+
			"backtick spans, and phase/guard anchors.",
			len(raw))
	}
}

// TestC411_012_OrchestratorTSCMarkerPresent asserts that agents/evolve-orchestrator.md
// contains the TSC applied marker (<!-- TSC applied).
//
// acs-predicate: config-check
//
// RED: the marker is absent in the baseline file.
func TestC411_012_OrchestratorTSCMarkerPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if !acsassert.FileContains(t, path, "<!-- TSC applied") {
		t.Errorf("RED: agents/evolve-orchestrator.md is missing the TSC marker.\n" +
			"Builder must add: <!-- TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md -->")
	}
}

// TestC411_013_OrchestratorHeaderCountExact asserts exactly 26 '## ' section headers
// in agents/evolve-orchestrator.md (the baseline count).
//
// acs-predicate: config-check
//
// ANTI-GAMING: deleting sections to hit the byte target fails here.
//
// Pre-existing GREEN: currently 26 headers.
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

// TestC411_014_OrchestratorCodeSpanLinesFloor asserts ≥99 lines containing at least
// one backtick in agents/evolve-orchestrator.md (~90% of baseline 110 code-span lines).
//
// BEHAVIORAL: counts lines with backticks.
//
// EDGE: a file with all code examples deleted (0 code-span lines) fails here.
//
// Pre-existing GREEN: currently 110 code-span lines (above the floor of 99).
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

// TestC411_015_OrchestratorPhaseGuardAnchorsPresent asserts that the phase/guard
// anchors in agents/evolve-orchestrator.md survive TSC compression.
//
// acs-predicate: config-check
//
// The five anchors form the orchestrator's phase-sequencing and gate contracts:
//   - STOP CRITERION (halt condition — must not be elided)
//   - Phase Loop (the phase-sequencing section header, exact text)
//   - evolve guard phase (the guard-call pattern)
//   - acs-verdict.json (the verdict-of-record output file read by orchestrator)
//   - phase-gate-precondition (kernel hook name)
//
// Pre-existing GREEN: all 5 anchors present in baseline.
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
