//go:build acs

package cycle2128

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	catalogPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	return acsassert.RepoRoot(t) + "/go"
}

func runNamed(t *testing.T, pkg string, names []string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2128_001_ClaudeDeepChainIsFableOpusSonnetForALiveEntryOnly(t *testing.T) {
	runNamed(t, catalogPkg, []string{
		"TestDispatchModelChain_ClaudeDeepWalksFableThenOpusThenSonnet",
		"TestDispatchModelChain_TheTierModelLeadsEvenWhenTheFallbackListOrdersItLater",
		"TestDispatchModelChain_ANonLiveClaudeEntryFallsBackToTheManifest",
		"TestDispatchModelChain_IsThePrimaryThenTheFallbacksDedupedForALiveEntryOnly",
	})
}

func TestC2128_002_AWallOnTheFirstModelSucceedsOnTheSecondBeforeAnyEscalation(t *testing.T) {
	runNamed(t, bridgePkg, []string{
		"TestLaunch_AQuotaWallOnTheFirstModelOfATierSucceedsOnTheNextModel",
		"TestLaunch_TheAuditorSucceedsOnOpusWhenFableHitsItsQuotaWall",
		"TestLaunch_TheAuditorStepsPastTwoWalledModelsToTheThird",
	})
}

func TestC2128_003_TheQuotaExitIsTerminalOnlyWhenTheWholeChainWalls(t *testing.T) {
	runNamed(t, bridgePkg, []string{
		"TestLaunch_OnlyAWallOfTheWholeChainReturnsTheQuotaExit",
		"TestLaunch_TheAuditorReturnsTheQuotaExitOnlyWhenFableOpusAndSonnetAllWall",
		"TestLaunch_TheAuditorDoesNotLeaveFableForAPromptShapeProblem",
		"TestLaunch_OnlyAQuotaWallOrAnUnsupportedModelAdvancesTheChain",
	})
}

func TestC2128_004_AFlooredAuditorIsUnblockedForEveryClaudeModelOfTheChain(t *testing.T) {
	runNamed(t, bridgePkg, []string{
		"TestLaunch_TheAuditorKeepsFableWhenTheCatalogPrimaryIsOpus",
		"TestLaunch_TheFamilyFloorRecognisesEveryClaudeAliasOfTheLiveChain",
		"TestLaunch_AFlooredSeatSkipsAFallbackModelOutsideTheClaudeFamily",
		"TestLaunch_AnUnflooredSeatKeepsEveryFallbackModel",
	})
}
