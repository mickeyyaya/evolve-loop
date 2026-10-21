package evalgate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func tddReport(t *testing.T, body string) core.ReviewInput {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "test-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.ReviewInput{Phase: "tdd", Workspace: ws}
}

func cycle1788Table(disposition string) string {
	p7, p8 := "007", "008"
	if strings.Contains(disposition, "console-followup") {
		p7, p8 = "-", "—"
	}
	return "## AC-Materialization\n\n" +
		"| # | Criterion | Disposition | Needs changed | Predicates |\n" +
		"|---|-----------|-------------|---------------|------------|\n" +
		"| 1 | the lint flags an inverted idiom | predicate | `go/internal/evalqualitycheck/unsatisfiable.go` | 001 |\n" +
		"| 7 | `evolve eval quality-check` runs the lint | " + disposition + " | `go/internal/cli/guardcmd/eval.go` | " + p7 + " |\n" +
		"| 8 | quality-check exits 2 on a finding | " + disposition + " | go/internal/cli/guardcmd/eval.go:40 | " + p8 + " |\n" +
		"\n" +
		"## RED Run Output\n"
}

func TestProtectedACGate_ThePredicatesOfCycle1788OnAProtectedPathBlock(t *testing.T) {
	reason, block := protectedACGate{}.check(tddReport(t, cycle1788Table("predicate")))

	if !block || !strings.Contains(reason, "AC 7, 8") || !strings.Contains(reason, "go/internal/cli/guardcmd/eval.go") || !strings.Contains(reason, "console-followup") {
		t.Fatalf("check = (%q, %v); a lane predicate on a protected path is unsatisfiable by construction, so it must become a console-followup", reason, block)
	}
	if strings.Contains(reason, "AC 1") {
		t.Errorf("reason %q names AC 1, whose path a lane can change", reason)
	}
}

func TestProtectedACGate_AConsoleFollowupOnAProtectedPathPasses(t *testing.T) {
	for _, disposition := range []string{"console-followup", "console-followup (no predicate)", "**console-followup**"} {
		if reason, block := (protectedACGate{}).check(tddReport(t, cycle1788Table(disposition))); block || reason != "" {
			t.Errorf("%s: check = (%q, %v); a console-followup is the legal disposition of a protected AC", disposition, reason, block)
		}
	}
	if _, block := (protectedACGate{}).check(tddReport(t, cycle1788Table("**predicate**"))); !block {
		t.Error("a bold predicate disposition is still a predicate")
	}
}

func TestProtectedACGate_AnACTableWithoutTheNeedsChangedColumnBlocks(t *testing.T) {
	report := "## AC-Materialization\n| # | Criterion | Disposition | Predicates |\n|---|---|---|---|\n| 1 | x | predicate | 001 |\n"

	reason, block := protectedACGate{}.check(tddReport(t, report))

	if !block || !strings.Contains(reason, "needs changed") {
		t.Fatalf("check = (%q, %v); without the column the gate cannot see a protected AC, so the table must carry it", reason, block)
	}
}

func TestProtectedACGate_AReportWithNoACTableIsNotChecked(t *testing.T) {
	for name, in := range map[string]core.ReviewInput{
		"no report":     {Phase: "tdd", Workspace: t.TempDir()},
		"no AC heading": tddReport(t, "## RED Run Output\n| # | Disposition |\n|---|---|\n| 1 | predicate |\n"),
	} {
		if reason, block := (protectedACGate{}).check(in); block || reason != "" {
			t.Errorf("%s: check = (%q, %v), want no finding", name, reason, block)
		}
	}
}

