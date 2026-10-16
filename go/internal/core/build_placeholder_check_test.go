package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaceholderTokenFailures_RefusesAnUnsubstitutedTemplateToken(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	body := "# Build Report\n\n## Full suite\n`go test -count=1 ./...` → FULLSUITE_PLACEHOLDER\n\nok: ACS_RESULT_PLACEHOLDER\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: ws, Worktree: t.TempDir()})
	if len(got) != 2 {
		t.Fatalf("two unsubstituted tokens on two lines → two failures, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "FULLSUITE_PLACEHOLDER") || !strings.Contains(got[0], "build-report.md:4") {
		t.Errorf("a failure names the token and its report line so the builder can substitute it exactly: %q", got[0])
	}
	if !strings.Contains(got[1], "ACS_RESULT_PLACEHOLDER") || !strings.Contains(got[1], "build-report.md:6") {
		t.Errorf("second token, own line: %q", got[1])
	}
}

func TestPlaceholderTokenFailures_CleanReportAndProseAreNotTokens(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	body := "# Build Report\n\nThe `_PLACEHOLDER` suffix rule is documented; a placeholder image and PLACEHOLDER_ALPHA are prose, and lowercase full_suite_placeholder is a variable name.\n"
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: ws, Worktree: t.TempDir()}); len(got) != 0 {
		t.Errorf("only an UPPER_SNAKE token ending in _PLACEHOLDER is a template token: %v", got)
	}
	if got := PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: t.TempDir(), Worktree: t.TempDir()}); len(got) != 0 {
		t.Errorf("no report → nothing to refuse (the deliverables gate owns absence): %v", got)
	}
}

func TestDefaultBuildFloorChecks_IncludesThePlaceholderTokenCheck(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("result: FULLSUITE_PLACEHOLDER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	got := DefaultBuildFloorChecks(context.Background(), ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: wt, ProjectRoot: wt})
	found := false
	for _, f := range got {
		if strings.Contains(f, "FULLSUITE_PLACEHOLDER") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the default floor engine runs the placeholder-token check (wiring proof): %v", got)
	}
}

func placeholderFailuresFor(t *testing.T, body string) []string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return PlaceholderTokenFailures(context.Background(), ReviewInput{Workspace: ws, Worktree: t.TempDir()})
}

func TestPlaceholderTokenFailures_CorrectionSectionQuotingTheTokenPasses(t *testing.T) {
	t.Parallel()

	body := "# Build Report\n\nresult: ok\n\n## Correction 2\nRemoved the `CMD_EVOLVE_PLACEHOLDER` slot flagged at build-report.md:62.\n```\nCMD_EVOLVE_PLACEHOLDER\n```\n"
	if got := placeholderFailuresFor(t, body); len(got) != 0 {
		t.Errorf("a correction note that quotes the finding is not an unfilled slot: %v", got)
	}
}

