//go:build acs

package cycle2447

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamed(t *testing.T, pkg string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2447_001_AUsageReadingAtTheLimitStopsTheLoopAtItsNextBoundaryWithNoConsole(t *testing.T) {
	runNamed(t, "./cmd/evolve",
		"TestUsageBrake_AReadingAtTheLimitStopsTheLoopAtTheNextBoundaryWithNoConsolePresent",
		"TestUsageBrake_TheBoundaryProceedsWhenTheReadingIsUnderTheLimitTheBrakeIsOffOrTheWindowReset",
	)
}

func TestC2447_002_ANewSeatPastTheLimitIsRefusedAndTheTextNamesTheLimitTheReadingAndTheReset(t *testing.T) {
	runNamed(t, "./internal/cli/agentcmd",
		"TestUsageBrake_ASeatPastTheLimitIsRefusedAndTheTextNamesTheLimitTheReadingAndTheReset",
		"TestUsageBrake_ASeatUnderTheLimitOrWithTheBrakeOffOrAfterTheResetStillRuns",
	)
}

func TestC2447_003_AnUnreadableReadingIsNotUnderTheLimitForTheLoopOrForASeat(t *testing.T) {
	t.Run("loop boundary", func(t *testing.T) {
		runNamed(t, "./cmd/evolve",
			"TestUsageBrake_AnUnreadableReadingIsNotUnderTheLimitAndStopsTheLoopNamingTheFile",
			"TestUsageBrake_AnUnreadableReadingDoesNotStopTheLoopWhenTheBrakeIsOff",
		)
	})
	t.Run("seat launch", func(t *testing.T) {
		runNamed(t, "./internal/cli/agentcmd",
			"TestUsageBrake_AnUnreadableReadingIsNotUnderTheLimitAndASeatIsRefused",
			"TestUsageBrake_AnUnreadableReadingDoesNotRefuseASeatWhenTheBrakeIsOff",
		)
	})
}

func TestC2447_004_TheHoldPercentIsAPolicyKeyOfTheCLIHealthBlockAndAbsentMeansOff(t *testing.T) {
	runNamed(t, "./internal/policy",
		"TestUsageBrake_TheHoldPercentComesFromTheCLIHealthBlockAndAbsentMeansOff",
	)
}
