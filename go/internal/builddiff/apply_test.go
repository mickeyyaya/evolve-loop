package builddiff

import (
	"errors"
	"strings"
	"testing"
)

const cycle1479Report = `# Build Report

## Worktree
- **Branch:** cycle-1479
- **Commit:** 1d6ee809
- **Files changed:** 2

<!-- ANCHOR:diff_summary -->
## Changes
` + "```tsv\nAction\tFile\tDescription\nMODIFY\tgo/a.go\tadds the guard\nMODIFY\tgo/gone.go\treverted later\n```" + `

## Metrics
| Metric | Value |
|---|---|
| files_changed | pending <!-- POSTHOC: git show HEAD --numstat | wc -l --> |
| lines_added / lines_removed | pending <!-- POSTHOC: git show HEAD --numstat | awk '{a+=$1;d+=$2}END{print a,d}' --> |
| num_turns | pending <!-- POSTHOC: jq '.num_turns' build-usage.json --> |

## Handoff Summary
- **Verdict:** PASS
`

func TestApply_ANoOpCycleStatesZeroAndNeverThePriorCommitsStats(t *testing.T) {
	got, warnings := Apply(cycle1479Report, Summary{}, "abc1234567890", nil)

	for _, want := range []string{
		"- **Files changed:** 0 (host-derived from the cycle base abc123456789)",
		"| files_changed | 0 <!-- host-derived: git diff --numstat abc123456789 --> |",
		"| lines_added / lines_removed | 0 / 0 <!-- host-derived: git diff --numstat abc123456789 --> |",
		"| num_turns | pending <!-- POSTHOC: jq '.num_turns' build-usage.json --> |",
		"go/a.go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Files changed:** 2") || strings.Contains(got, "MODIFY\tgo/a.go") {
		t.Errorf("a no-op cycle keeps a stat or a row of another commit:\n%s", got)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestApply_TheChangesTableIsTheDiffWithTheBuildersDescriptions(t *testing.T) {
	s := Summary{Files: 2, Added: 7, Deleted: 1, Changes: []Change{{Action: "MODIFY", Path: "go/a.go"}, {Action: "ADD", Path: "go/b.go"}}}

	got, _ := Apply(cycle1479Report, s, "abc", nil)

	for _, want := range []string{
		"MODIFY\tgo/a.go\tadds the guard",
		"ADD\tgo/b.go\t(not described by the build)",
		"Declared by the build but not in the diff: `go/gone.go`",
		"| lines_added / lines_removed | 7 / 1 <!-- host-derived: git diff --numstat abc --> |",
		"- **Files changed:** 2 (host-derived from the cycle base abc)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "## Changes") != 1 || !strings.Contains(got, "## Metrics") || !strings.Contains(got, "## Handoff Summary") {
		t.Errorf("the rewrite must keep one Changes section and the other sections:\n%s", got)
	}
}

func TestApply_ADeriveFaultWritesAbsentAndALoudWarningNeverPending(t *testing.T) {
	if unsafe, _ := Apply(cycle1479Report, Summary{}, "abc", errors.New("git diff\nfailed -->")); strings.Contains(unsafe, "failed -->") {
		t.Errorf("the cause must not close the comment early:\n%s", unsafe)
	}
	got, warnings := Apply(cycle1479Report, Summary{}, "abc", errors.New("git diff failed"))

	if strings.Contains(got, "| files_changed | pending") || strings.Contains(got, "| lines_added / lines_removed | pending") {
		t.Errorf("a fault leaves the literal pending:\n%s", got)
	}
	if !strings.Contains(got, "| files_changed | absent <!-- host could not derive the diff: git diff failed --> |") {
		t.Errorf("the metric must read absent with the cause:\n%s", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "git diff failed") {
		t.Errorf("warnings = %v, want one that names the cause", warnings)
	}
	if !strings.Contains(got, "MODIFY\tgo/gone.go\treverted later") {
		t.Errorf("a fault keeps the builder's Changes table:\n%s", got)
	}
}

func TestApply_AReportWithNoChangesSectionGainsOneAndEachMetricSpellingIsRead(t *testing.T) {
	report := "# Build Report\n\n| Files changed | pending <!-- POSTHOC: x --> |\n| lines_added | pending <!-- POSTHOC: x --> |\n| Lines removed | pending <!-- POSTHOC: x --> |\n"
	got, _ := Apply(report, Summary{Files: 1, Added: 2, Deleted: 3, Changes: []Change{{Action: "ADD", Path: "a"}}}, "b", nil)
	for _, want := range []string{"| Files changed | 1 <!--", "| lines_added | 2 <!--", "| Lines removed | 3 <!--", "## Changes", "ADD\ta\t(not described by the build)"} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
}

func TestApply_AMarkdownTableRowCarriesItsDescription(t *testing.T) {
	report := "## Changes\n| Action | File | Description |\n|---|---|---|\n| MODIFY | `go/a.go` | the fix |\n\n## Next\n"
	got, _ := Apply(report, Summary{Files: 1, Changes: []Change{{Action: "MODIFY", Path: "go/a.go"}}}, "b", nil)
	if !strings.Contains(got, "MODIFY\tgo/a.go\tthe fix") || !strings.Contains(got, "## Next") {
		t.Errorf("the markdown row's description must carry over:\n%s", got)
	}
}

func TestApply_TheBuildersProseInTheChangesSectionStays(t *testing.T) {
	s := Summary{Files: 1, Changes: []Change{{Action: "ADD", Path: "a"}}}
	cases := map[string]string{
		"prose and no table":  "## Changes\nThe build kept the old API.\n\n## Next\n",
		"an unclosed fence":   "## Changes\nThe build kept the old API.\n```tsv\nAction\tFile\tDescription\nADD\ta\tnew\n",
		"prose after a table": "## Changes\n| Action | File | Description |\n| ADD | a | new |\nThe build kept the old API.\n",
	}
	for name, report := range cases {
		got, _ := Apply(report, s, "b", nil)
		if !strings.Contains(got, "The build kept the old API.") || !strings.Contains(got, "ADD\ta\t") || strings.Count(got, "## Changes") != 1 {
			t.Errorf("%s: the prose or the host table is lost:\n%s", name, got)
		}
	}
}
