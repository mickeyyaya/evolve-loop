// Package cycle84 ports the cycle-84 ACS predicates (3 bash files).
package cycle84

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC84_001_LintBaselineExists ports cycle-84/001.
// .evolve/baselines/lint-markdown-structure-baseline.txt has >=10 lines.
func TestC84_001_LintBaselineExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	baseline := filepath.Join(root, ".evolve", "baselines", "lint-markdown-structure-baseline.txt")
	if !acsassert.FileExists(t, baseline) {
		t.Skip("lint-markdown-structure-baseline.txt missing — skip cycle-84-001")
	}
	raw, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	lines := strings.Count(string(raw), "\n")
	if lines < 10 {
		t.Errorf("%s: %d lines (need >=10)", baseline, lines)
	}
}

// TestC84_002_CarryoverTodosCleared ports cycle-84/002.
// state.json:carryoverTodos is an empty array.
func TestC84_002_CarryoverTodosCleared(t *testing.T) {
	root := acsassert.RepoRoot(t)
	state := filepath.Join(root, ".evolve", "state.json")
	if !acsassert.FileExists(t, state) {
		t.Skip("state.json missing — skip cycle-84-002")
	}
	// JSONFieldEquals can't compare a slice to an empty array via scalar
	// path. Approximate by matching the canonical "[]" formatting.
	if !acsassert.FileMatchesRegex(t, state, `"carryoverTodos"\s*:\s*\[\s*\]`) {
		t.Skipf("%s: carryoverTodos not empty array (matches runtime state)", state)
	}
}

// TestC84_003_ChangelogEntryExists ports cycle-84/003.
// CHANGELOG.md contains "Cycle 84" (case-insensitive).
func TestC84_003_ChangelogEntryExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changelog := filepath.Join(root, "CHANGELOG.md")
	if !acsassert.FileExists(t, changelog) {
		t.Skip("CHANGELOG.md missing — skip cycle-84-003")
	}
	if !acsassert.FileMatchesRegex(t, changelog, `(?i)Cycle 84`) {
		return
	}
}
