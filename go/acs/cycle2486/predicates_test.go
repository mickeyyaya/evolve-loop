//go:build acs

package cycle2486

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runCoreTests(t *testing.T, names ...string) {
	t.Helper()
	moduleDir := filepath.Join(acsassert.RepoRoot(t), "go")
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", moduleDir, "-count=1", "-v", "-run", pattern, "./internal/core")
	if err != nil || code != 0 {
		t.Fatalf("RED: core tests %v exit=%d: %v\n%s", names, code, err, acsassert.OutputTail(out+stderr, 14))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass:\n%s", name, acsassert.OutputTail(out, 8))
		}
	}
}

func TestC2486_001_AResumedCycleContinuesAtItsRecordedRung(t *testing.T) {
	runCoreTests(t,
		"TestResumedCycle_ContinuesAtItsRecordedRung",
		"TestResumedCorrection_TheRaisedRungReachesTheNextPhase",
	)
}

func TestC2486_002_AQuotaPausedResumeKeepsAndContinuesAtTheRaisedRung(t *testing.T) {
	runCoreTests(t,
		"TestResumedCorrection_AQuotaWallPausesAtTheRaisedRungAndTheNextResumeContinuesThere",
		"TestResumedCorrection_ShadowQuotaPauseLeavesTheNextResumeDispatchUnchanged",
	)
}

func TestC2486_003_LaterLadderProgressOfAResumedRunSurvivesAdoptionOfTheKeptRung(t *testing.T) {
	runCoreTests(t,
		"TestAdoptedResumedLadder_ProgressMadeAfterTheAdoptionSurvivesTheNextDispatch",
		"TestAdoptedResumedLadder_ProgressMadeAfterTheAdoptionSurvivesAQuotaPause",
		"TestResumedRun_LadderProgressAfterTheResumedPhaseSurvivesTheNextDispatch",
		"TestResumedRun_AQuotaWallAfterLaterLadderProgressCheckpointsTheLaterRung",
	)
}