func TestProtectedACGate_WiredIntoReviewer(t *testing.T) {
	rv := NewReviewer(config.StageEnforce).(*reviewer)
	rv.logf = func(string, ...any) {}

	res := rv.Review(context.Background(), tddReport(t, cycle1788Table("predicate")))

	if res.Approve || !strings.Contains(res.Reason, "go/internal/cli/guardcmd/eval.go") {
		t.Fatalf("Review = %+v; the reviewer must run the protected-AC gate on the TDD phase", res)
	}
	if !(protectedACGate{}).appliesTo("tdd") || (protectedACGate{}).appliesTo("build") {
		t.Error("the gate reviews the TDD report only")
	}
}

func TestProtectedACGate_OnlyTheACSectionsTableIsRead(t *testing.T) {
	earlier := "## Coverage Gaps\n| Gap | Disposition |\n|---|---|\n| flaky path | deferred |\n\n"
	if reason, block := (protectedACGate{}).check(tddReport(t, earlier+cycle1788Table("console-followup"))); block || reason != "" {
		t.Fatalf("check = (%q, %v); a Disposition table outside the AC section is not the AC table", reason, block)
	}
	later := "## AC-Materialization\nAll ACs are predicates.\n\n## Notes\n| # | Disposition |\n|---|---|\n| 1 | predicate |\n"
	if reason, block := (protectedACGate{}).check(tddReport(t, later)); !block || !strings.Contains(reason, "has no AC table") {
		t.Fatalf("check = (%q, %v); a table after the AC section ends is not read, so the AC section holds none", reason, block)
	}
	if reason, block := (protectedACGate{}).check(tddReport(t, "## Acceptance\n| # | Disposition | Needs changed |\n|---|---|---|\n| 7 | predicate | go/internal/cli/guardcmd/eval.go |\n")); !block || !strings.Contains(reason, "AC 7") {
		t.Fatalf("check = (%q, %v); the accepted heading alias ## Acceptance holds the AC table too", reason, block)
	}
}

func acReport(heading, rows string) string {
	return heading + "\n\n| # | Criterion | Disposition | Needs changed | Predicates |\n|---|---|---|---|---|\n" + rows + "\n## RED Run Output\n"
}

func TestProtectedACGate_APredicateRowThatNamesNoChangedPathBlocks(t *testing.T) {
	for _, cell := range []string{"", "-", "n/a", "none"} {
		reason, block := protectedACGate{}.check(tddReport(t, acReport("## AC-Materialization", "| 3 | the lint runs | predicate | "+cell+" | 003 |\n")))
		if !block || !strings.Contains(reason, "AC 3 names no path") {
			t.Errorf("cell %q: check = (%q, %v); a predicate always needs some path changed, so an empty cell is the omission the rule catches", cell, reason, block)
		}
	}
}

func TestProtectedACGate_AConsoleFollowupThatStillListsAPredicateBlocks(t *testing.T) {
	reason, block := protectedACGate{}.check(tddReport(t, acReport("## AC-Materialization", "| 7 | wired into quality-check | console-followup | go/internal/cli/guardcmd/eval.go | 007 |\n")))
	if !block || !strings.Contains(reason, "AC 7 is a console-followup but lists predicate(s) 007") {
		t.Fatalf("check = (%q, %v); a console-followup keeps no lane predicate, or the 1788 red-on-every-tree failure stays", reason, block)
	}
}

func TestProtectedACGate_AModuleRelativeProtectedPathBlocks(t *testing.T) {
	reason, block := protectedACGate{}.check(tddReport(t, acReport("## AC-Materialization", "| 7 | wired | predicate | internal/cli/guardcmd/eval.go | 007 |\n")))
	if !block || !strings.Contains(reason, "go/internal/cli/guardcmd/eval.go") {
		t.Fatalf("check = (%q, %v); a path relative to the go/ module names the same protected file", reason, block)
	}
}

func TestProtectedACGate_AHeadingVariantStillHoldsTheACTable(t *testing.T) {
	for _, heading := range []string{"## AC-Materialization (cycle 1788)", "### AC-Materialization", "## acceptance"} {
		if _, block := (protectedACGate{}).check(tddReport(t, acReport(heading, "| 7 | wired | predicate | go/internal/cli/guardcmd/eval.go | 007 |\n"))); !block {
			t.Errorf("heading %q: the AC table was not read", heading)
		}
	}
}

