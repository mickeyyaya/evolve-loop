//go:build acs

package cycle2101

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func assertNamedTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("test %s did NOT pass in %s (missing, failing, or not compiled).\ncombined go-test output:\n%s", name, pkg, out)
		}
	}
}

func TestC2101_001_TailScopedPendingClearsWhenTheDialogLeavesTheTail(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestAutoRespondTail_ScrolledUpDialogClearsPendingForATailScopedRule",
		"TestAutoRespondTail_DialogStillInsideTheTailStaysPending",
	)
}

func TestC2101_002_DecideAndResolveAgreeOnTheTailWindowAtEveryBoundary(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestAutoRespondTail_DecideAndResolveAgreeAtEveryTailBoundary",
	)
}

func TestC2101_003_RuleWithoutATailStillMatchesTheWholePane(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestAutoRespondTail_RuleWithoutATailKeepsMatchingTheWholePane",
	)
}

func TestC2101_004_HumanInputSentKeysLineNamesTheFiredRule(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestAutoRespondAttribution_HumanInputSentKeysLineNamesTheRule",
		"TestAutoRespondAttribution_AutoPathStillNamesTheRule",
	)
}
