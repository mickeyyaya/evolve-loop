//go:build acs

package cycle2104

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
			t.Errorf("test %s did NOT pass in %s (missing, failing, or not compiled).\ncombined go-test output:\n%s", name, pkg, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2104_001_CodexPlanRuleAnswersFiveOptionAndWrappedPickers(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestCodexPlanRuleTailHeadroom_MatchesFiveOptionAndWrappedForms",
	)
}

func TestC2104_002_CodexPlanRuleWindowLeavesSlackOverTheLargestPicker(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestCodexPlanRuleTailHeadroom_WindowLeavesSlackOverTheLargestForm",
	)
}

func TestC2104_003_OverLongPickerIsReportedUnmatchedAndWindowStaysBounded(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestCodexPlanRuleTailHeadroom_OverLongFormIsUnmatched",
		"TestCodexPlanRuleTailHeadroom_WindowStaysBounded",
		"TestCodexPlanRuleTailHeadroom_FixturesFitTheSpanBoundWithoutAWindow",
	)
}

func TestC2104_004_AnsweredPickersNeverRefireAfterTheWindowWidens(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestCodexPlanRuleTailHeadroom_AnsweredFormsStayUnmatched",
		"TestAutoRespond_PlanModeDialogs",
		"TestAutoRespond_PlanModeAnswersEveryLiveQuestion",
		"TestAutoRespond_PlanModeDoesNotMatchThisRepositorysOwnFiles",
	)
}
