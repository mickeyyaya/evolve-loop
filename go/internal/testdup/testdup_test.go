package testdup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const helperSource = `package %s

import (
	"os"
	"path/filepath"
	"testing"
)

func %s(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
`

const reformattedHelperSource = `package %s

import (
	"os"
	"path/filepath"
	"testing"
)

@@ a comment that changes nothing
func %s(t *testing.T, name string) string {
	t.Helper()

	dir := t.TempDir() @@ trailing comment
	path := filepath.Join(dir,   name)
	if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}


	return path
}
`

const otherHelperSource = `package %s

import (
	"os"
	"testing"
)

func %s(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty")
	}
	return data
}
`

func plant(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, src := range files {
		if !filepath.IsLocal(rel) {
			t.Fatalf("fixture path %q escapes the root", rel)
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func source(template, pkg, fn string) string {
	out := strings.Replace(template, "%s", pkg, 1)
	out = strings.Replace(out, "%s", fn, 1)
	return strings.ReplaceAll(out, "@@", "/"+"/")
}

func ageTree(t *testing.T, dir string, when time.Time) {
	t.Helper()
	if err := filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, when, when)
	}); err != nil {
		t.Fatal(err)
	}
}

var (
	baselineDay = time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC)
	longAgo     = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	justNow     = time.Date(2026, time.November, 10, 0, 0, 0, 0, time.UTC)
)

