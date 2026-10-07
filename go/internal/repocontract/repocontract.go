// Package repocontract owns ship's repo-contract fixed scanner pack: its suite
// list and whether it runs for a tree, shared by ship's gate and the build floor.
package repocontract

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

type thisPackage struct{}

func evolveLoopModule() string {
	module, _, _ := strings.Cut(reflect.TypeOf(thisPackage{}).PkgPath(), "/internal/")
	return module
}

func Packages() []string {
	return []string{
		"./internal/phasespec/...",
		"./internal/profiles/...",
		"./internal/phasecoherence/...",
		"./internal/routingtest/...",
		"./internal/rawgitratchet/...",
		"./internal/sizeratchet/...",
		"./internal/testmainexit/...",
		"./internal/repocontract/...",
		"./internal/policy/...",
		"./internal/guards/...",
		"./internal/acssuite/...",
		"./internal/fleet/...",
		"./internal/evalqualitycheck/...",
		"./internal/inboxrank/...",
	}
}

type TestSelection struct {
	Package string
	Tests   []string
}

func TreeReadingTests() []TestSelection {
	selections := make([]TestSelection, 0, len(treeReadingSelections))
	for _, selection := range treeReadingSelections {
		selections = append(selections, TestSelection{Package: selection.Package, Tests: slices.Clone(selection.Tests)})
	}
	return selections
}

var treeReadingSelections = []TestSelection{
	{Package: "./cmd/evolve", Tests: []string{
		"TestApicoverSubcommand_ByteParityWithStandalone",
		"TestBridgeEngineRootsAreEnumerated",
		"TestCLIUpdateWiring_ABootTimeoutWithADrainedWindowIsAVerifiedQuotaCauseAndBenchesUntilTheReset",
		"TestCLIUpdateWiring_ABootTimeoutWithHealthyUsageStaysABootTimeout",
		"TestCenterlessConfigLoadSitesArePinned",
		"TestChainEngines_OneConstructionSite",
		"TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent",
		"TestChainResult_IsTheLeafResultWithNoReSpelledSchema",
		"TestClihealthUsage_PrintsEachCLIsTypedWindowsAndWritesNothing",
		"TestClihealthUsage_TheTableNamesEveryWindowAndAFailedProbeExitsOne",
		"TestComposedApicoverGate_TargetRecipeEnforces",
		"TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport",
		"TestConsoleSinkThresholdHasOneHome",
		"TestEmitLoopWave_ProjectsToTheLeafProducerWithOneRegistrar",
		"TestInboxCenterlessRootsArePinned",
		"TestLoopPreflightOptions_AFailedBootIsExplainedByTheUsageEvidence",
		"TestNewUsageProber_ReadsEachCLIsWindowsThroughItsManifestAndBenchesTheRightFamily",
		"TestNilSignalBridgeRootsAreExplicit",
		"TestNilSignalCenterRootsArePinned",
		"TestObserverAdapterConstructionsAreWired",
		"TestRegistryPathSpelling_HasOneNonTestHome",
		"TestRoutingConfigLoader_ConstructionSitesArePinned",
		"TestRunLoopChain_SetsBoundaryRefreshOnReExecStop",
		"TestUnobservedLedgerRootsArePinned",
		"TestWaveEngine_OneConstructionSite",
		"TestWireBridgeStages_IsTheRootsOnlyStageForwarding",
	}},
	{Package: "./internal/bridge", Tests: []string{
		"TestCompletionContractVocabulary_EveryRequestNamesItsContractByTheTypedConstant",
		"TestCompletionContractVocabulary_SpelledOnce",
		"TestLaunchOutcome_OneClassificationSite",
	}},
	{Package: "./internal/changedpkgs", Tests: []string{
		"TestCoveringTests_ReachableFromProduction",
		"TestDirectImporters_ReachableFromProduction",
	}},
	{Package: "./internal/core", Tests: []string{
		"TestAgentSubprocessWriters_AllStampRunID",
		"TestAgentSubprocessWriters_SetIsClosed",
		"TestCarryoverLifecycle_OneConstructionSite",
		"TestFailureDiagWriter_OneConstructionSite",
		"TestFailureLearningEngine_OneConstructionSite",
		"TestPhaseAdvisor_OneConstructionSite",
		"TestPhaseTimings_SingleWriter",
	}},
	{Package: "./internal/cycleoutcome", Tests: []string{
		"TestLaneScopeProjection_SingleWireShapeDeclaration",
	}},
	{Package: "./internal/inboxmover", Tests: []string{
		"TestOptionsMover_OneConstructionSite",
	}},
	{Package: "./internal/inboxmover/lifecycle", Tests: []string{
		"TestLifecycle_OnlyHostImportsTheLeaf",
	}},
	{Package: "./internal/phaseobserver", Tests: []string{
		"TestObserverEngine_OneConstructionSite",
	}},
	{Package: "./internal/phases/audit", Tests: []string{
		"TestArtifactNames_HaveOneProductionSpellingEach",
		"TestCIParityGates_OneConstructionSite",
		"TestDefectLedgerSeam_OneConstructionSite",
		"TestNullLedgerFacades_HaveNoProductionCaller",
	}},
	{Package: "./internal/phases/runner", Tests: []string{
		"TestVerdictEngine_OneConstructionSite",
	}},
	{Package: "./internal/phases/ship", Tests: []string{
		"TestLanding_OneConstructionSite",
	}},
	{Package: "./internal/reachabilityprobe", Tests: []string{
		"TestBuildImportGraph_Named",
	}},
	{Package: "./internal/subagent", Tests: []string{
		"TestSubagentRun_OneConstructionSite",
	}},
}

func ModuleDir(root string) string {
	return filepath.Join(root, "go")
}

func GateOn(gate string) bool {
	return gate != "" && gate != "off"
}

func PackRuns(gate, root string) (runs bool, note string) {
	if !GateOn(gate) {
		return false, ""
	}
	if dir := ModuleDir(root); modulePath(filepath.Join(dir, "go.mod")) != evolveLoopModule() {
		return false, fmt.Sprintf("%s does not declare %s, so the fixed scanner pack's guard suites are not in this tree; the pack is skipped", dir, evolveLoopModule())
	}
	if gate != "enforce" {
		return true, fmt.Sprintf("unknown stage %q — treating as enforce (a typo must not silently disable a red-main guard)", gate)
	}
	return true, ""
}

func modulePath(goMod string) string {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "module" {
			return strings.Trim(fields[1], "\"`")
		}
	}
	return ""
}