func TestProtectedACGate_AnACSectionWithoutADispositionTableBlocks(t *testing.T) {
	report := "## AC-Materialization\n| # | Criterion | Status | Predicates |\n|---|---|---|---|\n| 1 | x | predicate | 001 |\n"
	reason, block := protectedACGate{}.check(tddReport(t, report))
	if !block || !strings.Contains(reason, "has no AC table with a \"disposition\" column") {
		t.Fatalf("check = (%q, %v); an AC section the gate cannot read must not pass silently", reason, block)
	}
}

func TestProtectedACGate_ASubHeadingOrAFencedHashLineDoesNotEndTheACSection(t *testing.T) {
	report := "## AC-Materialization\n\n#### Notes\n```\n# a shell comment\n```\n" +
		"| # | Criterion | Disposition | Needs changed | Predicates |\n|---|---|---|---|---|\n| 7 | wired | predicate | go/internal/cli/guardcmd/eval.go | 007 |\n\n## RED Run Output\n"
	if reason, block := (protectedACGate{}).check(tddReport(t, report)); !block || !strings.Contains(reason, "AC 7 need protected path") {
		t.Fatalf("check = (%q, %v); a deeper heading or a fenced # line stays inside the AC section", reason, block)
	}
}

func TestProtectedACGate_AHeadingThatOnlyStartsWithAnAcceptedNameIsNotTheACSection(t *testing.T) {
	report := "## Acceptance Test Plan\nprose only\n\n" + cycle1788Table("predicate")
	if reason, block := (protectedACGate{}).check(tddReport(t, report)); !block || !strings.Contains(reason, "AC 7, 8 need protected path") {
		t.Fatalf("check = (%q, %v); ## Acceptance Test Plan is not the AC section, the AC-Materialization one is", reason, block)
	}
}

func TestProtectedACGate_APredicateRowCutShortBeforeItsPathsBlocks(t *testing.T) {
	reason, block := protectedACGate{}.check(tddReport(t, acReport("## AC-Materialization", "| 9 | a row cut short | predicate |\n| 10 | no disposition |\n")))
	if !block || !strings.Contains(reason, "AC 9 names no path") || strings.Contains(reason, "10") {
		t.Fatalf("check = (%q, %v); a row with no Needs changed cell is the omission the rule catches, and a row with no disposition is not an AC row", reason, block)
	}
}

func TestProtectedACGate_AConsoleFollowupWithProseInItsPredicatesCellPasses(t *testing.T) {
	for _, cell := range []string{"none (console-followup)", "–", "no predicate", "n/a — console"} {
		if reason, block := (protectedACGate{}).check(tddReport(t, acReport("## AC-Materialization", "| 7 | wired | console-followup | go/internal/cli/guardcmd/eval.go | "+cell+" |\n"))); block {
			t.Errorf("cell %q: check = (%q, %v); prose that names no predicate id lists no predicate", cell, reason, block)
		}
	}
	if _, block := (protectedACGate{}).check(tddReport(t, acReport("## AC-Materialization", "| 7 | wired | console-followup | go/internal/cli/guardcmd/eval.go | TestC1788_007 |\n"))); !block {
		t.Error("a predicate named by its test still counts")
	}
}

func TestProtectedACGate_ADashSuffixedHeadingHoldsTheACTable(t *testing.T) {
	for _, heading := range []string{"## AC-Materialization — cycle 2020", "## AC-Materialization - cycle 2020"} {
		if _, block := (protectedACGate{}).check(tddReport(t, acReport(heading, "| 7 | wired | predicate | go/internal/cli/guardcmd/eval.go | 007 |\n"))); !block {
			t.Errorf("heading %q: the AC table was not read", heading)
		}
	}
}
