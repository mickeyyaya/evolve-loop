//go:build acs

package cycle2481

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func passes(t *testing.T, pkg string, names ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	if code < 0 {
		t.Fatalf("go test %s did not start: %v", pkg, err)
	}
	out := stdout + stderr
	if code != 0 {
		t.Errorf("go test %s exit %d\n%s", pkg, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s: %s did not run and pass", pkg, name)
		}
	}
}

func TestC2481_001_AnItemWhoseTextNamesAProtectedFileGetsNoRerouteProposal(t *testing.T) {
	passes(t, "./internal/inboxcurator",
		"TestReroutes_AnItemWhoseTextNamesAProtectedFileGetsNoRerouteProposal",
		"TestReroutes_TheFixSurfaceHookRefusesAProtectedFileTheItemNeverNames")
	passes(t, "./cmd/evolve", "TestCmd_InboxCurate_AnItemWhoseTextNamesAProtectedFileIsNotProposedForALane")
}

func TestC2481_002_AnItemWhoseFixIsAlreadyOnMainGetsAConsumeOrAStaleProposalAndNoReroute(t *testing.T) {
	passes(t, "./internal/inboxcurator",
		"TestReroutes_TheStaleHookStopsTheRerouteOfAnItemWhoseFixIsAlreadyOnMain",
		"TestCurate_AnItemWhoseProbesPassOnTheTreeGetsAConsumeAndNoReroute",
		"TestCurate_AnItemTheStaleHookNamesGetsAJudgmentProposalAndNoReroute")
	passes(t, "./cmd/evolve",
		"TestCmd_InboxCurate_AStaleItemOfTheWave178BoundaryGetsAConsumeOrQuarantineProposalNotAReroute",
		"TestCmd_InboxCurate_ANamedPremiseThatIsStillAbsentOnTheTreeStopsTheReroute",
		"TestCmd_InboxCurate_ANamedPremiseThatLandedOnTheTreeSinceTheVerdictIsRerouted",
		"TestCmd_InboxCurate_AnAlreadyShippedVerdictWhoseCommitIsNotOnMainIsNotTrusted")
}

func TestC2481_003_TheWave178BoundaryProposesTheNineLaneTriageKeptAndNoneOfTheSevenItBounced(t *testing.T) {
	passes(t, "./internal/inboxcurator", "TestCurate_TheWave178BoundaryProposesTheNineLaneTriageKeptAndNoneOfTheSevenItBounced")
	passes(t, "./cmd/evolve", "TestCmd_InboxCurate_TheWave178BoundaryThroughTheProductionCuratorProposesTheNineKeptAndNoneOfTheSeven")
}
