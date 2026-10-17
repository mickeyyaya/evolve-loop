//go:build acs

package cycle1926

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	runnerPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	bridgechainPkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
)

func runGoTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC1926_001_ADeepSeatLogsTheCLIThatTheTierCeilingDispatches(t *testing.T) {
	runGoTests(t, runnerPkg, "TestLogDecision_ADeepSeatWhoseChainStartsWithAForbiddenCLILogsTheDispatchedCLI")
}

func TestC1926_002_ABenchEndingOnALaterDayPrintsItsDateInTheReorderLine(t *testing.T) {
	runGoTests(t, bridgechainPkg, "TestApplyCLIHealthBench_TheReorderLineCarriesTheDateOfABenchEndingOnALaterDay")
}

func TestC1926_003_TheBridgeLaunchesTheCLITheRunnerLogged(t *testing.T) {
	runGoTests(t, runnerPkg, "TestRun_TheBridgeLaunchesTheCLITheRunnerLogged")
}

func TestC1926_004_ExistingRoutingLogContractsStayGreen(t *testing.T) {
	runGoTests(t, runnerPkg,
		"TestLogDecision_EveryRoutingTraceLineIsLoggedUnderThePhase",
		"TestLogDecision_ThePolicyPinAdvisorOverlayAndDefaultEachGetTheirOwnLine",
		"TestResolveDispatchPlan_TheInjectedRouterDecidesThePlan",
	)
	runGoTests(t, bridgechainPkg, "TestApplyCLIHealthBench_DemotesABenchedFamily")
}
