//go:build acs

package cycle1954

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func requirePass(t *testing.T, pattern string, tests ...string) {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(acsassert.RepoRoot(t), "go"),
		"test", "-count=1", "-v", "-run", pattern, "./internal/policy")
	if err != nil && code == 0 {
		t.Fatalf("go test did not run: %v", err)
	}
	all := out + stderr
	if code != 0 {
		t.Errorf("RED: go test -run %s ./internal/policy exit=%d\n%s", pattern, code, all)
	}
	for _, name := range tests {
		if !strings.Contains(all, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass\n%s", name, all)
		}
	}
	if strings.Contains(all, "--- SKIP") {
		t.Errorf("a selected test skipped:\n%s", all)
	}
}

func TestC1954_001_AnUnknownHandoffPairIsRefusedAtDecode(t *testing.T) {
	requirePass(t, "^TestHandoffPolicy_(UnknownPairRefusedAtDecode|AnUnknownPairErrorNamesThePairAndTheKnownOnes|AnUnknownPairBesideAKnownOneStillRefused|AnUnknownKeyInsideAPairRefused)$",
		"TestHandoffPolicy_UnknownPairRefusedAtDecode",
		"TestHandoffPolicy_AnUnknownPairErrorNamesThePairAndTheKnownOnes",
		"TestHandoffPolicy_AnUnknownPairBesideAKnownOneStillRefused",
		"TestHandoffPolicy_AnUnknownKeyInsideAPairRefused")
}

func TestC1954_002_WithNoHandoffBlockEveryPairResolvesToShadow(t *testing.T) {
	requirePass(t, "^TestHandoffPolicy_(AnAbsentBlockResolvesEveryPairToShadow|AFileWithoutTheBlockResolvesEveryPairToShadow|AnEmptyBlockResolvesEveryPairToShadow|EnforceOnOnePairLeavesTheOthersAtShadow|EveryPairTakesItsOwnStageAndJudgeTier|ANamedPairWithoutAStageStaysAtShadow|ReturnsACopyTheCallerCannotCorrupt|TheLoadedBlockIsNotMutatedByItsConfig)$",
		"TestHandoffPolicy_AnAbsentBlockResolvesEveryPairToShadow",
		"TestHandoffPolicy_AFileWithoutTheBlockResolvesEveryPairToShadow",
		"TestHandoffPolicy_AnEmptyBlockResolvesEveryPairToShadow",
		"TestHandoffPolicy_EnforceOnOnePairLeavesTheOthersAtShadow",
		"TestHandoffPolicy_EveryPairTakesItsOwnStageAndJudgeTier",
		"TestHandoffPolicy_ANamedPairWithoutAStageStaysAtShadow",
		"TestHandoffPolicy_ReturnsACopyTheCallerCannotCorrupt",
		"TestHandoffPolicy_TheLoadedBlockIsNotMutatedByItsConfig")
}

func TestC1954_003_AStageOtherThanShadowOrEnforceIsRefused(t *testing.T) {
	requirePass(t, "^TestHandoffPolicy_(InvalidStageRefused|AStageThatIsNotAStringRefused|ABlockThatIsNotAnObjectRefused)$",
		"TestHandoffPolicy_InvalidStageRefused",
		"TestHandoffPolicy_AStageThatIsNotAStringRefused",
		"TestHandoffPolicy_ABlockThatIsNotAnObjectRefused")
}

func TestC1954_004_TheHandoffBlockIsDocumentedInTheRuntimeReference(t *testing.T) {
	requirePass(t, "^TestRuntimeReference_DocumentsTheHandoffBlock$", "TestRuntimeReference_DocumentsTheHandoffBlock")
}

func TestC1954_005_TheCheckedInPolicyFileStaysWithoutAHandoffBlock(t *testing.T) {
	requirePass(t, "^TestHandoffPolicy_TheCheckedInPolicyFileCarriesNoHandoffBlock$", "TestHandoffPolicy_TheCheckedInPolicyFileCarriesNoHandoffBlock")
}
