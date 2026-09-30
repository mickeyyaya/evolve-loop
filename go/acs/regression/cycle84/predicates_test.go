//go:build acs

package cycle84

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

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
