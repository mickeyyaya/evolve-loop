//go:build acs

package cycle1857

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	flockPkg        = "./internal/adapters/flock"
	deadlineTests   = "TestLockWithin_ADeadline"
	isolatedRuns    = 40
	handoffAwaitRow = `lockwithin\.go:50\.[0-9]+,[0-9]+\.[0-9]+ [0-9]+ ([0-9]+)`
)

func buildCoverBinary(t *testing.T, goDir string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "flock.test")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-c", "-cover", "-o", bin, flockPkg)
	if err != nil || code != 0 {
		t.Fatalf("cannot build the flock cover binary: exit=%d err=%v stderr=%s", code, err, stderr)
	}
	return bin
}

func runCovered(t *testing.T, bin, pkgDir, runExpr, profile string) {
	t.Helper()
	_, stderr, code, err := acsassert.SubprocessOutput(
		"env", "-C", pkgDir, bin, "-test.run", runExpr, "-test.count=1", "-test.coverprofile="+profile)
	if err != nil || code != 0 {
		t.Fatalf("flock tests %q failed: exit=%d err=%v stderr=%s", runExpr, code, err, stderr)
	}
}

func TestC1857_001_TheGrantAfterDeadlineBranchIsCoveredByEveryIsolatedRun(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	pkgDir := filepath.Join(goDir, "internal", "adapters", "flock")
	bin := buildCoverBinary(t, goDir)
	row := regexp.MustCompile(handoffAwaitRow)

	uncovered := 0
	for i := 0; i < isolatedRuns; i++ {
		profile := filepath.Join(t.TempDir(), "c.out")
		runCovered(t, bin, pkgDir, deadlineTests, profile)
		raw, err := os.ReadFile(profile)
		if err != nil {
			t.Fatalf("run %d wrote no coverprofile: %v", i, err)
		}
		m := row.FindSubmatch(raw)
		if m == nil {
			t.Fatalf("run %d: no coverage block starts on lockwithin.go line 50 (the hand-off wait after a lost abandon swap)", i)
		}
		if string(m[1]) == "0" {
			uncovered++
		}
	}
	if uncovered != 0 {
		t.Errorf("lockwithin.go:50 was uncovered in %d of %d isolated runs: the contention branch is reached by timing, not by a barrier", uncovered, isolatedRuns)
	}
}

func TestC1857_002_TheFullFlockSuiteStillCoversEveryLockWithinBlock(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	pkgDir := filepath.Join(goDir, "internal", "adapters", "flock")
	bin := buildCoverBinary(t, goDir)
	profile := filepath.Join(t.TempDir(), "full.out")

	runCovered(t, bin, pkgDir, ".", profile)

	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	zero := regexp.MustCompile(`(?m)^.*lockwithin\.go:\S+ [0-9]+ 0$`).FindAllString(string(raw), -1)
	if len(zero) != 0 {
		t.Errorf("lockwithin.go has %d uncovered block(s) after the full flock suite: %v", len(zero), zero)
	}
}
