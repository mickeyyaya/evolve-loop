//go:build acs

// Package cycle84 ports the cycle-84 ACS predicates (3 bash files).
package cycle84

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// TestC84_001_LintBaselineExists ports cycle-84/001.
// .evolve/baselines/lint-markdown-structure-baseline.txt has >=10 lines.
func TestC84_001_LintBaselineExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	baseline := filepath.Join(root, ".evolve", "baselines", "lint-markdown-structure-baseline.txt")
	if !fixtures.FilePresent(baseline) {
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

// TestC84_002_CarryoverTodosSchemaValid ports cycle-84/002 (re-baselined
// 2026-06-05): state.json:carryoverTodos must be ABSENT or an ARRAY, and every
// entry must carry id/action/priority (go/internal/core/ports.go). The original
// "must be an empty array" assertion was retired — queueing deferred work via
// carryoverTodos[] is the sanctioned operator workflow, so a non-empty array is
// valid as long as it is well-formed. (The prior Go port both used the stale
// empty-array rule AND mis-used the asserting FileMatchesRegex helper, which
// t.Errorf'd before the intended t.Skipf could run — a false RED.)
func TestC84_002_CarryoverTodosSchemaValid(t *testing.T) {
	root := acsassert.RepoRoot(t)
	state := filepath.Join(root, ".evolve", "state.json")
	if !fixtures.FilePresent(state) {
		t.Skip("state.json missing — skip cycle-84-002")
	}
	raw, err := os.ReadFile(state)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	// Unmarshaling carryoverTodos into a typed slice fails if it is present but
	// not an array of objects → RED (matches the bash "not an array" branch).
	var s struct {
		CarryoverTodos []struct {
			ID       string `json:"id"`
			Action   string `json:"action"`
			Priority string `json:"priority"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Errorf("cycle-84-002: carryoverTodos is not a valid array of objects: %v", err)
		return
	}
	for i, td := range s.CarryoverTodos {
		if td.ID == "" || td.Action == "" || td.Priority == "" {
			t.Errorf("cycle-84-002: carryoverTodos[%d] missing required field (id/action/priority)", i)
		}
	}
}

// TestC84_003_ChangelogEntryExists ports cycle-84/003.
// CHANGELOG.md contains "Cycle 84" (case-insensitive).
func TestC84_003_ChangelogEntryExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changelog := filepath.Join(root, "CHANGELOG.md")
	if !fixtures.FilePresent(changelog) {
		t.Skip("CHANGELOG.md missing — skip cycle-84-003")
	}
	if !acsassert.FileMatchesRegex(t, changelog, `(?i)Cycle 84`) {
		return
	}
}
