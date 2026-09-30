//go:build acs

package cycle1532

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1532_001_JudgmentLessonReachesNextCyclePlannerPrompt(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestJudgmentLessonEndToEnd_ReachesNextCyclePlannerPrompt",
		"TestJudgmentLessonEndToEnd_SurvivesPlannerPromptCapUnderCrowdedCarryover",
		"TestJudgmentLessonEndToEnd_ControlPhaseFAILReachesNoPlannerPrompt",
	)
}

func TestC1532_002_TimeoutDiagnosticNeverMutatesRetryOrFailureLearning(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning",
	)
}

func TestC1532_003_TransientRecognitionIsManifestScopedNotHardCoded(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestArtifactTimeoutTransient_LivePaneIsFamilyScopedNotHardCodedText",
		"TestArtifactTimeoutTransient_EchoedProviderTextNeverClassifiesForAnyFamily",
	)
}

func TestC1532_004_LandedRegressionsNotWeakened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRecordAndBranch_PremiseChallengeFAILTeaches",
		"TestRecordAndBranch_PremiseChallengeFAILDoesNotRecordFailedApproach",
		"TestJudgmentLesson_ControlPhasesDoNotTeach",
		"TestJudgmentLesson_PersistedToStorage",
		"TestJudgmentLesson_AuthoritativePhaseYieldsToFloorRecorder",
	)
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane",
		"TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient",
		"TestClassifyTransientPane_IgnoresEchoedPromptText",
		"TestClassifyTransientPane_UnknownDriverFailsOpen",
	)
}
