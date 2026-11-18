//go:build acs

package cycle2515

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgechainPkg = "./internal/bridgechain"
	handoffPkg     = "./internal/handoff"
	policyPkg      = "./internal/policy"
	contractDoc    = "docs/architecture/phase-handoff-contract.md"
	walkerHeading  = "Where the judge seat is walked (H10)"
)

var walkerSymbols = []string{"WithCandidateFilter", "CandidateSpec", "LaunchSeat", "Independents"}

func requireTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	var tops []string
	for _, name := range names {
		top, _, _ := strings.Cut(name, "/")
		if len(tops) == 0 || tops[len(tops)-1] != top {
			tops = append(tops, top)
		}
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: "^(" + strings.Join(tops, "|") + ")$",
		Names:   names,
	})
}

func TestC2515_001_ARefusedSeatIsSkippedAndTheNextAdmittedSeatIsLaunched(t *testing.T) {
	requireTests(t, bridgechainPkg,
		"TestCandidateFilterExports_AreNamed",
		"TestWalking_CandidateFilterSkipsARefusedSeatAndLaunchesTheNext",
		"TestWalking_CandidateFilterIsEvaluatedPerTier",
		"TestWalking_CandidateFilterReadsATierAliasTheWayThePlanCeilingDoes")
}

func TestC2515_002_AFilterThatRefusesEverySeatErrorsWithZeroLaunchesAndChainAttemptCannotBypassIt(t *testing.T) {
	requireTests(t, bridgechainPkg,
		"TestWalking_CandidateFilterThatRefusesEverySeatLaunchesNothingAndErrors",
		"TestWalking_CandidateFilterAlsoRefusesTheCallersOwnCLIWhenThePlanIsEmpty",
		"TestWalking_ChainAttemptCannotBypassTheCandidateFilter",
		"TestWalking_ChainAttemptOnAnAdmittedSeatPassesThroughAsThatSeat")
}

func TestC2515_003_AFilteredWalkKeepsTheBenchTheChainAttemptHandoffAndTheSharedPlan(t *testing.T) {
	requireTests(t, bridgechainPkg,
		"TestWalking_FilteredWalkStillBenchesEachQuotaWall",
		"TestWalking_FilteredWalkHandsTheBridgeEachAttemptAsAChainAttempt",
		"TestWalking_CandidateFilterDoesNotMutateTheResolvedPlansCeiling",
		"TestWalking_CandidateFilterNeverWidensThePlansOwnCeiling")
}

func TestC2515_004_TheServedSeatIsTheRoutedDriverAndTheCoreBridgeEntryHonoursTheFilter(t *testing.T) {
	requireTests(t, bridgechainPkg,
		"TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead",
		"TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead/cli_fallback_without_a_filter",
		"TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead/cli_fallback_under_a_permissive_filter",
		"TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead/tier_step-down_after_every_cli_walls",
		"TestWalking_LaunchHonoursTheCandidateFilterOnTheCoreBridgeEntry")
}

func TestC2515_005_IndependentsAdmitsOnlyAnotherFamilyOrAStrongerTierAndFailsClosedOnAnUnknownAuthor(t *testing.T) {
	requireTests(t, handoffPkg,
		"TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier",
		"TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier/author_tier_empty_refuses_a_same-family_top_seat",
		"TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier/unattributed_author_refuses_another_family",
		"TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier/alias_of_the_author_tier_is_the_same_tier")
}

func TestC2515_006_TheJudgeFilterWalksThroughBridgechainAndNeverSeatsTheAuthorsOwnModel(t *testing.T) {
	requireTests(t, bridgechainPkg,
		"TestWalking_IndependentsFilterLeavesTheJudgeOnlyAnotherFamilyOrAStrongerTier",
		"TestWalking_IndependentsFilterRefusesTheAuthorsOwnModelAndLaunchesNothing")
}

func TestC2515_007_PolicyTierNamesEqualTheCanonicalCatalogTiers(t *testing.T) {
	requireTests(t, policyPkg,
		"TestTierNames_EqualTheCanonicalCatalogTiers",
		"TestTierRankAndName_FollowTheCanonicalCatalogOrder")
}

func TestC2515_008_TheNewSeamTestsPassOnTheTreeAndFailOnTheBaseProductionFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	bridgechainNames := []string{
		"TestWalking_CandidateFilterSkipsARefusedSeatAndLaunchesTheNext",
		"TestWalking_CandidateFilterThatRefusesEverySeatLaunchesNothingAndErrors",
		"TestWalking_LaunchSeatReportsTheServedFallbackSeatNotThePlanHead",
		"TestWalking_ChainAttemptCannotBypassTheCandidateFilter",
	}
	handoffNames := []string{"TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier"}
	requireTests(t, bridgechainPkg, bridgechainNames...)
	requireTests(t, handoffPkg, handoffNames...)

	baseBridgechain, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", "HEAD:go/internal/bridgechain/bridgechain.go")
	if err != nil || code != 0 {
		t.Fatalf("read the base bridgechain.go: code=%d err=%v stderr=%s", code, err, stderr)
	}
	scratch := t.TempDir()
	baseFile := filepath.Join(scratch, "bridgechain_base.go")
	if err := os.WriteFile(baseFile, []byte(baseBridgechain), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(scratch, "overlay.json")
	replace := map[string]string{
		filepath.Join(goDir, "internal", "bridgechain", "bridgechain.go"): baseFile,
		filepath.Join(goDir, "internal", "handoff", "independents.go"):    "",
	}
	raw, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	for pkg, names := range map[string][]string{bridgechainPkg: bridgechainNames, handoffPkg: handoffNames} {
		out, errOut, exit, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-overlay="+overlay,
			"-run", "^("+strings.Join(names, "|")+")$", pkg)
		if exit == 0 {
			t.Errorf("%s: the new tests pass on the base production files, so they do not need the change:\n%s%s", pkg, out, errOut)
		}
	}
}

func TestC2515_009_TheContractDocNamesTheSeamAndItsNewTextPassesTheSteLint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(contractDoc)))
	if err != nil {
		t.Fatalf("read %s: %v", contractDoc, err)
	}
	section := headingSection(string(raw), walkerHeading)
	if section == "" {
		t.Fatalf("RED: %s has no %q section", contractDoc, walkerHeading)
	}
	for _, symbol := range walkerSymbols {
		if !regexp.MustCompile(`\b` + symbol + `\b`).MatchString(section) {
			t.Errorf("RED: the %q section does not name %s", walkerHeading, symbol)
		}
	}
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	for _, finding := range stelint.Check([]byte(section), stelint.Options{Words: words}) {
		if !finding.IsSkipNotice() {
			t.Errorf("STE finding %s: %s: %q", finding.Rule, finding.Message, finding.Excerpt)
		}
	}
	guardRow := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "A judge seat is not the model of the author") {
			guardRow = line
			break
		}
	}
	if !strings.Contains(guardRow, "TestIndependents_AdmitsOnlyAnotherFamilyOrAStrongerTier") {
		t.Errorf("RED: the guard row of the judge seat does not name the Independents test: %q", guardRow)
	}
}

func headingSection(doc, heading string) string {
	lines := strings.Split(doc, "\n")
	start, level := -1, 0
	for i, line := range lines {
		if strings.HasPrefix(line, "#") && strings.Contains(line, heading) {
			start, level = i+1, headingLevel(line)
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if n := headingLevel(lines[i]); n > 0 && n <= level {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func headingLevel(line string) int {
	n := len(line) - len(strings.TrimLeft(line, "#"))
	if n == 0 || !strings.HasPrefix(line[n:], " ") {
		return 0
	}
	return n
}
