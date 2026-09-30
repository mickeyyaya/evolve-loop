//go:build acs

package cycle1073

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const topngatePkg = "github.com/mickeyyaya/evolve-loop/go/internal/topngate"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	args := []string{"test", "-count=1", pkg}
	if pattern != "" {
		args = []string{"test", "-run", "^(" + pattern + ")$", "-count=1", pkg}
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1073_001_LabelDriftAtTriageToTDDIsAdvisory(t *testing.T) {
	ok, out := runGoTest(t, topngatePkg, "TestTDDScopeGate_LabelDriftIsAdvisory")
	if !ok {
		t.Errorf("tddScopeGate still HARD-BLOCKS on a differently-labelled-but-committed task — the same false-rejection defect #348 closed for the build gate, one phase earlier:\n%s", out)
	}
}

func TestC1073_002_EmptyTopNAuthoringStaysFatal(t *testing.T) {
	ok, out := runGoTest(t, topngatePkg, "TestTDDScopeGate_EmptyTopNStillBlocks")
	if !ok {
		t.Errorf("the advisory conversion overreached: orphan TDD authoring under an EMPTY ## top_n must stay a hard block (gate.go's documented case-1 carve-out):\n%s", out)
	}
}

func TestC1073_003_TopngatePackageSuiteGreen(t *testing.T) {
	ok, out := runGoTest(t, topngatePkg, "")
	if !ok {
		t.Errorf("the topngate package suite is not green — the gate change broke a fail-open path or the sibling build gate:\n%s", out)
	}
}

func TestC1073_004_TouchedPackageVetsClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", topngatePkg)
	out := strings.TrimSpace(stdout + stderr)
	if code < 0 {
		t.Fatalf("go vet failed to launch: code=%d err=%v\n%s", code, err, out)
	}
	if code != 0 || out != "" {
		t.Errorf("go vet %s must be clean; exit=%d output:\n%s", topngatePkg, code, out)
	}
}
