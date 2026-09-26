package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// The builder writes the code, and the TDD engineer encodes the same duties one phase earlier.
var houseRulePersonas = []string{"evolve-builder.md", "evolve-tdd-engineer.md"}

// alwaysOnBody returns the persona body an agent receives: frontmatter removed, on-demand tail stripped.
func alwaysOnBody(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter %s: %v", path, err)
	}
	return StripOnDemandSections(body)
}

func TestPersonaHouseRules_ApicoverGraduation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, name := range houseRulePersonas {
		body := strings.ToLower(alwaysOnBody(t, filepath.Join(root, "agents", name)))
		for _, want := range []string{
			"go/.apicover-enforce",   // the file to edit
			"./internal/",            // the pattern-line shape appended to it
			"apicover_named_test.go", // the second edit
			"every exported symbol",  // what that test must discharge
			"repo-wide",              // ADR-0069: which of the TWO apicover gates
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s (always-on body) must carry the apicover graduation requirement token %q", name, want)
			}
		}
	}
}

func TestPersonaHouseRules_CallerProof(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, name := range houseRulePersonas {
		body := strings.ToLower(alwaysOnBody(t, filepath.Join(root, "agents", name)))
		for _, want := range []string{
			"production caller", // must be NAMED
			"reach",             // the seam must be proven REACHED from it
			"dead code",         // a test-only caller is dead code
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s (always-on body) must carry the caller-proof requirement token %q", name, want)
			}
		}
	}
}

func TestPersonaHouseRules_StayWithinLineBudget(t *testing.T) {
	root := acsassert.RepoRoot(t)
	total := 0
	for _, f := range personaFiles {
		total += countLines(t, filepath.Join(root, "agents", f))
	}
	if total >= 751 {
		t.Errorf("combined scout/builder/auditor line count = %d, want < 751 — earn space for the house rules by tightening prose, not by raising the cap", total)
	}
}