func TestScan_CountsTheExtraCopiesOfEqualHelperBodies(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(helperSource, "a", "makeFile"),
		"internal/b/b_test.go": source(helperSource, "b", "newPath"),
		"internal/c/c_test.go": source(helperSource, "c", "tempFileAt"),
		"internal/d/d_test.go": source(otherHelperSource, "d", "readAll"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 2 {
		t.Errorf("Copies = %d, want 2 (three equal bodies are one original and two extra copies)", report.Copies)
	}
	if len(report.Groups) != 1 {
		t.Fatalf("Groups = %d, want 1: the unique helper must not form a group", len(report.Groups))
	}
	group := report.Groups[0]
	if len(group.Sites) != 3 {
		t.Errorf("Sites = %v, want the three equal helpers", group.Sites)
	}
	for _, want := range []string{"internal/a/a_test.go", "internal/b/b_test.go", "internal/c/c_test.go"} {
		found := false
		for _, site := range group.Sites {
			found = found || strings.Contains(site, want)
		}
		if !found {
			t.Errorf("Sites = %v, want one that names %s", group.Sites, want)
		}
	}
	for _, site := range group.Sites {
		if strings.Contains(site, "internal/d/") {
			t.Errorf("site %s holds the unique helper and must not be in the group", site)
		}
	}
}

func TestScan_ReportsNoCopiesForADistinctHelperInEachFile(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(helperSource, "a", "makeFile"),
		"internal/d/d_test.go": source(otherHelperSource, "d", "readAll"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 0 || len(report.Groups) != 0 {
		t.Errorf("report = %+v, want no copies and no groups", report)
	}
}

func TestScan_TreatsFormattingAndCommentsAsNoDifference(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(helperSource, "a", "makeFile"),
		"internal/b/b_test.go": source(reformattedHelperSource, "b", "newPath"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 1 {
		t.Errorf("Copies = %d, want 1: spacing, blank lines and comments do not change a body", report.Copies)
	}
}

func TestScan_CountsOnlyNonTestFunctionsOfTestFiles(t *testing.T) {
	root := t.TempDir()
	testBody := "package %s\n\nimport \"testing\"\n\nfunc %s(t *testing.T) {\n\tvalues := []int{1, 2, 3}\n\ttotal := 0\n\tfor _, v := range values {\n\t\ttotal += v\n\t}\n\tif total != 6 {\n\t\tt.Fatal(total)\n\t}\n}\n"
	productionBody := "package %s\n\nfunc %s(values []int) int {\n\ttotal := 0\n\tfor _, v := range values {\n\t\ttotal += v\n\t}\n\tif total < 0 {\n\t\treturn 0\n\t}\n\treturn total\n}\n"
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(testBody, "a", "TestSumA"),
		"internal/b/b_test.go": source(testBody, "b", "TestSumB"),
		"internal/a/a.go":      source(productionBody, "a", "SumA"),
		"internal/b/b.go":      source(productionBody, "b", "SumB"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 0 {
		t.Errorf("Copies = %d, want 0: Test functions and production files are not helpers", report.Copies)
	}
}

func TestScan_IgnoresTrivialOneStatementHelpers(t *testing.T) {
	root := t.TempDir()
	tiny := "package %s\n\nfunc %s() int {\n\treturn 1\n}\n"
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(tiny, "a", "one"),
		"internal/b/b_test.go": source(tiny, "b", "uno"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 0 {
		t.Errorf("Copies = %d, want 0: a one-statement body is not worth a standard-library replacement", report.Copies)
	}
}

func TestScan_SkipsVendorTestdataHiddenAndUnderscoreDirectories(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go":          source(helperSource, "a", "makeFile"),
		"vendor/x/x_test.go":            source(helperSource, "x", "makeFile"),
		"internal/a/testdata/t_test.go": source(helperSource, "t", "makeFile"),
		".hidden/h_test.go":             source(helperSource, "h", "makeFile"),
		"internal/_scratch/s_test.go":   source(helperSource, "s", "makeFile"),
	})

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 0 {
		t.Errorf("Copies = %d, want 0: only the runtime copy under internal/a is scanned; groups = %v", report.Copies, report.Groups)
	}
}

func TestScan_SkipsAnAcsCycleDirectoryOlderThanTheBaselineDate(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go":          source(helperSource, "a", "makeFile"),
		"acs/cycle7/predicates_test.go": source(helperSource, "cycle7", "makeFile"),
		"acs/cycle8/predicates_test.go": source(helperSource, "cycle8", "makeFile"),
	})
	ageTree(t, filepath.Join(root, "acs"), longAgo)

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 0 {
		t.Errorf("Copies = %d, want 0: acs directories older than the baseline date are history; groups = %v", report.Copies, report.Groups)
	}
}

func TestScan_ScansAnAcsCycleDirectoryNewerThanTheBaselineDate(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go":          source(helperSource, "a", "makeFile"),
		"acs/cycle7/predicates_test.go": source(helperSource, "cycle7", "makeFile"),
		"acs/cycle9/predicates_test.go": source(helperSource, "cycle9", "makeFile"),
	})
	ageTree(t, filepath.Join(root, "acs/cycle7"), longAgo)
	ageTree(t, filepath.Join(root, "acs/cycle9"), justNow)

	report, err := Scan(root, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 1 {
		t.Errorf("Copies = %d, want 1: only the newer acs/cycle9 copy counts, not the old acs/cycle7 one", report.Copies)
	}
	joined := strings.Join(report.Groups[0].Sites, " ")
	if !strings.Contains(joined, "acs/cycle9/") || strings.Contains(joined, "acs/cycle7/") {
		t.Errorf("Sites = %s, want acs/cycle9 and not acs/cycle7", joined)
	}
}

func TestScan_FailsOnARootThatDoesNotExist(t *testing.T) {
	if _, err := Scan(filepath.Join(t.TempDir(), "missing"), baselineDay); err == nil {
		t.Fatal("Scan of a missing root returned no error; a ratchet that cannot read its tree must fail loudly")
	}
}

func TestScan_FailsOnATestFileThatDoesNotParse(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{"internal/a/a_test.go": "package a\n\nfunc broken( {\n"})

	if _, err := Scan(root, baselineDay); err == nil {
		t.Fatal("Scan of an unparseable test file returned no error; a skipped file would hide copies")
	}
}

func TestCheck_FailsWhenTheCopiesAreAboveTheBaseline(t *testing.T) {
	report := Report{Copies: 3, Groups: []Group{{Body: "body", Sites: []string{"internal/a/a_test.go:makeFile", "internal/b/b_test.go:newPath", "internal/z/z_test.go:freshCopy"}}}}

	err := Check(report, Baseline{Count: 2, Date: baselineDay})

	if err == nil {
		t.Fatal("Check passed with 3 copies against a baseline of 2")
	}
	if !strings.Contains(err.Error(), "internal/z/z_test.go") {
		t.Errorf("error = %q, want it to name the sites of the duplicated body", err)
	}
}

