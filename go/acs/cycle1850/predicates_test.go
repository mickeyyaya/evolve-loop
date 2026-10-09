//go:build acs

package cycle1850

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shippedReport = "## Verdict\n\nSHIPPED\n"

func classifyShippedCycleAgainstMainCommit(t *testing.T, cycleNum, commitMessage string) cycleclassify.Result {
	t.Helper()
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("checkout", "-B", "main")
	repo.Git("add", ".")
	repo.Git("commit", "-m", commitMessage)

	ws := filepath.Join(t.TempDir(), "cycle-"+cycleNum)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte(shippedReport), 0o644); err != nil {
		t.Fatal(err)
	}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo.Dir); err != nil {
		t.Fatal(err)
	}
	cycleclassify.SetHangClassifier(true)
	t.Cleanup(func() {
		cycleclassify.SetHangClassifier(false)
		_ = os.Chdir(prevWD)
	})
	return cycleclassify.Classify(ws)
}

func TestC1850_001_Cycle4NotMatchedByCycle42Commit(t *testing.T) {
	r := classifyShippedCycleAgainstMainCommit(t, "4", "cycle 42 shipped")
	if r.Class == cycleclassify.ClassExitTransportHang {
		t.Errorf("RED: cycle 4 reclassified as %q from a commit that only mentions cycle 42", r.Class)
	}
}

func TestC1850_002_Cycle4NotMatchedByCycle400Commit(t *testing.T) {
	r := classifyShippedCycleAgainstMainCommit(t, "4", "cycle 400 shipped")
	if r.Class == cycleclassify.ClassExitTransportHang {
		t.Errorf("RED: cycle 4 reclassified as %q from a commit that only mentions cycle 400", r.Class)
	}
}

func TestC1850_003_Cycle42StillMatchesItsOwnCommit(t *testing.T) {
	r := classifyShippedCycleAgainstMainCommit(t, "42", "cycle 42 shipped")
	if r.Class != cycleclassify.ClassExitTransportHang {
		t.Errorf("anchored grep over-narrowed: cycle 42 got %q want %q", r.Class, cycleclassify.ClassExitTransportHang)
	}
}

func TestC1850_004_Cycle4MatchesCommitWithTrailingPunctuation(t *testing.T) {
	r := classifyShippedCycleAgainstMainCommit(t, "4", "evolve: cycle 4, shipped")
	if r.Class != cycleclassify.ClassExitTransportHang {
		t.Errorf("anchored grep over-narrowed: cycle 4 got %q want %q", r.Class, cycleclassify.ClassExitTransportHang)
	}
}

// acs-predicate: config-check
func TestC1850_005_HangTestUsesLiveSwitchNotRemovedEnvFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "cycleclassify", "hang_coverage_test.go")
	if !acsassert.FileNotContains(t, path, "EVOLVE_HANG_CLASSIFIER") {
		t.Errorf("RED: hang test still sets the removed EVOLVE_HANG_CLASSIFIER flag")
	}
	if !acsassert.FileContains(t, path, "setHangClassifierForTest(t, true)") {
		t.Errorf("RED: hang test does not enable the classifier through setHangClassifierForTest(t, true)")
	}
}

// acs-predicate: config-check
func TestC1850_006_StdlibReplacesDuplicateTestHelpers(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, "go", "internal", "cycleclassify")
	if !acsassert.FileNotContains(t, filepath.Join(dir, "empty_output_test.go"), "func stringRepeat(") {
		t.Errorf("RED: stringRepeat duplicates strings.Repeat")
	}
	if !acsassert.FileNotContains(t, filepath.Join(dir, "echo_veto_c654_test.go"), "func itoa(") {
		t.Errorf("RED: itoa duplicates strconv.Itoa")
	}
}
