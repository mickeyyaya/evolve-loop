//go:build acs

package cycle987

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	evalgatePkg = "github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
)

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

func TestC987_001_ShipGateStaleAttestationBoundInDefaultSuite(t *testing.T) {
	assertDefaultSuiteTestsPass(t, shipPkg,
		"TestShipGate_StaleAttestationBlocked",
		"TestShipGate_FreshAttestationPasses",
		"TestShipGate_MissingAttestationBlocked",
	)
}

func TestC987_002_QualityGateWiredIntoReviewerBound(t *testing.T) {
	assertDefaultSuiteTestsPass(t, evalgatePkg,
		"TestQualityGate_WiredIntoReviewer",
		"TestNewReviewer_TautologyEvalBlocksAtEnforce",
	)
}