func TestCheck_PassesAtTheBaseline(t *testing.T) {
	report := Report{Copies: 2}

	if err := Check(report, Baseline{Count: 2, Date: baselineDay}); err != nil {
		t.Fatalf("Check failed at the baseline: %v", err)
	}
}

func TestCheck_FailsWhenTheBaselineIsAboveTheCountSoALoweredCountLowersIt(t *testing.T) {
	report := Report{Copies: 1}

	err := Check(report, Baseline{Count: 4, Date: baselineDay})

	if err == nil {
		t.Fatal("Check passed with 1 copy against a baseline of 4; the baseline must go down in the same commit")
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "4") {
		t.Errorf("error = %q, want it to name the count 1 and the baseline 4", err)
	}
}

func TestRatchet_FailsOnAFixtureTreeWithANewCopyOfAHelperBody(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(helperSource, "a", "makeFile"),
		"internal/b/b_test.go": source(helperSource, "b", "newPath"),
	})
	atBaseline, err := Scan(root, baselineDay)
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline{Count: atBaseline.Copies, Date: baselineDay}
	if err := Check(atBaseline, baseline); err != nil {
		t.Fatalf("the fixture tree must pass at its own baseline: %v", err)
	}

	plant(t, root, map[string]string{"internal/c/c_test.go": source(helperSource, "c", "freshCopy")})
	withNewCopy, err := Scan(root, baselineDay)
	if err != nil {
		t.Fatal(err)
	}

	if err := Check(withNewCopy, baseline); err == nil {
		t.Fatalf("the ratchet passed after a new copy of a helper body: copies %d, baseline %d", withNewCopy.Copies, baseline.Count)
	}
}

func TestRatchet_PassesAgainAfterTheNewCopyIsRemoved(t *testing.T) {
	root := t.TempDir()
	plant(t, root, map[string]string{
		"internal/a/a_test.go": source(helperSource, "a", "makeFile"),
		"internal/b/b_test.go": source(helperSource, "b", "newPath"),
		"internal/c/c_test.go": source(helperSource, "c", "freshCopy"),
	})
	withCopy, err := Scan(root, baselineDay)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "internal/c/c_test.go")); err != nil {
		t.Fatal(err)
	}
	afterRemoval, err := Scan(root, baselineDay)
	if err != nil {
		t.Fatal(err)
	}

	if afterRemoval.Copies != withCopy.Copies-1 {
		t.Errorf("copies went %d -> %d, want exactly one fewer after one replaced copy", withCopy.Copies, afterRemoval.Copies)
	}
	if err := Check(afterRemoval, Baseline{Count: withCopy.Copies, Date: baselineDay}); err == nil {
		t.Error("Check passed with a count below the old baseline: replacing a copy must lower the baseline too")
	}
}

func TestParseBaseline_ReadsTheCountAndTheDate(t *testing.T) {
	baseline, err := ParseBaseline([]byte("count=12\ndate=2026-11-09\n"))

	if err != nil {
		t.Fatal(err)
	}
	if baseline.Count != 12 {
		t.Errorf("Count = %d, want 12", baseline.Count)
	}
	if !baseline.Date.Equal(baselineDay) {
		t.Errorf("Date = %v, want %v", baseline.Date, baselineDay)
	}
}

func TestParseBaseline_RejectsMalformedInput(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"missing date":     "count=3\n",
		"missing count":    "date=2026-11-09\n",
		"negative count":   "count=-1\ndate=2026-11-09\n",
		"count not number": "count=many\ndate=2026-11-09\n",
		"date not a date":  "count=3\ndate=yesterday\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseBaseline([]byte(input)); err == nil {
				t.Errorf("ParseBaseline(%q) returned no error", input)
			}
		})
	}
}

