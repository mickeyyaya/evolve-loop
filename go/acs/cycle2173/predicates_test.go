//go:build acs

package cycle2173

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPkg     = "./cmd/evolve"
	gotypesPkg = "./internal/adapters/codeindex/gotypes"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func requireTestsPass(t *testing.T, names ...string) {
	t.Helper()
	requireTestsPassIn(t, cmdPkg, names...)
}

func requireTestsPassIn(t *testing.T, pkg string, names ...string) {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: exit=%d err=%v\n%s\n%s", code, err, acsassert.OutputTail(out, 40), acsassert.OutputTail(errOut, 20))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 40))
		}
	}
}

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func between(t *testing.T, text, from, to string) string {
	t.Helper()
	start := strings.Index(text, from)
	if start < 0 {
		t.Fatalf("heading %q not found", from)
	}
	rest := text[start:]
	if end := strings.Index(rest[len(from):], to); end >= 0 {
		return rest[:len(from)+end]
	}
	return rest
}

func TestC2173_001_RefreshBuildsTheGotypesIndexFromTheIndexWorktreeAfterTheTrailStep(t *testing.T) {
	requireTestsPass(t,
		"TestCodeRefresh_Gotypes_BuildsTheIndexFromTheIndexWorktreeAfterTheTrailStep",
		"TestSyncMainCodeIndex_Gotypes_TheMergeBoundaryBuildsTheIndexAfterTheTrailStep",
		"TestDefaultCodeIndexHost_Gotypes_BuildsThroughTheRealGotypesBuild",
	)
}

func TestC2173_002_AFailedBuildIsOneWarnLineAndTheRefreshKeepsTheTrailExit(t *testing.T) {
	requireTestsPass(t,
		"TestCodeRefresh_Gotypes_AFailedBuildPrintsOneWarnLineAndKeepsTheTrailExit",
		"TestCodeRefresh_Gotypes_AFailedBuildUnderJSONIsOneWarningAndNotAnError",
		"TestCodeRefresh_Gotypes_ASuccessfulBuildUnderJSONAddsNoWarning",
		"TestCodeRefresh_Gotypes_AFailedTrailStepStillBuildsAndKeepsItsOwnExit",
		"TestSyncMainCodeIndex_Gotypes_AFailedBuildIsOneWarnLineAndSyncMainStillExitsZero",
	)
}

func TestC2173_003_TheBuildWritesBesideTheIndexWorktreeOutsideEveryGitTree(t *testing.T) {
	requireTestsPass(t, "TestCodeRefresh_Gotypes_WritesTheIndexBesideTheWorktreeOutsideEveryGitTree")
}

func TestC2173_004_NoBuildRunsWhenTheRefreshStopsBeforeTheTrailStepOrInsideAPhase(t *testing.T) {
	requireTestsPass(t,
		"TestCodeRefresh_Gotypes_NoBuildRunsWhenTheRefreshStopsBeforeTheTrailStep",
		"TestCodeRefresh_Gotypes_InsideAPhaseNoBuildRuns",
	)
}

// acs-predicate: config-check
func TestC2173_005_TheSpecNamesTheStepInSectionFourAndThePlanMarksG2Built(t *testing.T) {
	spec := readDoc(t, "docs/architecture/code-index-cli.md")
	section := between(t, spec, "## 4. The index lifecycle", "\n## 5. ")
	for _, want := range []string{"gotypes", "index.gob", "WARN"} {
		if !strings.Contains(section, want) {
			t.Errorf("section 4 of code-index-cli.md does not mention %q", want)
		}
	}
	plan := readDoc(t, "docs/plans/code-index-backends-2026-10.md")
	var g2Row string
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "| G2 |") {
			g2Row = line
		}
	}
	if g2Row == "" {
		t.Fatal("the G2 row is missing from the plan")
	}
	if !strings.Contains(g2Row, "Built") || strings.Contains(g2Row, "Open:") {
		t.Errorf("the G2 row is not marked built: %s", g2Row)
	}
	status := ""
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "- **Status:**") {
			status = line
		}
	}
	if !strings.Contains(status, "G2") || strings.Contains(status, "G2 to G5 are open") {
		t.Errorf("the plan status line still lists G2 as open: %s", status)
	}
}

func TestC2173_006_ChangedGoFilesCarryNoCodeComments(t *testing.T) {
	for _, name := range []string{
		"cmd/evolve/cmd_code_refresh.go",
		"cmd/evolve/cmd_syncmain.go",
		"cmd/evolve/cmd_code_refresh_gotypes_test.go",
		"internal/adapters/codeindex/gotypes/build.go",
		"internal/adapters/codeindex/gotypes/gotypes_test.go",
	} {
		path := filepath.Join(goDir(t), filepath.FromSlash(name))
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, group := range parsed.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") && !strings.HasPrefix(c.Text, "// acs-predicate:") {
					t.Errorf("%s carries the comment %q: code has no comments", name, c.Text)
				}
			}
		}
	}
}

func TestC2173_007_TheGotypesBuildRunsUnderTheDeadlineOfTheRefreshAndStopsAtIt(t *testing.T) {
	requireTestsPass(t,
		"TestCodeRefresh_Gotypes_TheBuildRunsUnderTheDeadlineOfTheRefresh",
		"TestCodeRefresh_Gotypes_ABuildThatOutlivesTheDeadlineIsOneWarnLineAndTheRefreshStillExitsZero",
		"TestSyncMainCodeIndex_Gotypes_ABuildThatOutlivesTheDeadlineIsOneWarnLineAndSyncMainStillExitsZero",
	)
	requireTestsPassIn(t, gotypesPkg,
		"TestBuild_ACancelledContextIsAnErrorAndTheLastGoodIndexStays",
		"TestBuild_ADeadlineThatPassesAfterTheListStopsTheBuildAndWritesNothing",
	)
}

func TestC2173_008_AMultiLineBuildFailureIsStillOneWarnLine(t *testing.T) {
	requireTestsPass(t,
		"TestCodeRefresh_Gotypes_AMultiLineCauseIsStillOneWarnLine",
		"TestCodeRefresh_Gotypes_AMultiLineCauseUnderJSONIsOneWarningWithoutALineBreak",
		"TestSyncMainCodeIndex_Gotypes_AMultiLineCauseIsStillOneWarnLine",
	)
}
