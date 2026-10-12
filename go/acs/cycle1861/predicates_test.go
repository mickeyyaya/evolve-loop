//go:build acs

package cycle1861

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runGCTest(t *testing.T, name string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", name, "./internal/gc")
	if err != nil || code != 0 {
		t.Fatalf("go test -run %s ./internal/gc exit=%d err=%v\n%s\n%s", name, code, err, out, errOut)
	}
}

func TestC1861_001_SquashLandedBranchIsDeletedUnderNonEnglishLocale(t *testing.T) {
	runGCTest(t, "^TestDeleteBranch_SquashLandedDeleteWithNonEnglishLocale$")
}

func TestC1861_002_UnmergedBranchStaysRefusedUnderLocaleOverride(t *testing.T) {
	runGCTest(t, "^TestDeleteBranch_UnmergedBranchStaysRefusedUnderLocaleOverride$")
}
