package repocontract

import (
	"slices"
	"strings"
)

const WholeRepository = "/"

type DataFileRead struct {
	Path string
	TestSelection
}

func DataFileReads() []DataFileRead {
	rows := make([]DataFileRead, 0, len(dataFileReads))
	for _, row := range dataFileReads {
		rows = append(rows, DataFileRead{Path: row.Path, TestSelection: TestSelection{Package: row.Package, Tests: slices.Clone(row.Tests)}})
	}
	return rows
}

func ByNameTests() []TestSelection {
	return mergeSelections(treeReadingSelections, dataFileReads)
}

func UnknownChange() []string {
	return []string{WholeRepository}
}

func ByNameTestsFor(changed []string) []TestSelection {
	if slices.ContainsFunc(changed, func(path string) bool { return !strings.HasSuffix(path, ".go") }) {
		return ByNameTests()
	}
	always := slices.DeleteFunc(slices.Clone(dataFileReads), func(row DataFileRead) bool {
		return row.Path != WholeRepository && !strings.HasPrefix(row.Path, "go/")
	})
	return mergeSelections(treeReadingSelections, always)
}

func mergeSelections(tree []TestSelection, data []DataFileRead) []TestSelection {
	var order []string
	byPackage := map[string][]string{}
	add := func(selection TestSelection) {
		if _, seen := byPackage[selection.Package]; !seen {
			order = append(order, selection.Package)
		}
		byPackage[selection.Package] = append(byPackage[selection.Package], selection.Tests...)
	}
	for _, selection := range tree {
		add(selection)
	}
	for _, row := range data {
		add(row.TestSelection)
	}
	merged := make([]TestSelection, 0, len(order))
	for _, pkg := range order {
		tests := byPackage[pkg]
		slices.Sort(tests)
		merged = append(merged, TestSelection{Package: pkg, Tests: slices.Compact(tests)})
	}
	return merged
}

