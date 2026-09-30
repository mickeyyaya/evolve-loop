//go:build acs

package cycle1155

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"
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
			t.Errorf("binding test %s did NOT pass in %s (missing, renamed, or failing). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

func TestC1155_001_ReplanUnknownPhaseRecordsRejection(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestReplan_UnknownPhase_RecordsRejection")
}

func TestC1155_002_ReplanCleanNoSpuriousAndUpfrontSurvives(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestReplan_KnownPhasesOnly_NoSpuriousRejection")
}

func TestC1155_003_MultipleReplansAllRejectionsRecorded(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestReplan_MultipleReplans_AllRejectionsRecorded")
}

func TestC1155_004_CoreAndRouterSuitesGreen(t *testing.T) {
	for _, pkg := range []string{corePkg, routerPkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", pkg)
		if code == -1 {
			t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
		}
		if code != 0 {
			t.Errorf("regression: %s is not green (exit=%d)\n%s", pkg, code, stdout+stderr)
		}
	}
}
