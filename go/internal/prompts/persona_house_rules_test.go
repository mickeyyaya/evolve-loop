package prompts

// persona_house_rules_test.go — pins the two MANDATORY house rules that lived
// only in operator lore, so every cycle rediscovered them by failing:
//
//  1. apicover graduation (inbox acs-apicover-enrollment-in-builder-brief, 0.94)
//     — batch-21 HALTED at cycle-1218 because THREE lanes aborted on "new
//     internal package absent from go/.apicover-enforce"; console hit the same
//     class twice the same day on PR #372. The requirement appeared in NEITHER
//     the builder's nor the TDD engineer's brief.
//
//  2. caller proof (inbox builder-persona-requires-caller-proof, 0.90) — two
//     console implementer agents satisfied their stated contract exactly and
//     left integration unwired (a struct field with no consumer; a lint with no
//     caller). Same class in loop cycles: the P2 accounting seam wired only into
//     the sequential loop body was unreachable in fleet mode (#373), and
//     ADR-0074's typed routing shipped inert once.
//
// The assertions are deliberately phrase-level: an instruction the agent never
// reads is worthless, so each rule must be present AND must survive
// StripOnDemandSections (i.e. sit ABOVE the "## Reference Index" on-demand
// marker, which compaction truncates at).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// houseRulePersonas are the two implementer personas that must carry both rules:
// the builder writes the production code, and the TDD engineer authors the RED
// contract that has to encode the same obligations one phase earlier.
var houseRulePersonas = []string{"evolve-builder.md", "evolve-tdd-engineer.md"}

// alwaysOnBody returns the persona body an agent actually receives under
// compaction: frontmatter removed, on-demand tail stripped.
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

// TestPersonaHouseRules_ApicoverGraduation asserts both personas carry the
// enrollment requirement WITH the concrete two-edit example. Naming the enforce
// file alone is what the floor already did and it was not enough — the persona
// must show the pattern line and the test-file path.
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

// TestPersonaHouseRules_CallerProof asserts both personas make integration an
// acceptance criterion: name the production caller, cite the REACHABILITY test,
// and treat a test-only caller as dead code.
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

// TestPersonaHouseRules_StayWithinLineBudget is the scope guard that makes the
// additions honest: agents/evolve-{scout,builder,auditor}.md share a hard
// combined budget (TestPersonaStopCriterionDedupe_CombinedLineCountReduced,
// enforced in-lane by core.personaBudgetFailures). New house rules must be paid
// for by tightening existing prose, never by raising the cap — so this asserts
// the SAME ceiling from the additions' own test file.
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
