//go:build acs

package cycle2231

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2231_001_TheRatchetFailsOnAFixtureTreeWithANewCopyOfAHelperBody(t *testing.T) {
	root := acsassert.RepoRoot(t)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "./internal/testdup",
		Pattern: "^(TestRatchet_FailsOnAFixtureTreeWithANewCopyOfAHelperBody|TestCheck_FailsWhenTheCopiesAreAboveTheBaseline|TestScan_CountsTheExtraCopiesOfEqualHelperBodies|TestScan_ReportsNoCopiesForADistinctHelperInEachFile|TestScan_TreatsFormattingAndCommentsAsNoDifference|TestScan_CountsOnlyNonTestFunctionsOfTestFiles|TestScan_IgnoresTrivialOneStatementHelpers)$",
		Names: []string{
			"TestRatchet_FailsOnAFixtureTreeWithANewCopyOfAHelperBody",
			"TestCheck_FailsWhenTheCopiesAreAboveTheBaseline",
			"TestScan_CountsTheExtraCopiesOfEqualHelperBodies",
			"TestScan_ReportsNoCopiesForADistinctHelperInEachFile",
			"TestScan_TreatsFormattingAndCommentsAsNoDifference",
			"TestScan_CountsOnlyNonTestFunctionsOfTestFiles",
			"TestScan_IgnoresTrivialOneStatementHelpers",
		},
	})
}

func TestC2231_002_TheRatchetPassesOnTheRealTreeAtTheCountInTheBaselineFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	baselinePath := filepath.Join(root, "go", ".test-dup-baseline")
	if !acsassert.FileExists(t, baselinePath) {
		t.Fatal("go/.test-dup-baseline is missing")
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", "go/.test-dup-baseline"); code == 0 {
		t.Fatal("go/.test-dup-baseline is gitignored and would be dropped at ship")
	}
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		t.Fatal(err)
	}
	report, err := testdup.Scan(filepath.Join(root, "go"), baseline.Date)
	if err != nil {
		t.Fatal(err)
	}
	if err := testdup.Check(report, baseline); err != nil {
		t.Errorf("the real tree is not at its baseline: %v", err)
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "./internal/repocontract",
		Pattern: "^TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline$",
		Names:   []string{"TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline"},
	})
}

func TestC2231_003_TheCountWentDownByTheReplacedCopiesAndTheBaselineWentDownWithIt(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "go", ".test-dup-baseline"))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		t.Fatal(err)
	}
	current, err := testdup.Scan(filepath.Join(root, "go"), baseline.Date)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Count != current.Copies {
		t.Errorf("baseline holds %d but the tree has %d copies: the baseline must equal the count in the same commit", baseline.Count, current.Copies)
	}

	baseDir := t.TempDir()
	tarball := filepath.Join(baseDir, "base.tar")
	if _, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "archive", "--format=tar", "-o", tarball, "HEAD", "--",
		":(glob)go/**/*_test.go", ":(exclude)go/vendor", ":(exclude)go/acs"); err != nil || code != 0 {
		t.Fatalf("git archive of the base commit failed: exit=%d err=%v stderr=%s", code, err, stderr)
	}
	if _, stderr, code, err := acsassert.SubprocessOutput("tar", "-xf", tarball, "-C", baseDir); err != nil || code != 0 {
		t.Fatalf("tar extract failed: exit=%d err=%v stderr=%s", code, err, stderr)
	}
	before, err := testdup.Scan(filepath.Join(baseDir, "go"), baseline.Date)
	if err != nil {
		t.Fatal(err)
	}
	if before.Copies == 0 {
		t.Fatal("the base commit has no copies of a helper body, so there is nothing to replace; the scanner counts nothing")
	}
	if current.Copies >= before.Copies {
		t.Errorf("copies went %d -> %d: the first copies must move to the standard library and the count must go down", before.Copies, current.Copies)
	}

	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "./internal/testdup",
		Pattern: "^(TestRatchet_PassesAgainAfterTheNewCopyIsRemoved|TestCheck_FailsWhenTheBaselineIsAboveTheCountSoALoweredCountLowersIt|TestCheck_PassesAtTheBaseline|TestParseBaseline_ReadsTheCountAndTheDate|TestParseBaseline_RejectsMalformedInput|TestFormatBaseline_RoundTripsThroughParseBaseline)$",
		Names: []string{
			"TestRatchet_PassesAgainAfterTheNewCopyIsRemoved",
			"TestCheck_FailsWhenTheBaselineIsAboveTheCountSoALoweredCountLowersIt",
			"TestCheck_PassesAtTheBaseline",
			"TestParseBaseline_ReadsTheCountAndTheDate",
			"TestParseBaseline_RejectsMalformedInput",
			"TestFormatBaseline_RoundTripsThroughParseBaseline",
		},
	})
}