func TestPlaceholderTokenFailures_TokenOutsideCorrectionSectionStillFails(t *testing.T) {
	t.Parallel()

	body := "# Build Report\n\nresult: CMD_EVOLVE_PLACEHOLDER\n`FULLSUITE_PLACEHOLDER`\n\n## Correction 1\nquote: `CMD_EVOLVE_PLACEHOLDER`\n"
	got := placeholderFailuresFor(t, body)
	if len(got) != 2 {
		t.Fatalf("bare and backticked tokens before the correction still fail, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "build-report.md:3") || !strings.Contains(got[1], "build-report.md:4") {
		t.Errorf("line numbers stay exact: %v", got)
	}
}

func TestPlaceholderTokenFailures_SectionAfterCorrectionIsCheckedAgain(t *testing.T) {
	t.Parallel()

	body := "# Build Report\n\n## Correction 1\nquote: `A_PLACEHOLDER`\n\n## Full suite\nresult: `B_PLACEHOLDER`\n"
	got := placeholderFailuresFor(t, body)
	if len(got) != 1 || !strings.Contains(got[0], "B_PLACEHOLDER") || !strings.Contains(got[0], "build-report.md:7") {
		t.Errorf("a new ## heading ends the exemption: %v", got)
	}
}

func TestPlaceholderTokenFailures_BareTokenUnderCorrectionHeadingFails(t *testing.T) {
	t.Parallel()

	got := placeholderFailuresFor(t, "# Build Report\n\n## Correction 1\nresult: CMD_EVOLVE_PLACEHOLDER\n")
	if len(got) != 1 || !strings.Contains(got[0], "build-report.md:4") {
		t.Errorf("a heading must not launder a real unfilled slot: %v", got)
	}
}

func TestPlaceholderTokenFailures_QuotedTokenUnderCorrectionHeadingPasses(t *testing.T) {
	t.Parallel()

	body := "## Correction 1\nremoved `A_PLACEHOLDER` and `B_PLACEHOLDER` slots\n```\nC_PLACEHOLDER\n```\nafter fence\n"
	if got := placeholderFailuresFor(t, body); len(got) != 0 {
		t.Errorf("backtick spans and fenced blocks are quotes: %v", got)
	}
}

func TestPlaceholderTokenFailures_FenceClosesAndBareTokenAfterItFails(t *testing.T) {
	t.Parallel()

	got := placeholderFailuresFor(t, "## Correction 1\n```\nA_PLACEHOLDER\n```\nB_PLACEHOLDER\n")
	if len(got) != 1 || !strings.Contains(got[0], "build-report.md:5") {
		t.Errorf("a closed fence stops quoting: %v", got)
	}
}

func TestPlaceholderTokenFailures_SubheadingKeepsCorrectionSectionOpen(t *testing.T) {
	t.Parallel()

	got := placeholderFailuresFor(t, "## Correction 1\n### Detail\nbare: A_PLACEHOLDER\nquoted: `B_PLACEHOLDER`\n")
	if len(got) != 1 || !strings.Contains(got[0], "build-report.md:3") {
		t.Errorf("a ### subheading stays in the section, where a bare token still fails: %v", got)
	}
}

func TestPlaceholderTokenFailures_H1EndsCorrectionSection(t *testing.T) {
	t.Parallel()

	got := placeholderFailuresFor(t, "## Correction 1\nq: `A_PLACEHOLDER`\n# Appendix\nr: `B_PLACEHOLDER`\n")
	if len(got) != 1 || !strings.Contains(got[0], "build-report.md:4") {
		t.Errorf("an H1 ends the section, so a backticked token after it fails: %v", got)
	}
}

func TestPlaceholderTokenFailures_TitledCorrectionHeadingsOpenASection(t *testing.T) {
	t.Parallel()

	for _, heading := range []string{"## Correction 2: removed slot", "## Correction 2 — fixed X", "## Correction 12 - fixed"} {
		if got := placeholderFailuresFor(t, heading+"\nquote `A_PLACEHOLDER`\n"); len(got) != 0 {
			t.Errorf("heading %q opens a section: %v", heading, got)
		}
	}
}

func TestPlaceholderTokenFailures_UnnumberedCorrectionHeadingsDoNotExempt(t *testing.T) {
	t.Parallel()

	for _, heading := range []string{"## Corrections", "## Correction notes", "## Correction", "## Correction 2b", "### Correction 2"} {
		body := "# Build Report\n\n" + heading + "\nresult: `A_PLACEHOLDER`\n"
		got := placeholderFailuresFor(t, body)
		if len(got) != 1 || !strings.Contains(got[0], "build-report.md:4") {
			t.Errorf("heading %q must not exempt its lines: %v", heading, got)
		}
	}
}

func TestPlaceholderTokenFailures_UnclosedFenceInCorrectionEndsAtTheNextHeading(t *testing.T) {
	t.Parallel()

	got := placeholderFailuresFor(t, "## Correction 1\n```\nA_PLACEHOLDER\n## Full suite\nresult: B_PLACEHOLDER\n")
	if len(got) != 1 || !strings.Contains(got[0], "build-report.md:5") || !strings.Contains(got[0], "B_PLACEHOLDER") {
		t.Errorf("an unclosed fence must not exempt the sections after it: %v", got)
	}
}
