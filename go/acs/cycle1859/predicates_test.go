//go:build acs

package cycle1859

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedTest(t *testing.T, pattern string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", pattern, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Errorf("go test -run %s exit=%d err=%v\n%s\n%s", pattern, code, err, strings.TrimSpace(out), strings.TrimSpace(errOut))
	}
}

func TestC1859_001_FailedDevCreateLeavesNoBranch(t *testing.T) {
	runNamedTest(t, "^TestWorktreeDev_FailedCreateLeavesNoBranch$")
}

func TestC1859_002_UncreatableDevDirCaseRequiresNoBranch(t *testing.T) {
	runNamedTest(t, "^TestRunWorktreeCreateDev_RefusalsAndGitFailures$/the_dev_dir_cannot_be_created")
}
