package codeshape

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func TestParseDiff_RangesAndAddedLinesOfProductionFilesOnly(t *testing.T) {
	t.Parallel()
	text := "+++ b/go/internal/a/a.go\n@@ -1,0 +2,3 @@\n+x := 1\n+y := 2\n+z := 3\n@@ -9 +11 @@\n+w := 4\n@@ -20,2 +22,0 @@\n@@@ -1 +1 @@@\n+++ b/go/internal/a/a_test.go\n@@ -1 +1 @@\n+t := 1\n+++ /dev/null\n@@ -1,2 +0,0 @@\n"

	d := parseDiff(text)

	if got := d.Ranges["go/internal/a/a.go"]; len(got) != 2 || got[0] != (lineRange{2, 4}) || got[1] != (lineRange{11, 11}) {
		t.Errorf("ranges = %+v, want [2,4] and [11,11]; a deletion-only hunk and a combined header add none", got)
	}
	if len(d.Added) != 2 || len(d.Added[0]) != 3 || d.Added[1][0] != "w := 4" {
		t.Errorf("added = %q, want two hunks of production lines and no test lines", d.Added)
	}
	if len(d.Ranges) != 1 {
		t.Errorf("ranges = %v, want only the production file: a test file and a deleted file have none", d.Ranges)
	}
}

func shipFixture(t *testing.T) (string, []pipelinerates.Cycle) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/internal/a/a.go", smallFunc)
	first := commit(t, repo, "small")
	repo.Write("go/internal/a/a.go", smallFunc+branchy)
	repo.Write("go/internal/a/copy.go", "package a\n\nfunc Copy(x int) int {\n"+branchy[len("\nfunc Branchy(x int) int {\n"):])
	second := commit(t, repo, "branchy")
	cycles := []pipelinerates.Cycle{
		{ID: 10, Wave: 1, Shipped: true, Commit: first},
		{ID: 11, Wave: 2, Shipped: true, Commit: second},
		{ID: 12, Wave: 2},
	}
	return repo.Dir, cycles
}

func TestChangedConformance_OneReadingPerShipOfTheClosingWaveAndOneForTheWave(t *testing.T) {
	t.Parallel()
	root, cycles := shipFixture(t)

	readings, err := ChangedConformance(root, cycles)

	if err != nil || len(readings) != 2 {
		t.Fatalf("readings %+v, %v; want the cycle-11 reading and the wave reading", readings, err)
	}
	if r := readings[0]; r.Cycle != 11 || r.Window != "cycle:11" || r.N != 2 || r.Interval.Value != 0 || r.Index != "CQ1" {
		t.Errorf("cycle reading = %+v, want 0 of 2 changed functions conforming", r)
	}
	if r := readings[1]; r.Cycle != 0 || r.Window != "wave:2" || r.N != 2 {
		t.Errorf("wave reading = %+v", r)
	}
	if _, err := ChangedConformance(root, []pipelinerates.Cycle{{ID: 1, Wave: 1, Shipped: true, Commit: "0000000000000000000000000000000000000000"}}); err == nil {
		t.Error("an unknown commit must be an error")
	}
	if got, err := ChangedConformance(root, nil); err != nil || len(got) != 1 || got[0].N != 0 {
		t.Errorf("no cycles = %+v, %v; want one empty wave reading", got, err)
	}
}

func TestDiffDuplication_CountsAddedWindowsThatAlsoAppearElsewhere(t *testing.T) {
	t.Parallel()
	root, cycles := shipFixture(t)
	tree, err := Scan(GoRoot(root))
	if err != nil {
		t.Fatal(err)
	}

	r, err := DiffDuplication(root, tree, cycles)

	if err != nil || r.Index != "CQ4" || r.Window != "wave:2/added" || r.N == 0 || r.Interval.Value <= 0 {
		t.Errorf("reading %+v, %v; want a positive rate: Copy repeats Branchy's body", r, err)
	}
	if _, err := DiffDuplication(root, tree, []pipelinerates.Cycle{{ID: 1, Wave: 1, Shipped: true, Commit: "nope"}}); err == nil {
		t.Error("an unknown commit must be an error")
	}
}

func TestChangedFuncs_AShipThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("go/internal/a/a.go", "package a\n\nfunc {\n")
	sha := commit(t, repo, "broken")
	if _, err := ChangedFuncs(repo.Dir, sha); err == nil {
		t.Error("a production file that does not parse at the ship must fail")
	}
}

func TestTreeReadings_CensusValuesAndTheTreeFloor(t *testing.T) {
	t.Parallel()
	tree, err := Scan(fixtureTree(t))
	if err != nil {
		t.Fatal(err)
	}

	cq1, cq3 := TreeConformance(tree), InterfaceDepth(tree)

	if math.Abs(cq1.Interval.Value-6.0/9) > 1e-9 || cq1.N != 9 || cq1.Threshold == nil || *cq1.Threshold != 0.878 || cq1.Method != qualitypanel.MethodCensus {
		t.Errorf("CQ1 = %+v, want 6/9 against the tree floor 0.878", cq1)
	}
	if cq3.Index != "CQ3" || cq3.N != 1 {
		t.Errorf("CQ3 = %+v", cq3)
	}
	if empty := TreeConformance(Tree{}); empty.Interval.Value != 0 || empty.N != 0 {
		t.Errorf("empty tree = %+v", empty)
	}
	if GoRoot("/plane") != filepath.Join("/plane", "go") {
		t.Error("GoRoot is the go module under the project root")
	}
}

func TestScan_AnUnreadableFileFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "internal/x/x.go", "package x\n")
	if err := os.Chmod(filepath.Join(root, "internal/x/x.go"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(root); err == nil {
		t.Error("an unreadable source file must fail the scan")
	}
}

func TestChangedFuncs_AFileThatGitCannotShowIsAnError(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("go/internal/a/a.go", smallFunc)
	sha := commit(t, repo, "small")
	gittest.InstallFault(t, gittest.GitFault{Verb: "show", Skip: 1, Code: 128})

	if _, err := ChangedFuncs(repo.Dir, sha); err == nil {
		t.Error("a failed show of the file at the ship must fail")
	}
}