func TestC2231_004_OldAcsCycleDirectoriesOlderThanTheBaselineDateAreNotScanned(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	raw, err := os.ReadFile(filepath.Join(goDir, ".test-dup-baseline"))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		t.Fatal(err)
	}

	everything, err := testdup.Scan(goDir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	nothingNew, err := testdup.Scan(goDir, time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	acsSite := regexp.MustCompile(`^acs/(cycle[0-9]+|regression|redteam)/`)
	acsSitesWhenAllAreNew, acsSitesWhenNoneAreNew := 0, 0
	for _, group := range everything.Groups {
		for _, site := range group.Sites {
			if acsSite.MatchString(site) {
				acsSitesWhenAllAreNew++
			}
		}
	}
	for _, group := range nothingNew.Groups {
		for _, site := range group.Sites {
			if acsSite.MatchString(site) {
				acsSitesWhenNoneAreNew++
			}
		}
	}
	if acsSitesWhenAllAreNew == 0 {
		t.Error("with a baseline date of 1970 the scan reads no acs test file: the acs directories are never scanned, so the date gates nothing")
	}
	if acsSitesWhenNoneAreNew != 0 {
		t.Errorf("with a baseline date of 2999 the scan still reports %d acs sites", acsSitesWhenNoneAreNew)
	}

	atBaseline, err := testdup.Scan(goDir, baseline.Date)
	if err != nil {
		t.Fatal(err)
	}
	checkedDirs := map[string]bool{}
	for _, group := range atBaseline.Groups {
		for _, site := range group.Sites {
			dir := regexp.MustCompile(`^(acs/cycle[0-9]+)/`).FindString(site)
			if dir == "" || checkedDirs[dir] {
				continue
			}
			checkedDirs[dir] = true
			out, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "log", "-1", "--format=%ct", "--", "go/"+strings.TrimSuffix(dir, "/"))
			committed, parseErr := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
			if code != 0 || parseErr != nil {
				continue
			}
			if time.Unix(committed, 0).Before(baseline.Date) {
				t.Errorf("%s was last committed %s, before the baseline date %s, yet the scan reads it: a fresh checkout makes the mtime new, so the age must come from the commit", dir, time.Unix(committed, 0).UTC().Format(time.RFC3339), baseline.Date.Format("2006-01-02"))
			}
		}
	}

	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir,
		Package: "./internal/testdup",
		Pattern: "^(TestScan_SkipsAnAcsCycleDirectoryOlderThanTheBaselineDate|TestScan_ScansAnAcsCycleDirectoryNewerThanTheBaselineDate|TestScan_DatesAnAcsCycleDirectoryByItsCommitNotByTheMtimeOfAFreshCheckout)$",
		Names: []string{
			"TestScan_SkipsAnAcsCycleDirectoryOlderThanTheBaselineDate",
			"TestScan_ScansAnAcsCycleDirectoryNewerThanTheBaselineDate",
			"TestScan_DatesAnAcsCycleDirectoryByItsCommitNotByTheMtimeOfAFreshCheckout",
		},
	})
}

func TestC2231_005_TheNewPackageIsEnrolledInTheApicoverGateAndEveryExportedSymbolIsNamedInATest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(filepath.Join(root, "go", ".apicover-enforce"), "./internal/testdup") {
		t.Error("go/.apicover-enforce does not enroll ./internal/testdup")
	}
	pkgDir := filepath.Join(root, "go", "internal", "testdup")
	symbols, err := apicover.Enumerate(context.Background(), pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) == 0 {
		t.Fatal("go/internal/testdup exports no symbol")
	}
	named, err := apicover.NamesReferencedInTests(context.Background(), pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range symbols {
		if symbol.Ignored {
			continue
		}
		name := symbol.Name
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if !named[name] {
			t.Errorf("exported symbol %s (%s) is named in no test of go/internal/testdup", symbol.Name, symbol.Kind)
		}
	}
}
