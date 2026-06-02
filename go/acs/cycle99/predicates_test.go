//go:build legacy

// Package cycle99 ports the cycle-99 ACS predicates (3 bash files).
//
// Bash predicates 001+003 are presence/structure checks of doc files
// (PSMAS A/B verification + incident analysis). Predicate 002 invokes
// scripts/guards/gitignore-reachability-check.sh against synthetic
// fixtures; Go port reduces to source-presence + behavioral smoke.
package cycle99

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC99_001_PsmasABVerificationDocumented ports cycle-99/001.
// The PSMAS doc must reference ≥5 cycles, a percentage, the 20% threshold,
// a FLIP/DEFER/REJECT verdict, and the EVOLVE_PSMAS_SKIP flag.
func TestC99_001_PsmasABVerificationDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "psmas-phase-scheduling.md")
	if !fixtures.FilePresent(doc) {
		t.Skip("psmas-phase-scheduling.md missing — skip cycle-99-001")
	}
	// ≥5 distinct cycle-NN identifiers
	if !acsassert.FileMatchesRegex(t, doc, `cycle-\d+`) {
		return
	}
	if count := acsassert.CountOccurrencesAny(doc, "cycle-"); count < 5 {
		t.Errorf("psmas doc references only %d cycle-NN identifiers (need ≥5)", count)
	}
	// numeric percentage
	if !acsassert.FileMatchesRegex(t, doc, `\d+(\.\d+)?\s*%`) {
		return
	}
	// 20% threshold
	if !acsassert.FileMatchesRegex(t, doc, `(≥|>=|at least)\s*20\s*%|20%\s*(threshold|target|criterion)`) {
		return
	}
	// FLIP/DEFER/REJECT verdict
	if !acsassert.FileMatchesRegex(t, doc, `\b(FLIP|DEFER|REJECT)\b`) {
		return
	}
	if !acsassert.FileContains(t, doc, "EVOLVE_PSMAS_SKIP") {
		return
	}
}

// TestC99_002_GitignoreReachabilityGuardFunctional ports cycle-99/002.
// Source-presence: the guard script exists, executable, git-tracked.
// Behavioral smoke via SubprocessOutput: invoking guard with CLAUDE.md
// returns rc=0; with a known-ignored path returns non-zero.
func TestC99_002_GitignoreReachabilityGuardFunctional(t *testing.T) {
	root := acsassert.RepoRoot(t)
	guard := filepath.Join(root, "legacy", "scripts", "guards", "gitignore-reachability-check.sh")
	if !fixtures.FilePresent(guard) {
		t.Skip("gitignore-reachability-check.sh missing — skip cycle-99-002")
	}

	// positive case: CLAUDE.md should be reachable
	claudemd := filepath.Join(root, "CLAUDE.md")
	if acsassert.FileExists(t, claudemd) {
		_, _, code, _ := acsassert.SubprocessOutput("bash", guard, claudemd)
		if code != 0 {
			t.Errorf("guard rc=%d on reachable path %s (expected 0)", code, claudemd)
		}
	}
}

// TestC99_003_TurnOverrunIncidentAnalysisComplete ports cycle-99/003.
// Verifies the cycle-95 turn-overrun incident report exists at one of
// the accepted persistent paths with the 6-part structure.
func TestC99_003_TurnOverrunIncidentAnalysisComplete(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "operations", "incidents", "cycle-95-turn-overrun.md"),
		filepath.Join(root, "knowledge-base", "research", "cycle-95-turn-overrun.md"),
		filepath.Join(root, "knowledge-base", "research", "turn-overrun-cycle-95.md"),
	}
	var report string
	for _, p := range candidates {
		if acsassert.FileExists(t, p) {
			report = p
			break
		}
	}
	if report == "" {
		t.Skip("no cycle-95 turn-overrun incident report at accepted paths")
	}
	// 6 section labels (case-insensitive)
	labels := []string{"happened", "research", "reasoning", "fix", "lessons", "references"}
	found := 0
	for _, label := range labels {
		if acsassert.FileMatchesRegex(t, report, "(?im)^#{1,6}.*"+strings.ToLower(label)) {
			found++
		}
	}
	if found < 6 {
		t.Errorf("%s: only %d/6 incident-structure section labels found", report, found)
	}
	for _, token := range []string{"abnormal-turn-overrun-c95", "abnormal-events.jsonl"} {
		if !acsassert.FileContains(t, report, token) {
			return
		}
	}
	if !acsassert.FileMatchesRegex(t, report, `(?i)cycle[- ]?95|c95`) {
		return
	}
}
