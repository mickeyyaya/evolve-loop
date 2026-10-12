//go:build acs

package cycle1860

import (
	"path/filepath"
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

func TestC1860_001_SteDisplayKeepsDotDotPrefixedRootFilesRelative(t *testing.T) {
	runNamedTest(t, "./cmd/evolve", "^TestSteDisplay_RootFilesNamedWithTwoLeadingDotsStayRelative$")
}

func TestC1860_002_SteLintCLIShowsDotDotPrefixedRootFileRelative(t *testing.T) {
	runNamedTest(t, "./cmd/evolve", "^TestDocsSteLint_ADotDotPrefixedRootFileShowsItsRelativePath$")
}

func TestC1860_003_FormatVerbParsesIndexedWidthAndPrecision(t *testing.T) {
	runNamedTest(t, "./internal/stelint", "^TestCheckGo_IndexedWidthAndPrecisionVerbsAreCodeTokens$")
}
