package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var houseRulePersonas = []string{"evolve-builder.md", "evolve-tdd-engineer.md"}

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
			"go/.apicover-enforce",
			"./internal/",
			"apicover_named_test.go",
			"every exported symbol",
			"repo-wide",
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
			"production caller",
			"reach",
			"dead code",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s (always-on body) must carry the caller-proof requirement token %q", name, want)
			}
		}
	}
}

func TestPersonaHouseRules_GitBackedFixturesUseGittest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, name := range houseRulePersonas {
		body := strings.ToLower(alwaysOnBody(t, filepath.Join(root, "agents", name)))
		for _, want := range []string{"internal/gittest", "gittest.fixture", "gittest.bare", "gittest.clone", "raw `git init`", "rawgitratchet"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s (always-on body) must carry the git-fixture requirement token %q", name, want)
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
