//go:build acs

package cycle2267

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	notePath = "docs/architecture/packages/internal-gc.md"
	planPath = "docs/plans/gc-closes-the-legacy-gap-2026-11.md"
)

var amendedDecisionIDs = []string{"D2", "D4", "D6", "D7", "D9"}

func requireTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/gc",
		Pattern: "^(" + strings.Join(topLevel(names), "|") + ")$",
		Names:   names,
	})
}

func topLevel(names []string) []string {
	var tops []string
	for _, name := range names {
		top, _, _ := strings.Cut(name, "/")
		if len(tops) == 0 || tops[len(tops)-1] != top {
			tops = append(tops, top)
		}
	}
	return tops
}

func repoText(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func lineWith(text, marker string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	return ""
}

func TestC2267_001_TheDocsCheckFindsTheAmendedByLineAndBothAmendedInvariants(t *testing.T) {
	requireTests(t, "TestInternalGCNote_AmendedByLinksThePlan")
}

func TestC2267_002_TheCheckRejectsEachMissingAmendmentAndAnEmptyNote(t *testing.T) {
	requireTests(t,
		"TestInternalGCNote_ANoteWithEveryAmendmentPasses",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/no_Amended_by_line",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/Amended_by_links_another_plan",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/Amended_by_omits_D7",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/Amended_by_names_D20_instead_of_D2",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/old_branch_wording_returns",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/old_hook_wording_returns",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/hook_invariant_omits_the_scope",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/hook_invariant_omits_D7",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/branch_invariant_omits_D4",
		"TestInternalGCNote_EachMissingAmendmentFailsTheCheck/no_Invariants_section",
		"TestInternalGCNote_AnEmptyNoteFailsWithAClearMessage")
}

func TestC2267_003_TheAmendedPassagesOfTheNotePassTheSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := repoText(t, notePath)
	var passages []string
	for _, marker := range []string{"Amended by", "gc.wave_end.scope", "deleted only after"} {
		line := lineWith(note, marker)
		if line == "" {
			t.Fatalf("RED: the note has no line with %q to lint", marker)
		}
		passages = append(passages, line)
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, finding := range stelint.Check([]byte(strings.Join(passages, "\n\n")), stelint.Options{Words: words}) {
		if !finding.IsSkipNotice() {
			t.Errorf("STE finding %s: %s: %q", finding.Rule, finding.Message, finding.Excerpt)
		}
	}
}

func TestC2267_004_EveryDecisionIdOfTheAmendedByLineIsADecisionOfTheLinkedPlan(t *testing.T) {
	note := repoText(t, notePath)
	plan := repoText(t, planPath)
	amended := lineWith(note, "Amended by")
	if amended == "" {
		t.Fatalf("RED: the note has no Amended by line")
	}
	for _, id := range amendedDecisionIDs {
		if !regexp.MustCompile(`\b` + id + `\b`).MatchString(amended) {
			t.Errorf("RED: the Amended by line does not name %s", id)
		}
		if !regexp.MustCompile(`(?m)^\| ` + id + ` \|`).MatchString(plan) {
			t.Errorf("the plan has no decision row %s", id)
		}
	}
	for _, named := range regexp.MustCompile(`\bD\d+\b`).FindAllString(amended, -1) {
		if !regexp.MustCompile(`(?m)^\| ` + named + ` \|`).MatchString(plan) {
			t.Errorf("the Amended by line names %s, which is no decision row of the plan", named)
		}
	}
}