var dataFileReads = []DataFileRead{
	{Path: ".evolve/policy.json", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestAgyClaudeRouting_AFloorSeatNeedsTheLaunchModelCheck",
		"TestAgyClaudeRouting_ANonFloorSeatNeedsTheLaunchModelCheck",
		"TestDeepAndTopProfiles_NoChainEntryResolvesTheirTierToAGeminiProModel",
		"TestGC_TheEnforceHookAndTheOperatorRunKeepTheLedgerEvidenceStore",
		"TestRouter_EveryLaunchOfTheAdvisorStaysOnClaudeModels",
		"TestRunGCHook_TheCheckedInEnforcePolicyNeverTouchesADevWorktreeALiveLaneAnUnmergedBranchOrAContinuationSnapshot",
		"TestTheCheckedInCatalogFiltersEachTargetToItsModelFamily",
		"TestTheCheckedInPolicyRunsTheWaveEndGCInEnforce",
	}}},
	{Path: ".evolve/profiles/", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestAgyClaudeRouting_AFloorSeatNeedsTheLaunchModelCheck",
		"TestAgyClaudeRouting_ANonFloorSeatNeedsTheLaunchModelCheck",
		"TestDeepAndTopProfiles_NoChainEntryResolvesTheirTierToAGeminiProModel",
		"TestModelClassifierProfile_DeniesBashAndTheWeb",
		"TestModelClassifierProfile_GrantMatchesTheArtifactOfTheWorkspace",
		"TestRouter_EveryLaunchOfTheAdvisorStaysOnClaudeModels",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestAgentRunBridge_TheSeatPromptHoldsTheRequiredSectionsOfThePhaseContract",
		"TestAgentRun_EachDiffReviewerRunsOnANonClaudeFamily",
		"TestAgyClaudeRouting_TheFloorStillAdmitsOnlyClaudeCode",
		"TestCLIRoutingShow_EveryShippedProfileKeepsItsResolvedRoute",
		"TestConsoleBuildSeat_RoutingDataMovesTheSeatToAnotherFamily",
		"TestConsoleReviewChain_ANonClaudeSeatReviewsAndTheGateAttestsTheBridgeRecords",
		"TestDecidePRWindow_AFileNameThatIsNotJSONIsAnError",
		"TestDecidePRWindow_EachUnreadableInputIsAnError",
		"TestDecidePRWindow_ReadsThePRFilesWithTheirOldNamesThroughGH",
		"TestNewPRMerger_AnUnreadableWindowStageFailsAndTheOpenGateIsRead",
		"TestPRBoundary_AWindowPRDoesNotWaitAndTheBoundComesFromPolicy",
		"TestPRBoundary_FailsLoudOnWhatItCannotRead",
		"TestPRBoundary_NoLiveLoopIsAlreadyABoundary",
		"TestPRBoundary_TheLongestGreenWaitPastTheBoundWithAnUnderfilledPoolCallsAnEarlyBoundary",
		"TestPRMerge_ALoopThatStartsDuringTheWaitRefusesABoundaryPRAtTheMerge",
		"TestPRMerge_AnOpenWindowFailsLoudOnAnUnreadableWindowOrPlane",
		"TestPRMerge_AnOpenWindowWithAnIdlePlaneMergesWithoutTheWindowRule",
		"TestPRMerge_BelowTheOpenWindowALiveLoopRefusesEveryPRAsToday",
		"TestPRMerge_WithALiveLoopAndTheWindowOnADocsPRMergesAndAGoPRIsRefused",
		"TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex",
		"TestPRWindow_AnEmptyChangeIsABoundary",
		"TestPRWindow_AtStageOnACodePathIsABoundaryAndExitsThree",
		"TestPRWindow_AtStageOnADocsAndCIPRIsAWindowAndExitsZero",
		"TestPRWindow_IOFailuresExitTwo",
		"TestPRWindow_UntilIntegrationEnforceEveryPRIsABoundaryAndTheRuleStillShows",
		"TestRealTreeRoutingOrder_AnchoredPhasesFollowTheirAnchors",
		"TestRealTreeRoutingOrder_EveryPreBuildPhaseSitsBeforeBuild",
		"TestSelfhealPropose_AFailedRunIsRecorded",
		"TestSelfhealPropose_AFailedRunWaitsForItsBackoff",
		"TestSelfhealPropose_ATornShipJournalIsAWarningAndRefusesOnlyTheRevert",
		"TestSelfhealPropose_AnEvidenceFaultIsAFailedProposalWithNoLaunch",
		"TestSelfhealPropose_IOFaultsAreExitTwo",
		"TestSelfhealPropose_NoPendingIncidentReadsNoSignal",
		"TestSelfhealPropose_StageOffLaunchesNoAgent",
		"TestSelfhealPropose_TheBudgetRefusesTheFourthT1RepairOfAWindow",
		"TestSelfhealPropose_TheDefaultPassDiagnosesTheTwoMostSevereIncidents",
		"TestSelfhealPropose_TheLiveIncidentsEachGetOneCheckedProposalThroughTheBridge",
		"TestSelfhealPropose_TheValidatorDecidesEachProposalAndRecordsWouldApplyDecisions",
		"TestSelfhealPropose_TheVerbGivesTheValidatorTheRealInventory",
		"TestSelfhealPropose_ValidatorIOFaultsAreExitTwo",
		"TestSelfhealPropose_ValidatorStageOffDecidesNothing",
		"TestSkills_CheckDetectsDrift",
		"TestSkills_NoDrift",
		"TestSpikeAdvisorLive",
		"TestTheCheckedInTableCompiles",
		"TestTheCheckedInTableRoutesNoLaunchToCodex",
		"TestUniversalFallbackTail_LeavesTheExclusionToTheRouter",
		"TestWireOrchestrator_MemoRunnerRegistered",
	}}},
	{Path: "agents/", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestWireOrchestratorDeps_ContractVerifierReachesEveryPhaseRunner",
		"TestWireOrchestrator_HostEffectsWired",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestNewHostEffects_AFailedRouteLeavesAProtectedFilesItemAtTheInboxRoot",
		"TestNewHostEffects_AProtectedFilesItemIsRoutedAndAClaimableItemIsClaimed",
		"TestRoutingPlanInput_LoadsTheAdvisorCatalogFromTheRegistry",
		"TestRunCompose_AnUnresolvablePhaseIsReportedPerPhase",
		"TestSolutionCheck",
		"TestWireOrchestratorDeps_ContractVerifierReachesEveryPhaseRunner",
		"TestWireOrchestrator_APostBuildPhaseAnchoredAfterBuildRaisesNoRoutingDefect",
		"TestWireOrchestrator_APreBuildPhaseAnchoredAfterBuildReachesTheSignalCenter",
		"TestWireOrchestrator_HostEffectsWired",
	}}},
	{Path: "docs/architecture/signal-codes.md", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry",
	}}},
	{Path: "docs/operations/runtime-reference.md", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestHandoffVerbs_AreInTheRegistryTheUsageTextAndTheRuntimeReference",
	}}},
	{Path: "skills/", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestLoopSkill_NoBudgetReferences",
	}}},
	{Path: "skills/loop/SKILL.md", TestSelection: TestSelection{Package: "./cmd/evolve", Tests: []string{
		"TestLoopSkill_NoBudgetReferences",
	}}},
	{Path: ".evolve/", TestSelection: TestSelection{Package: "./internal/adapters/ledger", Tests: []string{
		"TestIter_RealLedger_NoStringCycleError",
	}}},
	{Path: "agents/evolve-auditor.md", TestSelection: TestSelection{Package: "./internal/auditchain", Tests: []string{
		"TestPersona_InstructsExactlyTheShapeTheParserReads",
	}}},
	{Path: ".evolve/policy.json", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestEveryShippedProfileAndTierLaunchesWithAnEffort",
	}}},
	{Path: ".evolve/profiles/", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestAgentSandboxContract",
		"TestAgentSandboxContract_AForgedLaneScopeGainsNoGrantAtTheNextDispatch",
		"TestAgentSandboxContract_AnOpenOfAnIndexedPathWritesOneAccessRecord",
		"TestAgentSandboxContract_N10_AnAgyPaneCannotWriteAProtectedPath",
		"TestAgentSandboxContract_ShadowReportsTheExecAndLetsItRun",
		"TestAgentSandboxContract_TheHostHoldsTheCredentialThatTheCellCannotRead",
		"TestAgentSandboxContract_TheLauncherStillStartsTheCLIThatTheCellCannotExec",
		"TestAgentSandboxContract_TheSelfhealOrchestratorSeatHoldsN3AndN4",
		"TestDefaultSandboxWrap_RendersEveryDeclaredWriteSubpath",
		"TestEveryAgentProfileRunsInTheSandbox",
		"TestEveryClaudeLaunchIsStrictMCPAndNamesNoMCPConfig",
		"TestEveryProfileWritesOnlyItsOwnCycleRunDir",
		"TestEveryShippedProfileAndTierLaunchesWithAnEffort",
		"TestNativeDecisionProfileOwnedCWD",
		"TestNativeRetrospectiveLessonBoundary",
		"TestNativeRoleEvalAuthoringBoundary",
		"TestNoProfileCanWriteTheDossierPendingDir",
		"TestNoProfileCanWriteTheResumeWitness",
		"TestRealProfiles_NoToolIsBothAllowedAndDenied",
		"TestRepoProfiles_DeclareTheSettingsScopeInsteadOfCopyingItsFlags",
	}}},
	{Path: ".evolve/profiles/debugger.json", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestDebuggerProfile_KeepsTheWriteAndEditToolsItsPersonaNeeds",
	}}},
	{Path: ".evolve/profiles/deliverable-recovery.json", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestDeliverableRecoveryLaunch_DeclaresNoNetworkAndTheWrapperStillForcesIt",
		"TestDeliverableRecoveryLaunch_GrantsNothingOfTheRepositoryButTheRunDir",
		"TestDeliverableRecoveryProfile_WritesOnlyTheRunDir",
	}}},
	{Path: ".evolve/profiles/triage.json", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestDebuggerProfile_KeepsTheWriteAndEditToolsItsPersonaNeeds",
		"TestDefaultSandboxWrap_GrantsTriagesDeclaredInboxClaimDir",
		"TestDefaultSandboxWrap_RendersEveryDeclaredWriteSubpath",
		"TestDeliverableRecoveryLaunch_DeclaresNoNetworkAndTheWrapperStillForcesIt",
		"TestDeliverableRecoveryLaunch_GrantsNothingOfTheRepositoryButTheRunDir",
		"TestDeliverableRecoveryProfile_WritesOnlyTheRunDir",
		"TestEveryAgentProfileRunsInTheSandbox",
		"TestEveryProfileWritesOnlyItsOwnCycleRunDir",
		"TestNativeDecisionProfileOwnedCWD",
		"TestNativeRetrospectiveLessonBoundary",
		"TestNativeRoleEvalAuthoringBoundary",
		"TestNoProfileCanWriteTheDossierPendingDir",
		"TestNoProfileCanWriteTheResumeWitness",
		"TestRealProfiles_NoToolIsBothAllowedAndDenied",
		"TestTriageProfile_DeniesTheInboxAuditTrail",
		"TestTriageProfile_GrantsTheClaimMove",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestAutoRespond_CodexUpdateMenuRulesDoNotMatchThisRepositorysOwnFiles",
		"TestRealDriversDeclareInputLineMarker",
	}}},
	{Path: "docs/", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestAutoRespond_TrustRulesDoNotMatchThisRepositorysOwnFiles",
	}}},
	{Path: "docs/incidents/2026-08-27-plan-mode-dialog-blind-spot.md", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestAutoRespond_PlanModeDoesNotMatchThisRepositorysOwnFiles",
	}}},
	{Path: "go/internal/bridge/stopreview.go", TestSelection: TestSelection{Package: "./internal/bridge", Tests: []string{
		"TestDriverLivenessRouting_StopReviewHasNoCLILiterals",
	}}},
	{Path: "adapters/", TestSelection: TestSelection{Package: "./internal/capability", Tests: []string{
		"TestQualityTier_GoldenParityWithBashManifests",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/changedpkgs", Tests: []string{
		"TestClosure_TestableIsASubsetOfPatterns",
		"TestImporterClosureChecked_NotDerivableOutsideAModule",
		"TestImporterClosureChecked_TestableExcludesTagOnlyDirs",
		"TestImporterClosure_BestEffortOnBadInput",
		"TestImporterClosure_ExcludesNonImporters",
		"TestImporterClosure_RouterRoutingtest",
		"TestImporterClosure_SortedDedupedAndModuleRoot",
		"TestImporterClosure_TestOnlyImporter",
		"TestImporterClosure_Transitive",
	}}},
	{Path: ".github/workflows/", TestSelection: TestSelection{Package: "./internal/ciparity", Tests: []string{
		"TestCIConcurrency_DispatchGroupsByRunID",
		"TestCIConcurrency_DistinctPullRequestsRefsAndRunsNeverShareAGroup",
		"TestCIConcurrency_PullRequestGroupsByNumberAndCancels",
		"TestCIConcurrency_PushGroupsByRefAndNeverCancels",
		"TestCIConcurrency_RequiredAndGoWorkflowsAreEvaluatedForEveryEvent",
		"TestCIConcurrency_ScheduleGroupsByRunIDAndNeverCancels",
		"TestGateSteps_EachRowNamesARealJobAndStepOfGoYml",
		"TestGoWorkflow_EveryJobHasABoundedTimeout",
		"TestGoWorkflow_GatesTheSuiteOnAPinnedGovulncheckFromTheMakeTarget",
		"TestGoWorkflow_MacOSRunsOnlyTheDarwinSetWithItsCoverageGatesAndTheE2ETier",
		"TestGoWorkflow_ReportsPackageWallBudgetWithoutGatingTheJob",
		"TestGoWorkflow_RunsCoverChangedRightAfterTheIntegrationProfile",
		"TestGoWorkflow_Ubuntu123OwnsEveryWholeModuleGateAndGo127RunsTheSuiteOnly",
		"TestGoWorkflow_UsesSharedIntegrationAndAPIGates",
		"TestGoWorkflow_VetsAndFormatsOncePerPlatformNotOncePerMatrixLeg",
		"TestLandingValidation_PagesUsesCompleteValidatedArtifact",
		"TestLandingWorkflow_TestsPullRequestsBeforeBuild",
		"TestMacosWeekly_IsASoftGateOutsideTheRequiredResult",
		"TestRelease_RequiresSharedValidationBeforePublishing",
		"TestRequiredResult_ConsumesEachReusableJobResult",
		"TestRequiredResult_RejectsMissingOrUnsuccessfulWork",
		"TestRequiredResult_WaitsForEveryOtherJob",
		"TestRequiredRouting_AnEmptyPullRequestDiffKeepsTheFullSelection",
		"TestRequiredRouting_ConsumedPathsAndExplicitDocumentationSkips",
		"TestRequiredRouting_DocsOnlyPullRequestSelectsOnlyTheLint",
		"TestRequiredRouting_GitFailuresStopWithoutShellErrexit",
		"TestRequiredRouting_InboxOnlyChangesSkipTheGoMatrix",
		"TestRequiredRouting_MergeBaseExcludesBaseBranchOnlyChanges",
		"TestRequiredRouting_MissingEvidenceCannotBecomeDocumentationSkip",
		"TestRequiredRouting_NonFastForwardPushDiffsTheReachableOldTip",
		"TestRequiredRouting_ScheduleRoutesLikeAManualRun",
		"TestRequiredRouting_ScheduleSelectsEverySuiteWhateverTheDiff",
		"TestRequiredRouting_UsesHostEventAndPublishesBothDecisions",
		"TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies",
		"TestRequiredWorkflow_ConcurrencyCancelsOnlySupersededPullRequestRuns",
		"TestRequiredWorkflow_InboxJobLintsTheInboxAndRunsItsRealTreeTests",
		"TestRequiredWorkflow_LintJobRunsThePinnedGolangciLintOnTheGoRoute",
		"TestRequiredWorkflow_ReusableSuitesAreNotDuplicated",
		"TestRequiredWorkflow_RunsTheFullGraphOnceADayOnTheDefaultBranch",
		"TestRequiredWorkflow_ScheduleLeavesTheResultAndRoutingGraphUnchanged",
		"TestRequiredWorkflow_SteLintJobRunsOnlyTheLintOnTheAddedText",
		"TestReusableGoValidation_PreservesReleaseHistory",
	}}},
	{Path: ".github/workflows/go.yml", TestSelection: TestSelection{Package: "./internal/ciparity", Tests: []string{
		"TestDarwinCIStep_OutlastsTheTestTimeout",
		"TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier",
	}}},
	{Path: ".github/workflows/macos-weekly.yml", TestSelection: TestSelection{Package: "./internal/ciparity", Tests: []string{
		"TestMacosWeekly_IsASoftGateOutsideTheRequiredResult",
		"TestMacosWeekly_OnlyTheReportJobCanWriteIssues",
		"TestMacosWeekly_RunsTheFullSuiteThroughTheMakeTargets",
		"TestMacosWeekly_RunsWeeklyOnMainAndIsNotAPullRequestCheck",
	}}},
	{Path: "go/", TestSelection: TestSelection{Package: "./internal/ciparity", Tests: []string{
		"TestDarwinSetFile_MatchesTheSetDerivedFromTheTree",
	}}},
	{Path: "go/.apicover-enforce", TestSelection: TestSelection{Package: "./internal/ciparity", Tests: []string{
		"TestGateSteps_ApicoverBoundHoldsTheEnforcedPackageList",
	}}},
	{Path: "go/Makefile", TestSelection: TestSelection{Package: "./internal/ciparity", Tests: []string{
		"TestEnvExclusive_TheNotmuxMakeTargetRunsTheBridgeTiersUnderTheFloorTags",
		"TestGoWorkflow_GatesTheSuiteOnAPinnedGovulncheckFromTheMakeTarget",
		"TestMakeAPICheck_RejectsMissingOrMalformedProfile",
		"TestMakeAPIEnforce_AcceptsTestOnlyOrEmptyEnrollment",
		"TestMakeAPIEnforce_ExcludesCyclePredicatesFromMeasurement",
		"TestMakeAPIEnforce_LeavesTheIntegrationProfileForCoverChanged",
		"TestMakeAll_ExecutesCompoundE2ETag",
		"TestMakeApicover_NeverFeedsTheCoverageGates",
		"TestMakeCoverStrict_ChecksFinalLineWithoutNewline",
		"TestMakeCoverStrict_PreservesFloor",
		"TestMakeCoverStrict_RefusesAMissingOrStaleProfile",
		"TestMakeCoverStrict_RejectsInvalidFloorOrEvaluator",
		"TestMakeCoverStrict_RejectsManifestDirectory",
		"TestMakeCoverageGates_PropagateTestFailure",
		"TestMakeCoverageGates_RejectMissingManifest",
		"TestMakeCoverageGates_RejectToolFailure",
		"TestMakeDarwinTargets_TestAndGateOnlyThePackagesOfTheDarwinSet",
		"TestMakeE2E_FailsWhenNoTestCarriesTheTag",
		"TestMakeE2E_RunsOnlyTheTestsOfE2ETaggedFiles",
		"TestMakeFixture_CancellationFailsHarness",
		"TestMakeIntegrationDarwin_GivesTheSlowerHostItsOwnTimeout",
		"TestMakeIntegration_LeavesAProfileOnlyAfterAGreenRun",
		"TestMakeIntegration_PassesTheTestTimeout",
		"TestMakeIntegration_RoutesTestBinariesThroughTheRusageWrapperOnlyWhenAsked",
		"TestMakeIntegration_SelectsEntireRuntime",
		"TestMakeIntegration_TeesAJSONStreamWithoutLosingTheVerdict",
		"TestMakeRaceRecipes_DropTheExitSleepAndKeepOtherOptions",
		"TestMakeTestIntegration_ABadSplitListStopsTheRunBeforeAnyTest",
		"TestMakeTestIntegration_AFailedSplitRunFailsTheRunAndLeavesNoProfile",
		"TestMakeTestIntegration_ASplitPackageCountsForThePackageItTests",
		"TestMakeTestRecipes_RunGitWithBackgroundMaintenanceOff",
		"TestMakefile_DarwinTargetsReadTheOneDarwinSetFile",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/cli/agentcmd", Tests: []string{
		"TestAgentList_JSON",
		"TestAgentList_NamesEachConfinedProfileInNameOrder",
		"TestAgentList_NamesReadOnlySpecProfilesOnly",
		"TestAgentList_NamesTheBuildSeatAsAWriter",
		"TestAgentList_ProjectRootDefaultsToTheEnvironment",
		"TestAgentRun_ACloudSeatReportPutsTheSessionInTheRecordAndTheEnvelope",
		"TestAgentRun_ADispatchErrorEmitsNoMismatchSignal",
		"TestAgentRun_ADispatchErrorRecordsNoVerdictAndAWriterIsMarked",
		"TestAgentRun_AFailWithNoReportKeepsNoEmptyDeliverable",
		"TestAgentRun_AFailedCloudSeatRecordsNoSessionFromAStaleBlock",
		"TestAgentRun_AFencedVerdictOnlyReportEmitsOneParseDefect",
		"TestAgentRun_ALabelCollisionIsADispatchErrorAndKeepsTheOtherRun",
		"TestAgentRun_ALabelWhoseLaunchRecordExistsIsNotReusedAndDispatchesNothing",
		"TestAgentRun_ALaunchRecordWriteFailureIsADispatchError",
		"TestAgentRun_ALaunchedCloudSeatPassesWithNoReportDerivation",
		"TestAgentRun_AMatchingPassReportKeepsPassWithNoSignal",
		"TestAgentRun_AProfileNameThatMakesNoRunIDIsADispatchError",
		"TestAgentRun_AProfileWithoutAConfinedSandboxIsRefused",
		"TestAgentRun_AReaderRefusesAWritableDir",
		"TestAgentRun_AReportSeatKeepsTheReportVerdictOfAReportWithASessionBlock",
		"TestAgentRun_AReportThatListsOneHighIsAFailVerdictThoughTheFieldSaysPass",
		"TestAgentRun_AReportThatStatesFailIsAFailVerdict",
		"TestAgentRun_AReportThatStatesWarnIsAWarnVerdictWithNoSignal",
		"TestAgentRun_AReportWithNoSessionRecordsNoCloudSession",
		"TestAgentRun_AReportWithoutTheRequiredHeadingRecordsTheMissingSectionAndKeepsTheFile",
		"TestAgentRun_AReviewSeatRoutedToTheCloudSeatNeverLaunchesTheCloudSeat",
		"TestAgentRun_ASeatTokenFileThatCannotBeMadeAbsoluteIsAUsageError",
		"TestAgentRun_ASessionBlockFromAnotherSeatIsNotACloudLaunch",
		"TestAgentRun_ATokenGivenAsTheSeatTokenFileValueIsRefusedAndNeverPrinted",
		"TestAgentRun_ATreeStateErrorIsNamedOnStderrAndLeavesTheTreeEmpty",
		"TestAgentRun_ATreeThatChangedDuringTheRunRecordsBothStates",
		"TestAgentRun_AVerdictSectionWithNoStatedVerdictIsAFailVerdict",
		"TestAgentRun_AVerifiedReportKeepsNoDeliverableAndRecordsNoViolation",
		"TestAgentRun_AWorkspaceThatCannotBeRemovedIsNamedOnStderr",
		"TestAgentRun_AWriterWhoseSandboxCanWriteTheReviewRecordsIsRefused",
		"TestAgentRun_AdversarialReviewDiffOnStdinPrintsTheReportAndLeavesNoCycleFootprint",
		"TestAgentRun_AnAttachedFileWithAFenceCannotCloseItsSection",
		"TestAgentRun_AnInputOverTheLimitIsRefused",
		"TestAgentRun_AnIsolationSignalOfTheBridgeReachesTheIsolationChannel",
		"TestAgentRun_AnUndeclaredWriterOnAWriteGrantingProfileIsRefused",
		"TestAgentRun_DispatchErrorExitsTwentyOneAndNamesTheError",
		"TestAgentRun_EachRunWritesABridgeLaunchRecordThatNamesTheDriverFamilyModelAndTree",
		"TestAgentRun_FailVerdictExitsOneAndStillPrintsTheReport",
		"TestAgentRun_FailingDeliverablesAreBoundedAndRecordsAreNeverPruned",
		"TestAgentRun_InputErrors",
		"TestAgentRun_InputSources",
		"TestAgentRun_JSONEnvelopeCarriesReportVerdictCostAndTokens",
		"TestAgentRun_KeepLeavesTheWorkspaceWithInputAndReport",
		"TestAgentRun_LedgerFailureIsADispatchError",
		"TestAgentRun_LedgerFailureIsNamedInTheJSONEnvelope",
		"TestAgentRun_NoReportIsAFailVerdictWithAnEmptyStdout",
		"TestAgentRun_NoStatedVerdictWithNoBlockingFindingEmitsAParseDefect",
		"TestAgentRun_Refusals",
		"TestAgentRun_RefusedRunWritesNothing",
		"TestAgentRun_SeatTokenFileReachesTheBridgeRequestAsAnAbsolutePath",
		"TestAgentRun_StdinReadErrorIsAnInputError",
		"TestAgentRun_TerminalStdinWithAnAttachedFileRuns",
		"TestAgentRun_TheBuildSeatAcceptsAWritableDirAsItsOnlyGrant",
		"TestAgentRun_TheBuildSeatAcceptsAWritableThatIsAFileOrNotYetThere",
		"TestAgentRun_TheBuildSeatKeepsANewFileInAWritableDirAndRestoresAnExistingFileOutsideIt",
		"TestAgentRun_TheBuildSeatRefusals",
		"TestAgentRun_TheBuildSeatRefusesAWritableDirThatIsNotACleanLocalDirectory",
		"TestAgentRun_TheBuildSeatRefusesAWritableThatNamesADirectory",
		"TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantEndsWithAVerdictThatIsNotPASS",
		"TestAgentRun_TheBuildSeatThatCreatesAFileOutsideItsGrantGetsAnErrorThatNamesThePathAndTheGrant",
		"TestAgentRun_TheBuildSeatWritesOnlyInAGivenLinkedWorktreeWithAFence",
		"TestAgentRun_TheEnvelopeNamesTheSeatLoginAndNeverTheTokenFileContent",
		"TestAgentRun_TheInputCapCoversAllSourcesTogether",
		"TestAgentRun_TheJourneyOfASeatThatCreatesAFileOutsideItsGrantExitsNonZeroAndRecordsTheVerdict",
		"TestAgentRun_WithoutTheFlagTheRequestHasNoSeatTokenFile",
		"TestAgentRun_WorkspaceCreateFailureIsADispatchError",
		"TestDefaultBridge_IsTheCenterLessRegistryBridge",
		"TestEveryAgentRunProfileGrantsWriteOnItsDeliverable",
		"TestNew_ZeroDepsUseTheProcessEnvironment",
	}}},
	{Path: ".claude/settings.json", TestSelection: TestSelection{Package: "./internal/cli/guardcmd", Tests: []string{
		"TestHookWiring_EveryWiredGuardIsAGuardTheBinaryBuilds",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/cli/phasecmd", Tests: []string{
		"TestNewSpecRunner_RunsTheSpecPhaseOnTheGivenBridge",
		"TestSpecRunnable",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/cli/phasecmd", Tests: []string{
		"TestCatalogPhaseNamesExcludesNativeAndControlSpecs",
		"TestCollectProvenance_ALedgerThatCannotBeScannedFailsTheCheckF2",
		"TestCollectProvenance_APhaseRootWithABrokenSpecWarnsAndStillChecksTheRestF2",
		"TestCollectProvenance_AnArtifactThatCannotBeReadIsAnErrorNamingItF2",
		"TestCollectProvenance_SkipsAbsentAndNonMarkdownArtifactsAndCountsTheCheckedOnesF2",
		"TestFormatUnknownPhaseError_ListsSpecsOnlyWhenTheCatalogHasThem",
		"TestIsKnownPhase",
		"TestPhaseVerify_DeclaredEffectFollowsCycleState",
		"TestPhasesCheckProvenance_ExitCodes",
		"TestPhasesCheckProvenance_MissingHeaderWarnsAndPasses",
		"TestPhasesCheckProvenance_RejectsStrayPositional",
		"TestPhasesCheckProvenance_WriteFailureIsUnreadable",
		"TestResolvePhaseRunner_ExitCodes",
		"TestResolveRunnerSkipsUnrunnableSpec",
		"TestResolveRunner_RunnableSpecBuildsASpecRunner",
		"TestRunPhase_SpecKnownAtTheCwdRootButUnloadableAtTheRequestRootExits1",
	}}},
	{Path: ".evolve/phases/", TestSelection: TestSelection{Package: "./internal/codereview", Tests: []string{
		"TestPhaseName_IsThePhaseJSONsNameArtifactSectionsAndGrammar",
	}}},
	{Path: "agents/evolve-code-reviewer.md", TestSelection: TestSelection{Package: "./internal/codereview", Tests: []string{
		"TestThePersonasFindingExampleParsesWithEveryField",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/config", Tests: []string{
		"TestDefaults_TddRuleMatchesRegistry",
		"TestDocumentSpec_AndKindVocabulary",
		"TestLoad_DeliverableKinds_FromRegistry",
		"TestLoad_RealRegistry",
		"TestLoad_RegistryTddRule_ReleasesDocumentCycles",
	}}},
	{Path: ".evolve/phases/", TestSelection: TestSelection{Package: "./internal/core", Tests: []string{
		"TestSeedPhase_BugReproductionReachesAdvisorCatalog",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/core", Tests: []string{
		"TestDefaultBuildFloorChecks_AtEnforceABrokenStandardSendsTheDocBack",
		"TestDefaultBuildFloorChecks_AtEnforceACleanDocPasses",
		"TestDefaultBuildFloorChecks_AtEnforceAFailingDocGoesBackToTheBuild",
		"TestDefaultBuildFloorChecks_AtEnforceAnUnreadableDocWarnsBesideTheFailure",
		"TestDefaultBuildFloorChecks_AtEnforceOnlyTheAddedLinesOfADocCount",
		"TestDefaultBuildFloorChecks_WarnsOnSteFindingsInTheChangedDocs",
		"TestDispositionExample_PersonaAndGoAgreeAsJSON",
		"TestDocsFloorChecks_APolicyThatDoesNotLoadWarnsAndFailsClosed",
		"TestDocsFloorWarn_ABrokenStandardWarnsThatTheLintDidNotRun",
		"TestDocsFloorWarn_AChangedGeneratedDocIsNotAFinding",
		"TestDocsFloorWarn_CleanOrOutOfScopeDocsPrintNothing",
		"TestDocsFloorWarn_NamesADocThatItCannotRead",
		"TestDocsFloorWarn_NamesTheFirstFindingOfTheChangedDocs",
		"TestDocsFloorWarn_SteStageOffPrintsNothing",
		"TestDocsFloorWarn_TheShadowStageOnlyWarns",
		"TestExplanationGap_OneRejectionAlsoListsTheSteAndCoverageGaps",
		"TestJudgmentTeachingPhases_CoverEveryPhaseThatCanStateItsOwnVerdict",
		"TestRepoPhaseCatalog_CatalogWordIsKnown",
		"TestRepoPhaseCatalog_MenuPhasesResolveAPersona",
		"TestSteFindingLines_AScopeThatCannotBeReadJudgesTheWholeFile",
	}}},
	{Path: "agents/evolve-router.md", TestSelection: TestSelection{Package: "./internal/core", Tests: []string{
		"TestComposePlanPrompt_RealPersonaExistingExampleCarriesATierAndNoCLI",
		"TestRealPersonaFrontmatterOutputFormatEnumeratesTheTierOnly",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/core", Tests: []string{
		"TestRegistryActivatingFields_AgreeWithLiteralKernel",
		"TestSeedPhase_BugReproductionReachesAdvisorCatalog",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/deliverable", Tests: []string{
		"TestNonSpinePhaseDeclaredSignalsReachTheRoutingDigest",
		"TestPerform_ReportsAHandoffSignalsWriteFailure",
		"TestWriteHandoffSignals_WritesNothingWhenThereIsNothingToWrite",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/deliverable", Tests: []string{
		"TestHostEffects_TheRegistryDeclaresTriagesDerivation",
		"TestPhaseRegistry_EveryDeclaredEffectHasACheck",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/derived", Tests: []string{
		"TestCatalog_EveryInputExists",
		"TestCatalog_EveryOutputCarriesItsGeneratedMarker",
		"TestResolveGoInputs_ReadsTheRepository",
	}}},
	{Path: "schemas/cycle-dossier.schema.json", TestSelection: TestSelection{Package: "./internal/dossier", Tests: []string{
		"TestSchemaVersion_SchemaDeclaresTheField",
		"TestSchema_NoDrift",
		"TestSchema_NoDrift_CatchesTheMutationsThatMatter",
	}}},
	{Path: "agents/", TestSelection: TestSelection{Package: "./internal/evalgate", Tests: []string{
		"TestSlugParserContract",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/explanationdocs", Tests: []string{
		"TestExplanationPersonaAndSchemaContract_NoDrift",
	}}},
	{Path: "agents/evolve-builder-reference.md", TestSelection: TestSelection{Package: "./internal/explanationdocs", Tests: []string{
		"TestBuilderLiteralExamplesMatchProductionReaders",
	}}},
	{Path: "docs/architecture/packages/internal-gc.md", TestSelection: TestSelection{Package: "./internal/gc", Tests: []string{
		"TestInternalGCNote_AmendedByLinksThePlan",
	}}},
	{Path: ".evolve/inbox/consumed/", TestSelection: TestSelection{Package: "./internal/inboxmover", Tests: []string{
		"TestConsumedRecord_RoundTripAgainstCorpus",
	}}},
	{Path: ".github/workflows/required.yml", TestSelection: TestSelection{Package: "./internal/mergewindow", Tests: []string{
		"TestClassify_ADocsAndInboxPRIsBoundaryBecauseCIRunsTheGoSuiteForIt",
		"TestClassify_AnEmptyChangeIsBoundaryAsCIRoutesItToEverySuite",
		"TestClassify_OneBoundaryPathMakesThePRBoundaryAndIsNamed",
		"TestClassify_TheLiveCIRoutingTableDecidesEachPath",
		"TestParse_FailsClosedOnATableItCannotRead",
		"TestParse_ReadsASmallTableAndMatchesStarAcrossSlashes",
		"TestParse_TheLiveRoutingAcceptsAScheduledRunAsAFullRun",
	}}},
	{Path: ".evolve/profiles/", TestSelection: TestSelection{Package: "./internal/phasecontract", Tests: []string{
		"TestArtifactNameMatchesProfileOutput",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/phasecontract", Tests: []string{
		"TestSourceWriterPhases_AreTheRealTreeCatalogsSourceWriters",
	}}},
	{Path: "agents/", TestSelection: TestSelection{Package: "./internal/phasecontract", Tests: []string{
		"TestExplanationDocumentation_TitleAndProducerDeclaration",
		"TestProducersDeclareCanonical",
		"TestSelfReview_TheHookNeverEditsEarlierPhaseOwnedFiles",
		"TestSelfReview_TheWriterPersonasAndTheSkillDeclareTheHeadingTheRunnerChecks",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/phasecontract", Tests: []string{
		"TestCatalogResolver_OverlaysDeclaredFieldsOntoBuiltins",
		"TestRealRegistry_BuiltinArtifactNameMatchesDeclaredPrimary",
		"TestRealRegistry_EveryBuiltinWithArtifactResolvesARegistryEntry",
		"TestRealRegistry_SecondariesArePartitioned",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/phaseregistrar", Tests: []string{
		"TestRegister_UnknownPhaseNameStaysCatalogGreen",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/phases/audit", Tests: []string{
		"TestAuditAndShipSkills_PointAtTheRenderedWarnRuleInsteadOfRestatingIt",
		"TestAuditorPromptAndArchDocDispositionExamplesAgree",
		"TestAuditorPromptDispositionExampleIsAcceptedByProductionReader",
		"TestComposePrompt_TheRulesLiveOnlyInTheRenderedBlockAndNoStalePhrasingReappears",
		"TestDispositionSchemaExampleMatchesDocumentedExample",
		"TestNewDefault_WiresSkillsDriftCheck",
		"TestSkillsDriftGate_BaseGeneratorChangeWithoutRegenerationFails",
		"TestSkillsDriftGate_CommittedGeneratorChangeIsGradedByTheWorktreeWithOrWithoutABase",
		"TestSkillsDriftGate_Cycle1840ReplayPasses",
		"TestSkillsDriftGate_Cycle1841ReplayBaseNewerThanTheHostPasses",
		"TestSkillsDriftGate_GeneratorChangeThatAgreesWithTheHostIsSilent",
		"TestSkillsDriftGate_GeneratorChangeWithRegeneratedStubsPasses",
		"TestSkillsDriftGate_GeneratorChangeWithoutRegenerationFails",
		"TestSkillsDriftGate_ImportedPackageChangeIsGradedByTheWorktreeGenerator",
		"TestSkillsDriftGate_ReworkedReportProtocolStillFails",
		"TestSkillsDriftGate_TreeWithoutTheGeneratorSourceIsGradedInProcess",
	}}},
	{Path: "agents/evolve-auditor-reference.md", TestSelection: TestSelection{Package: "./internal/phases/audit", Tests: []string{
		"TestAuditorExplanationLiteralExampleMatchesProductionReader",
	}}},
	{Path: "agents/evolve-builder.md", TestSelection: TestSelection{Package: "./internal/phases/build", Tests: []string{
		"TestComposePrompt_ProvidesCanonicalExplanationBinding",
	}}},
	{Path: "agents/evolve-retrospective.md", TestSelection: TestSelection{Package: "./internal/phases/retro", Tests: []string{
		"TestLessonPathIsSingleSourcedWithThePersona",
		"TestRetroExplanationLiteralExampleMatchesProductionReader",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/phases/triage", Tests: []string{
		"TestHostChain_ARoutedPinnedItemPassesTheGateAndNothingIsLeftToBuild",
		"TestHostChain_AnUnroutedPinnedItemTheHostClaimsPassesTheGate",
		"TestRouteJourney_AClaimThatFailsOnceFailsTheCycleWithTheHostTextAndBuildsNothing",
		"TestRouteJourney_APinOfARoutedAndAnUnroutedItemBuildsOnlyTheUnrouted",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestSmoke_RealAgentFiles",
	}}},
	{Path: "agents/", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestAllPerCycleAgentsStrictlyCompact",
		"TestAlwaysOnDocFilesExist",
		"TestAlwaysOnPhaseDocsHaveCompactMarker",
		"TestRealDocOnDemandStrip",
	}}},
	{Path: "agents/evolve-intent.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestIntentAgentFileExists",
		"TestIntentCompaction_AskWhenNeededAboveMarker",
		"TestIntentCompaction_HeadFloor7000_Amplified",
		"TestIntentCompaction_OperationalAnchorsAboveMarker",
		"TestIntentCompaction_RealDocIdempotent",
		"TestIntentCompaction_RealDocIdempotent_PostCycle422",
		"TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative",
		"TestIntentCompaction_ReferenceSectionAbsentAfterStrip",
		"TestIntentCompaction_ReflectionAuthoringNotDeleted",
		"TestIntentCompaction_StripsTheCompositionAndReferenceTail",
		"TestIntentCompaction_TighterBytesSavingThreshold",
		"TestIntentHasCompactMarkerViaGate",
		"TestIntentOnDemandSectionsNotDeleted_Amplified",
		"TestIntentStrippedBodyFloor",
	}}},
	{Path: "agents/evolve-orchestrator.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestOrchestratorCompaction_ByteSavingsFloorAmplified",
		"TestOrchestratorCompaction_Idempotent",
		"TestOrchestratorOnDemandSectionsNotDeleted",
	}}},
	{Path: "agents/evolve-reflector-reference.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestReflectorReferenceStubExists",
		"TestReflectorReferenceStubHasNarrativeHeading",
	}}},
	{Path: "agents/evolve-reflector.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestReflectorCompaction_MarkerAfterOperationalSections",
		"TestReflectorCompaction_MarkerPresent",
		"TestReflectorCompaction_NarrativeAbsentAfterStrip_Negative",
		"TestReflectorCompaction_OperationalAnchorsAboveMarker",
		"TestReflectorCompaction_RealDocIdempotent",
		"TestReflectorCompaction_StripSavesBytes",
		"TestReflectorHasCompactMarkerViaGate",
		"TestReflectorStrippedBodyFloor",
	}}},
	{Path: "agents/evolve-retrospective.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestRetroCompaction_HeadFloor_Amplified",
		"TestRetroCompaction_Idempotent",
		"TestRetroDocHasLineAnchoredMarker",
		"TestRetroVersionedSectionsNotDeleted",
	}}},
	{Path: "agents/evolve-router.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestRouterCompaction_66RowsRetained",
		"TestRouterCompaction_CoreValuesSectionUnder8000Bytes",
		"TestRouterCompaction_NoDuplicatePhaseNames",
		"TestRouterCompaction_NoEmptyTriggerRows_Negative",
		"TestRouterCompaction_SectionByteFloor",
		"TestRouterCompaction_TriggerMinimumLength",
		"TestRouterPersona_CatalogByteIdentical_Negative",
		"TestRouterPersona_DomainVocabPreserved",
		"TestRouterPersona_ExtendedVocabRunField",
		"TestRouterPersona_GeneratedBlockMarkersPreserved",
		"TestRouterPersona_LoaderAndRenderParseGreen",
		"TestRouterPersona_ProseFloor",
		"TestRouterPersona_ProseRegionByteReduction",
		"TestRouterPersona_SectionHeadingsPreserved",
		"TestRouterPersona_TSCMarkerPresent",
	}}},
	{Path: "agents/evolve-tdd-engineer-reference.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestTddEngineerReferenceStubExists",
	}}},
	{Path: "agents/evolve-tdd-engineer.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestTddEngineerAdditionalAnchors",
		"TestTddEngineerCompaction",
		"TestTddEngineerStrippedBodyFloor",
	}}},
	{Path: "agents/evolve-triage.md", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestTriageCompaction",
		"TestTriageCompaction_HeadFloor6000_Amplified",
		"TestTriageCompaction_Idempotent_Amplified",
		"TestTriageCompaction_MarkerSectionExists_Amplified",
		"TestTriageOnDemandSectionNotDeleted_Amplified",
		"TestTriageStrippedBodyFloor",
	}}},
	{Path: "go/", TestSelection: TestSelection{Package: "./internal/prompts", Tests: []string{
		"TestRouterPersona_LoaderAndRenderParseGreen",
	}}},
	{Path: "docs/architecture/review-loop-and-quality-index.md", TestSelection: TestSelection{Package: "./internal/qualityindex", Tests: []string{
		"TestTheDesignDocProjectsTheIndexItsCodeDefines",
	}}},
	{Path: "skills/quality-index/", TestSelection: TestSelection{Package: "./internal/qualityindex", Tests: []string{
		"TestTheSkillProjectsTheIndexItsCodeDefines",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/recoveryretro", Tests: []string{
		"TestRenderJudgmentLesson_NamesTheCycleThePhaseAndQuotesTheLesson",
		"TestRenderRecovery_PassesTheStrictSTELint",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/regressiontia", Tests: []string{
		"TestChangedScope_DegenerateInputs",
		"TestChangedScope_SelectsTestOnlyImporters",
		"TestChangedScope_WidensByReverseDependency",
	}}},
	{Path: "go/", TestSelection: TestSelection{Package: "./internal/regressiontia", Tests: []string{
		"TestCompute_NeverSkipsAnEscapeHatchPredicate",
		"TestListCorpusDeps_ResolvesTheRealCorpus",
		"TestResolveDeps_DropsPredicatesThatReachOutsideTheImportGraph",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/releasepipeline", Tests: []string{
		"TestCurrentBranch_ValidGitRepo",
		"TestResolveInitCommit_ValidGitRepo",
	}}},
	{Path: "go/", TestSelection: TestSelection{Package: "./internal/releasepipeline", Tests: []string{
		"TestDefaultRebuildBinary_NonDryRun_RealRepo",
	}}},
	{Path: ".evolve/phases/forces-analysis/phase.json", TestSelection: TestSelection{Package: "./internal/router", Tests: []string{
		"TestDomainPhaseTrigger_FiresOnScoutGoalType",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/router", Tests: []string{
		"TestBugfixCategoryPhases_DeclareTheyDoNotAdmitADocument",
		"TestClampPlanToFloor_RecordsWhichEntriesTheFloorForced",
		"TestCodeLane_AFloorForcedTDDStillRuns",
		"TestCodeReviewPin_IsTheRegistryRuleNotAGoList",
		"TestCodeReviewPin_RunsNextOnEveryCodeCycleThatTouchedFilesEvenWhenTheAdvisorOmitsIt",
		"TestCodeReviewPin_SkipsADocumentCycleAndAZeroFileBuild",
		"TestDocumentLane_AFloorForcedTDDLapsesOnceScoutDeclaresADocument",
		"TestDocumentLane_Cycle1692TddDecisionIsNotRecordedForcedOn",
		"TestDocumentLane_NeverDispatchesTheBugfixPhasesOnCycle1692sPlan",
		"TestDocumentLane_TheGateIsThePhaseJSONDeclarationNotAGoList",
		"TestGoalRecipePhases_GoalTypeTriggersAdmitTheRecipeGoalType",
		"TestGoalRecipeStrategyOptions_DocumentLaneRunsItsRecipePhases",
		"TestRoute_LaneItemsAreAdvisorContextTheWalkIgnores",
		"TestShouldRunFromPlan_AFloorForcedTDDRunsWhileThePinHoldsEvenOnADocumentLane",
	}}},
	{Path: "agents/", TestSelection: TestSelection{Package: "./internal/router", Tests: []string{
		"TestPersonaTemplates_CarryTheHeaderLines",
	}}},
	{Path: "agents/evolve-router.md", TestSelection: TestSelection{Package: "./internal/router", Tests: []string{
		"TestRouterPersonaRecipeTable_NoDrift",
	}}},
	{Path: "docs/architecture/phase-registry.json", TestSelection: TestSelection{Package: "./internal/router", Tests: []string{
		"TestRouterPersonaRecipeTable_NoDrift",
	}}},
	{Path: "agents/evolve-selfheal-orchestrator.md", TestSelection: TestSelection{Package: "./internal/selfheal", Tests: []string{
		"TestCatalog_ThePersonaStatesTheRefusalsOfEachAction",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/skillcheck", Tests: []string{
		"TestCheck_AnUnlistableSkillsDirectoryIsAnError",
		"TestCheck_CleanRepoNoDrift",
		"TestCheck_CorruptSkillMDError",
		"TestCheck_DetectsMutatedRegion",
		"TestCheck_SurfacesManifestProblems",
		"TestInstructionBudget_TheCheckedInCLIFilesStayWithinTheirByteBudget",
		"TestInstructionDiffs_TheCheckedInGeminiFileMatchesItsRender",
		"TestManifestProblems_CleanRepoNoProblems",
		"TestRunAndCheck_FailOnAnInstructionRenderError",
		"TestRunAndCheck_ReportAnInstructionFileDrift",
		"TestRunAndCheck_ReportAnOverBudgetCLIFile",
		"TestRun_ASkillsDirectoryThatCannotBeListedFailsTheRunWithExitOne",
		"TestRun_AnUnusableClaudeManifestFailsTheRunWithExitOne",
		"TestRun_CatalogWarningsCarryWARNPrefix",
		"TestRun_CheckDetectsMissingCommand",
		"TestRun_CheckMode_Drift",
		"TestRun_CheckMode_Drift_OutputIsolation",
		"TestRun_CheckMode_FrontmatterNameDrift",
		"TestRun_CheckMode_NoDrift",
		"TestRun_CheckMode_PhaseNotInRegistry_DoesNotPanic",
		"TestRun_CheckOKLine",
		"TestRun_GenerateWritesCommandStubs",
		"TestRun_LegacyPrefixedStubReaped",
		"TestRun_NameMismatchFailsCheckButNotWrite",
		"TestRun_OrphanCommand",
		"TestRun_OrphanCommandCheckAndReapLines",
		"TestRun_StaleFactsDriftLineNamesRemedy",
		"TestRun_SurfacesManifestProblems",
		"TestRun_WriteFail",
		"TestRun_WriteMode_Idempotent",
		"TestRun_WriteMode_NoDrift",
		"TestRun_WriteMode_RewritesDrift",
	}}},
	{Path: "skills/", TestSelection: TestSelection{Package: "./internal/skilloverlay", Tests: []string{
		"TestMaterialize_TheCodeReviewSimplifyOverlayIsItsCompactProjection",
		"TestMaterialize_TheDocsWriteOverlayIsItsCompactProjectionWithoutFrontmatter",
	}}},
	{Path: "skills/docs-write/SKILL.md", TestSelection: TestSelection{Package: "./internal/skilloverlay", Tests: []string{
		"TestMaterialize_TheDocsWriteOverlayIsItsCompactProjectionWithoutFrontmatter",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/stelint", Tests: []string{
		"TestParseWordTable_ReadsTheRealStandard",
		"TestTheLimitsMatchTheStandard",
	}}},
	{Path: ".gitattributes", TestSelection: TestSelection{Package: "./test/structure", Tests: []string{
		"TestParallelLanesAddingDistinctPackagesComposeInBothMergeOrders",
		"TestParallelLanesAddingOneIdenticalLineKeepOneLineInBothMergeOrders",
		"TestParallelLanesAddingOnePackageFailTheGuardInBothMergeOrders",
		"TestUnionMergesTriesBothOrders",
		"TestUnionRegistriesMatchGitAttributes",
	}}},
	{Path: WholeRepository, TestSelection: TestSelection{Package: "./test/structure", Tests: []string{
		"TestNoTrackedFileRegistersAnMCPServer",
	}}},
	{Path: ".evolve/profiles/", TestSelection: TestSelection{Package: "./test/trustkernel", Tests: []string{
		"TestProfile_AllPhaseProfilesValid",
	}}},
}
