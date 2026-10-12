//go:build acs

package cycle1862

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedTest(t *testing.T, pkg, pattern string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", pattern, pkg)
	if err != nil || code != 0 {
		t.Errorf("go test -run %s %s exit=%d err=%v\n%s%s", pattern, pkg, code, err, stdout, stderr)
	}
}

func TestC1862_001_ACheckpointSaveCutByTheBudgetStillFinishesTheArchive(t *testing.T) {
	runNamedTest(t, "./internal/gc", "^TestApplyDevWorktrees_ArchiveRunsUnderDedicatedContext$")
}

func TestC1862_002_AWorktreeRemoveCutByTheBudgetStillFinishesTheArchive(t *testing.T) {
	runNamedTest(t, "./internal/gc", "^TestApplyDevWorktrees_ABudgetThatEndsAsTheRemoveStartsStillFinishesTheArchive$")
}

func TestC1862_003_ABudgetCutLeavesEachTreeUntouchedOrFullyArchived(t *testing.T) {
	runNamedTest(t, "./internal/gc", "^TestApplyDevWorktrees_BudgetCutLeavesTreeUntouchedOrFullyArchived$")
}

func TestC1862_004_EveryGCPackageTestStillPasses(t *testing.T) {
	runNamedTest(t, "./internal/gc", ".")
}

func TestC1862_005_TheGCChangeAddsNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "origin/main")
	if err != nil || code != 0 {
		t.Fatalf("git merge-base exit=%d err=%v", code, err)
	}
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", strings.TrimSpace(base), "--", "go/internal/gc")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d err=%v %s", code, err, stderr)
	}
	for _, line := range strings.Split(diff, "\n") {
		added := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") && (strings.HasPrefix(added, "//") || strings.HasPrefix(added, "/*")) {
			t.Errorf("added comment line in go/internal/gc: %q", line)
		}
	}
}
