//go:build acs

package cycle2661

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePrefix = "github.com/mickeyyaya/evolve-loop/go/"

const coveringTests = "X11|BranchSweep|RemoteSweep|DefaultChecks|RunGCCheck|GCRemoteSweep|PlanAndApplyWorktrees|BundleBranch"

func TestC2661_001_CheckOnlyBranchesPlansTheMergedAndTheClosedBranchAndKeepsEveryOther(t *testing.T) {
	names := []string{
		"TestX11_CheckPlansTheMergedAndTheClosedBranchAndKeepsEveryOther",
		"TestX11_CheckPrintsTheAllowListWithTipPullRequestAndClass",
		"TestX11_CheckOnlyTakesTheNamesOfTheRegistryAndGCOnlyKeepsItsScopes",
		"TestX11_B2PlansALandedBranchWithOwnWorkAndKeepsAnEmptyOneAndARemoteOnlyOne",
		"TestX11_OwnedPrefixesComeFromPolicy",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_002_AClosedBranchPastItsPullRequestIsKeptAndAMovedTipStaysAfterGC(t *testing.T) {
	names := []string{
		"TestX11_AClosedBranchThatMovedPastItsPullRequestIsKept",
		"TestX11_ABranchWhoseTipMovesAfterThePlanStaysAfterGC",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_003_TheBranchThatOriginHeadNamesStaysWhenItIsNotMain(t *testing.T) {
	names := []string{
		"TestX11_TheDefaultBranchThatOriginHeadNamesStays",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_004_AFullPullRequestLookupFallsBackToOneLookupPerCandidateBranch(t *testing.T) {
	names := []string{
		"TestX11_AFullPullRequestLookupFallsBackToOneLookupPerCandidate",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_005_GCRecordsEachDeleteAndThePrintedRestoreRebuildsALocalAndARemoteBranch(t *testing.T) {
	names := []string{
		"TestX11_GCRecordsEachDeleteAndThePrintedRestoreRebuildsBothSides",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_006_ShadowPrintsWouldDeleteAndAFailedRemoteDeleteIsAWarnThatKeepsTheExitCode(t *testing.T) {
	names := []string{
		"TestX11_ShadowPrintsWouldDeleteAndDeletesNothing",
		"TestX11_AFailedRemoteDeleteIsAWarnAndTheExitCodeStays",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_007_ACycleBranchIsNeverPlannedByTheBranchesCheck(t *testing.T) {
	names := []string{
		"TestX11_ACycleBranchIsNeverPlannedByThisCheck",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2661_009_AClosedBranchIsPlannedPastTheThreeDayGraceAndKeptBefore(t *testing.T) {
	names := []string{
		"TestX11_AClosedBranchIsPlannedPastTheGraceAndKeptBefore",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

type changedFile struct {
	rel   string
	lines []int
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutputIn(dir, "git", args...)
	if err != nil || code != 0 {
		t.Fatalf("git %v exit=%d err=%v\n%s", args, code, err, stderr)
	}
	return out
}

func diffBase(t *testing.T, goDir string) string {
	t.Helper()
	for _, ref := range []string{"main", "origin/main"} {
		out, _, code, err := acsassert.SubprocessOutputIn(goDir, "git", "merge-base", "HEAD", ref)
		if err == nil && code == 0 {
			return strings.TrimSpace(out)
		}
	}
	t.Fatalf("no merge base of HEAD with main or origin/main")
	return ""
}

func isProduction(rel string) bool {
	return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "acs/")
}

func changedProduction(t *testing.T, goDir string) []changedFile {
	t.Helper()
	base := diffBase(t, goDir)
	names := map[string]bool{}
	untracked := map[string]bool{}
	for _, rel := range strings.Fields(git(t, goDir, "diff", "--name-only", "--relative", base, "--", ".")) {
		if isProduction(rel) {
			names[rel] = true
		}
	}
	for _, rel := range strings.Fields(git(t, goDir, "ls-files", "--others", "--exclude-standard")) {
		if isProduction(rel) {
			names[rel] = true
			untracked[rel] = true
		}
	}
	hunk := regexp.MustCompile(`(?m)^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
	var files []changedFile
	for rel := range names {
		file := changedFile{rel: rel}
		for _, m := range hunk.FindAllStringSubmatch(git(t, goDir, "diff", "-U0", "--relative", base, "--", rel), -1) {
			start, _ := strconv.Atoi(m[1])
			count := 1
			if m[2] != "" {
				count, _ = strconv.Atoi(m[2])
			}
			for l := start; l < start+count; l++ {
				file.lines = append(file.lines, l)
			}
		}
		if len(file.lines) == 0 && untracked[rel] {
			raw, err := os.ReadFile(filepath.Join(goDir, rel))
			if err != nil {
				t.Fatal(err)
			}
			for l := 1; l <= strings.Count(string(raw), "\n"); l++ {
				file.lines = append(file.lines, l)
			}
		}
		if len(file.lines) > 0 {
			files = append(files, file)
		}
	}
	slices.SortFunc(files, func(a, b changedFile) int { return strings.Compare(a.rel, b.rel) })
	return files
}

type coverBlock struct {
	start, end, count int
	startCol, endCol  int
}

func parseProfile(t *testing.T, path string) map[string][]coverBlock {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no cover profile: %v", err)
	}
	line := regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) \d+ (\d+)$`)
	blocks := map[string][]coverBlock{}
	for _, l := range strings.Split(string(raw), "\n") {
		m := line.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[2])
		startCol, _ := strconv.Atoi(m[3])
		end, _ := strconv.Atoi(m[4])
		endCol, _ := strconv.Atoi(m[5])
		count, _ := strconv.Atoi(m[6])
		rel := strings.TrimPrefix(m[1], modulePrefix)
		block := coverBlock{start: start, end: end, count: count, startCol: startCol, endCol: endCol}
		merged := false
		for i, b := range blocks[rel] {
			if b.start == start && b.end == end && b.startCol == startCol && b.endCol == endCol {
				blocks[rel][i].count = max(b.count, count)
				merged = true
				break
			}
		}
		if !merged {
			blocks[rel] = append(blocks[rel], block)
		}
	}
	return blocks
}

func TestC2661_008_EveryChangedLineOfTheProductionCodeIsCoveredByAnX11TestAndTheRealVerbHasAnEndToEndTest(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	files := changedProduction(t, goDir)
	if len(files) == 0 {
		t.Fatalf("RED: no production Go file changed against the merge base; the branches check is not built")
	}
	pkgs := map[string]bool{"./cmd/evolve": true, "./internal/gc": true}
	for _, f := range files {
		pkgs["./"+filepath.ToSlash(filepath.Dir(f.rel))] = true
	}
	list := slices.Sorted(func(yield func(string) bool) {
		for p := range pkgs {
			if !yield(p) {
				return
			}
		}
	})
	profile := filepath.Join(t.TempDir(), "cover.out")
	args := append([]string{"test", "-count=1", "-run", coveringTests, "-coverpkg=" + strings.Join(list, ","), "-coverprofile=" + profile}, list...)
	out, stderr, code, err := acsassert.SubprocessOutputIn(goDir, "go", args...)
	if err != nil || code != 0 {
		t.Fatalf("go test -run X11 exit=%d err=%v\n%s", code, err, acsassert.OutputTail(out+stderr, 40))
	}
	blocks := parseProfile(t, profile)
	measured := 0
	for _, f := range files {
		for _, l := range f.lines {
			for _, b := range blocks[filepath.ToSlash(f.rel)] {
				if b.start <= l && l <= b.end {
					measured++
					if b.count == 0 {
						t.Errorf("%s:%d is changed and no covering test reaches it", f.rel, l)
					}
				}
			}
		}
	}
	if measured == 0 {
		t.Errorf("no changed line falls in a measured statement of %v", list)
	}
	endToEnd := filepath.Join(goDir, "cmd", "evolve", "cmd_gc_branches_x11_test.go")
	if !acsassert.FileContains(t, endToEnd, `w.run("check", "--only", "branches")`) {
		t.Errorf("the end-to-end test through the real verb is gone")
	}
}