func TestFormatBaseline_RoundTripsThroughParseBaseline(t *testing.T) {
	want := Baseline{Count: 7, Date: baselineDay}

	got, err := ParseBaseline(FormatBaseline(want))

	if err != nil {
		t.Fatal(err)
	}
	if got.Count != want.Count || !got.Date.Equal(want.Date) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestScan_DatesAnAcsCycleDirectoryByItsCommitNotByTheMtimeOfAFreshCheckout(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("internal/a/a_test.go", source(helperSource, "a", "makeFile"))
	repo.Write("acs/cycle7/predicates_test.go", source(helperSource, "cycle7", "makeFile"))
	t.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "old")
	ageTree(t, repo.Dir, justNow)

	oldDir, err := Scan(repo.Dir, baselineDay)
	if err != nil {
		t.Fatal(err)
	}
	if oldDir.Copies != 0 {
		t.Fatalf("Copies = %d, want 0: acs/cycle7 was committed in 2020, and a checkout that touched its files today does not make it new; groups = %v", oldDir.Copies, oldDir.Groups)
	}

	repo.Write("acs/cycle9/predicates_test.go", source(helperSource, "cycle9", "makeFile"))
	t.Setenv("GIT_AUTHOR_DATE", "2026-11-10T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-11-10T00:00:00Z")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "new")

	withNewDir, err := Scan(repo.Dir, baselineDay)
	if err != nil {
		t.Fatal(err)
	}
	if withNewDir.Copies != 1 {
		t.Errorf("Copies = %d, want 1: acs/cycle9 was committed after the baseline date", withNewDir.Copies)
	}
}

func TestScan_ScansAStagedAcsCycleDirectoryThatNoCommitHolds(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("internal/a/a_test.go", source(helperSource, "a", "makeFile"))
	t.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write("acs/cycle9/predicates_test.go", source(helperSource, "cycle9", "makeFile"))
	ageTree(t, filepath.Join(repo.Dir, "acs"), justNow)
	repo.Git("add", "-A")

	report, err := Scan(repo.Dir, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 1 {
		t.Errorf("Copies = %d, want 1: acs/cycle9 is staged for the landing commit, so it is the change under review, not history; groups = %v", report.Copies, report.Groups)
	}
}

func TestGroupsUnder_KeepsOnlyTheGroupsWithASiteInTheDirectory(t *testing.T) {
	inside := Group{Body: "a", Sites: []string{"acs/cycle9/predicates_test.go:write", "cmd/evolve/x_test.go:writeX"}}
	sibling := Group{Body: "b", Sites: []string{"acs/cycle90/predicates_test.go:has", "internal/a/a_test.go:has"}}
	elsewhere := Group{Body: "c", Sites: []string{"internal/a/a_test.go:read", "internal/b/b_test.go:read"}}
	report := Report{Copies: 3, Groups: []Group{inside, sibling, elsewhere}}

	for _, dir := range []string{"acs/cycle9", "acs/cycle9/"} {
		got := GroupsUnder(report, dir)

		if len(got) != 1 || got[0].Body != "a" {
			t.Errorf("GroupsUnder(%q) = %v, want only the group with a site under acs/cycle9, not the one under acs/cycle90", dir, got)
		}
	}
	if got := GroupsUnder(report, "acs/cycle7"); len(got) != 0 {
		t.Errorf("GroupsUnder(acs/cycle7) = %v, want none", got)
	}
}

func TestScan_ScansAnOldAcsDirectoryWithAChangeThatNoCommitHoldsYet(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("internal/a/a_test.go", source(helperSource, "a", "makeFile"))
	repo.Write("acs/regression/old_test.go", source(otherHelperSource, "regression", "readBack"))
	t.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "old")
	repo.Write("acs/regression/new_test.go", source(helperSource, "regression", "makeFile"))
	repo.Git("add", "-A")

	report, err := Scan(repo.Dir, baselineDay)

	if err != nil {
		t.Fatal(err)
	}
	if report.Copies != 1 {
		t.Errorf("Copies = %d, want 1: the landing commit makes acs/regression new on main, so the gate before it must scan it too; groups = %v", report.Copies, report.Groups)
	}
}
