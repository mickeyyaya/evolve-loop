package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
)

func TestEveryTopLevelTestIsParallelOrListedSerial(t *testing.T) {
	t.Parallel()
	sources, err := paralleltest.Sources(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range paralleltest.Violations(sources, paralleltest.Config{Serial: serialRequiredTests}) {
		t.Error(v)
	}
}

const (
	changesDirectory                = "test changes the process directory"
	setsEnv                         = "test sets the process environment through a helper"
	swapsAbsPath                    = "test swaps package variable absPath"
	swapsWorkingDir                 = "test swaps package variable workingDir"
	swapsReauditRebase              = "test swaps package variable reauditRebase"
	swapsBackoffSleep               = "test swaps package variable backoffSleep"
	swapsBuildSelfCheckRunner       = "test swaps package variable buildSelfCheckRunner"
	swapsChallengeTokenRead         = "test swaps package variable challengeTokenRead"
	swapsCoverChangedProfiler       = "test swaps package variable coverChangedProfiler"
	swapsCoverLockWait              = "test swaps package variable coverLockWait"
	swapsCoverStrictRunner          = "test swaps package variable coverStrictRunner"
	swapsCoveringTestsDeriver       = "test swaps package variable coveringTestsDeriver"
	swapsFailureDecide              = "test swaps package variable failureDecide"
	swapsFailureDigestMarshal       = "test swaps package variable failureDigestMarshal"
	swapsMarshalIndentJSON          = "test swaps package variable marshalIndentJSON"
	swapsRandRead                   = "test swaps package variable randRead"
	swapsResumeBoundaryCheckpointer = "test swaps package variable ResumeBoundaryCheckpointer"
	swapsGitRunner                  = "test swaps package variable gitRunner"
	swapsIdentityPatchID            = "test swaps package variable identityPatchID"
	swapsNextCorrection             = "test swaps package variable nextCorrection"
	swapsOsArgs                     = "test swaps os.Args"
	swapsPhaseBoundaryCheckpointer  = "test swaps package variable PhaseBoundaryCheckpointer"
	swapsPhasePlanMarshal           = "test swaps package variable phasePlanMarshal"
	swapsPostReviewExecutable       = "test swaps package variable postReviewExecutable"
	swapsPostBuildRepinProvenanceFn = "test swaps package variable postBuildRepinProvenanceFn"
	swapsPostScoutReplanProbe       = "test swaps package variable postScoutReplanProbe"
	swapsPredicateModuleDir         = "test swaps package variable predicateModuleDir"
	swapsQuotaBoundaryCheckpointer  = "test swaps package variable QuotaBoundaryCheckpointer"
	swapsStderr                     = "test swaps os.Stderr"
	swapsWorktreeAddRetrySleep      = "test swaps package variable worktreeAddRetrySleep"
)

var serialRequiredTests = map[string]string{
	"core.TestReauditCycle_MovedBaseRebasesAuditsAndRecordsTheBinding":                              swapsReauditRebase,
	"core.TestReauditCycle_RequestIsDerivedFromTheCycleState":                                       swapsReauditRebase,
	"core.TestReauditCycle_TheAuditRunsBehindTheReadOnlyFence":                                      swapsReauditRebase,
	"core.TestReauditCycle_VerdictIsDataNotAnError":                                                 swapsReauditRebase,
	"core.TestReauditCycle_LiveLeaseRefusesBeforeAnyGitWrite":                                       swapsReauditRebase,
	"core.TestReauditCycle_StaleLeaseDoesNotBlock":                                                  swapsReauditRebase,
	"core.TestReauditCycle_RebaseConflictLeavesTheWorktreeUnchanged":                                swapsReauditRebase,
	"core.TestReauditCycle_UnknownCycleIsNoSuchCycle":                                               swapsReauditRebase,
	"core.TestReauditCycle_AuditRunnerFailureIsInfraAndRecordsNoBinding":                            swapsReauditRebase,
	"core.TestReauditError_ErrorNamesTheKindAndConflicts":                                           swapsReauditRebase,
	"core.TestReauditCycle_UnresolvableMainIsInfraAndAuditsNothing":                                 swapsReauditRebase,
	"core.TestReauditCycle_RebasesThroughTheSharedRebaseStep":                                       swapsReauditRebase,
	"core.TestCheckpointInterruptedPhase_CheckpointFailureWarns":                                    swapsResumeBoundaryCheckpointer,
	"core.TestOriginDefaultBranch_GitFailureFallsBackToMain":                                        swapsGitRunner,
	"core.TestGitWorktree_CreateReusedWorktreeFailures":                                             swapsGitRunner,
	"core.TestPersonaBudgetFailures_LongOutputIsTruncated":                                          swapsBuildSelfCheckRunner,
	"core.TestMarshalFailures_AreReportedNotFatal":                                                  swapsMarshalIndentJSON,
	"core.TestMintRunID_EntropyFailureZeroesTheRandomPart":                                          swapsRandRead,
	"core.TestWorktreeBaseSHA_SkipsMalformedLogLines":                                               swapsGitRunner,
	"core.TestEnsureCleanWorktree_QuarantineFailures":                                               swapsGitRunner,
	"core.TestOriginDefaultBranch_EmptyRemoteHeadFallsBackToMain":                                   swapsGitRunner,
	"core.TestDefaultCurrentHead_GitTransportError":                                                 swapsGitRunner,
	"core.TestDiscardMainLeak_GitFailures":                                                          swapsGitRunner,
	"core.TestGitStatusNames_Edges":                                                                 swapsGitRunner,
	"core.TestCarryforward_GitFailureModes":                                                         swapsGitRunner,
	"core.TestAdvanceContinuationBase_UnresolvableMain":                                             swapsGitRunner,
	"core.TestSnapshotPreservedWorktree_GitStepFailures":                                            swapsGitRunner,
	"core.TestValidateContinuation_Rejections":                                                      swapsGitRunner,
	"core.TestDetectLostLanding_NoWorkspaceDoesNotReadTheProcessCWD":                                changesDirectory,
	"core.TestEnsureFailureDigest_MissingWorkspaceNeverWritesToWorkingDirectory":                    changesDirectory,
	"core.TestGitAndFileHelperCoverage":                                                             changesDirectory,
	"core.TestGitWorktree_RelativeBaseRefused":                                                      changesDirectory,
	"core.TestGitWorktree_RelativeProjectRootRefused":                                               changesDirectory,
	"core.TestWorktreeDiffSince_AGitFaultIsAnEvidenceFaultNotAnEmptyDiff":                           setsEnv,
	"core.TestNewCycleRun_RejectsRootCycleOccupiedInFetchedWorktree":                                setsEnv,
	"core.TestNoteContractGateDemotion_EachFailingSinkWarnsAndTheOthersStillRecord":                 setsEnv,
	"core.TestWriteBuildSelfCheck_HealthyWriteIsSilent":                                             swapsStderr,
	"core.TestWriteBuildSelfCheck_WriteFailureSurfaces":                                             swapsStderr,
	"core.TestWriteBuildSelfCheckArtifact_HealthyRoundTrip_LargeAndUnicodeContent":                  swapsStderr,
	"core.TestWriteBuildSelfCheckArtifact_MkdirAllFailure_WARNsAndDoesNotPanic":                     swapsStderr,
	"core.TestStampContinuation_NeverSnapshotsTheProjectRoot":                                       swapsStderr,
	"core.TestBridgeRetryAdjudicator_PromptCarriesTheAuditsReasonsFromTheOneReader":                 swapsStderr,
	"core.TestApplyPostReviewGuards_AGitignoredBinaryIsNeverCheckedOut":                             swapsPostReviewExecutable,
	"core.TestApplyPostReviewGuards_AGuardThatCannotReSnapshotAfterTheDiscardAbortsUnchecked":       swapsPostReviewExecutable,
	"core.TestApplyPostReviewGuards_ALeakedPathUnderAConsoleLeaseContinuesButSaysItIsNotClean":      swapsPostReviewExecutable,
	"core.TestApplyPostReviewGuards_ARebuiltBinaryInsideTheMainTreeIsDiscardedAndThePhaseContinues": swapsPostReviewExecutable,
	"core.TestApplyPostReviewGuards_AnExecutableOutsideTheMainTreeOrUnknownLeavesNoRelativeBinary":  swapsPostReviewExecutable,
	"core.TestApplyPostReviewGuards_TheKnownGoEvolvePathIsNotDiscardedTwice":                        swapsPostReviewExecutable,
	"core.TestLaneStartRef_RetriesRefLockContention":                                                swapsGitRunner,
	"core.TestBaseBehindMain_WithNoWorktreeNeverRunsGit":                                            swapsGitRunner,
	"core.TestCoverfixS2_UnresolvablePathsSurfaceAsRefusalNotSuccess":                               swapsAbsPath + "; " + swapsWorkingDir,
	"core.TestExecuteRetryBackoff_AppliedOnAttempt2":                                                swapsBackoffSleep,
	"core.TestExecuteRetryBackoff_ZeroBaseDisables":                                                 swapsBackoffSleep,
	"core.TestShipRecovery_JitteredBackoffBetweenReaudits":                                          swapsBackoffSleep,
	"core.TestBuildSelfCheck_ClearsStaleArtifactOnPass":                                             swapsBuildSelfCheckRunner,
	"core.TestBuildSelfCheck_NoGoChangesIsNoOp":                                                     swapsBuildSelfCheckRunner,
	"core.TestBuildSelfCheck_WritesArtifactOnFailure":                                               swapsBuildSelfCheckRunner,
	"core.TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags":                    swapsBuildSelfCheckRunner,
	"core.TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck":                                   swapsBuildSelfCheckRunner,
	"core.TestPersonaBudgetFailures":                                                                swapsBuildSelfCheckRunner,
	"core.TestRecordAndBranch_BuildGraduationGuardAborts":                                           swapsBuildSelfCheckRunner,
	"core.TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds":                                 swapsBuildSelfCheckRunner,
	"core.TestPlanCycle_ACallerTokenIsNeverReminted":                                                swapsChallengeTokenRead,
	"core.TestPlanCycle_AFailedTokenMintLeavesNoTokenAnywhere":                                      swapsChallengeTokenRead,
	"core.TestCoverChangedFailures_AFailingTestRefusesAtEnforceAndOnlyWarnsAtShadow":                swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_EnforceRefusesTheChangedFunctionOfTheBuild":                      swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_ShadowOffAndEveryPlumbingFaultLeaveTheHandoffAlone":              swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_ExcludesACSTagOnlyPackageFromProfiler":                           swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_LeavesAFailureTheUnitRunAlsoRefusesToTheChangedPackageFloor":     swapsBuildSelfCheckRunner,
	"core.TestCoverChangedFailures_StrictRunner":                                                    swapsCoverStrictRunner,
	"core.TestCoverChangedFailures_StrictRunnerDefaultMeasuresOnlyTheChangedStrictPackages":         swapsCoverChangedProfiler,
	"core.TestCoverChangedFailures_UncoveredFunction":                                               swapsStderr,
	"core.TestCoverStrictFailures_ARunnerErrorWarnsAndLeavesTheHandoffAlone":                        swapsCoverStrictRunner,
	"core.TestScopedCoverFunc_BehindAHeldLockWaitsOnlyItsBoundThenRunsUnserialized":                 swapsCoverLockWait,
	"core.TestCoveringTests_AvailableToTestAmplificationWithoutFreshBuild":                          swapsCoveringTestsDeriver,
	"core.TestDecideAfterRetro_ABlockingDecisionEndsTheCycleNamingTheAction":                        swapsFailureDecide,
	"core.TestRecordAndBranch_ARetroBranchTheStateMachineForbidsAborts":                             swapsFailureDecide,
	"core.TestRecordAndBranch_ARetroDecisionCarriesItsEnvironmentIntoTheNextPhases":                 swapsFailureDecide,
	"core.TestResumeExecution_ARetroBranchTheStateMachineForbidsAbortsTheResume":                    swapsFailureDecide,
	"core.TestResumeExecution_ARetroDecisionCarriesItsEnvironmentIntoTheRescheduledPhase":           swapsFailureDecide,
	"core.TestAssembleFailureDigest_EachPersistenceFailureReturnsTheBuiltDigest":                    swapsFailureDigestMarshal,
	"core.TestCarryforwardCandidateLandable_CleanMergeIsLandable":                                   swapsGitRunner,
	"core.TestCarryforwardCandidateLandable_ConflictIsFalseNotError":                                swapsGitRunner,
	"core.TestCarryforwardCandidateLandable_PatchIdDuplicateSuperseded":                             swapsGitRunner,
	"core.TestCarryforwardCandidateLandable_SupersededShortCircuitsBeforeMerge":                     swapsGitRunner,
	"core.TestClassifyFleetRebaseCandidate_GitInfraErrorIsError":                                    swapsGitRunner,
	"core.TestClassifyFleetRebaseCandidate_ThreeWayVerdicts":                                        swapsGitRunner,
	"core.TestCleanup_DeletesMergedCycleBranch":                                                     swapsGitRunner,
	"core.TestCleanup_UnmergedBranchSurvives_WarnsOnly":                                             swapsGitRunner,
	"core.TestDefaultGitHEAD_GitFailureDegrades":                                                    swapsGitRunner,
	"core.TestDefaultGitHEAD_UsesInjectedSeam":                                                      swapsGitRunner,
	"core.TestGitCapture_NonzeroExitReportedViaCode":                                                swapsGitRunner,
	"core.TestGitCapture_RoutesThroughInjectedSeam":                                                 swapsGitRunner,
	"core.TestCoreCoverageBatch_WorktreeBaseTreeSHA":                                                swapsGitRunner,
	"core.TestCoreCoverageBatch_RecordBuildBinding":                                                 swapsGitRunner,
	"core.TestCoreCoverageBatch_RecordGenericBinding":                                               swapsGitRunner,
	"core.TestCoreCoverageBatch_PackageNewThisCycle":                                                swapsGitRunner,
	"core.TestGitWorktreeCleanup_NoWorktreeIsANoOp":                                                 swapsGitRunner,
	"core.TestGitWorktreeCreateFrom_CleanRunCostsOneAttemptAndNoSleep":                              swapsGitRunner,
	"core.TestGitWorktreeCreateFrom_PersistentFailureStillFailsLoudly":                              swapsGitRunner,
	"core.TestGitWorktreeCreateFrom_RetriesTransientAddFailure":                                     swapsGitRunner,
	"core.TestGitWorktreeCreate_PersistentFailureStillFailsLoudly":                                  swapsGitRunner,
	"core.TestGitWorktreeCreate_RetriesTransientAddFailure":                                         swapsGitRunner,
	"core.TestGitWorktree_Create_FetchFailureIsFatal":                                               swapsGitRunner,
	"core.TestGitWorktree_Create_FetchesOriginBeforeBasingLane":                                     swapsGitRunner,
	"core.TestGitWorktree_Create_NoOriginFallsBackToLocalHEAD":                                      swapsGitRunner,
	"core.TestGitWorktree_Create_ReuseSkipsFetch":                                                   swapsGitRunner,
	"core.TestLaneStartRef_FetchesOnlyTheDefaultBranch":                                             swapsGitRunner,
	"core.TestLaneStartRef_FollowsTheRemotesDefaultBranch":                                          swapsGitRunner,
	"core.TestLaneStartRef_PersistentContentionFailsAfterTheBound":                                  swapsGitRunner,
	"core.TestPruneSupersededOrphans_CheckedOutAndBoundReportsOneReason":                            swapsGitRunner,
	"core.TestPruneSupersededOrphans_CheckedOutMatchIsExactBranchName":                              swapsGitRunner,
	"core.TestPruneSupersededOrphans_CheckedOutRefSkippedNotAborted":                                swapsGitRunner,
	"core.TestPruneSupersededOrphans_ContinuationBoundIsKept":                                       swapsGitRunner,
	"core.TestPruneSupersededOrphans_CorruptRegistryAbortsBeforeAnyDelete":                          swapsGitRunner,
	"core.TestPruneSupersededOrphans_DeleteFailureReportedPerRefNotAborted":                         swapsGitRunner,
	"core.TestPruneSupersededOrphans_DistinctBranchUntouched":                                       swapsGitRunner,
	"core.TestPruneSupersededOrphans_HasOpenPRErrorAborts":                                          swapsGitRunner,
	"core.TestPruneSupersededOrphans_HasOpenPRErrorStillAbortsAfterCheckedOutSkip":                  swapsGitRunner,
	"core.TestPruneSupersededOrphans_MultipleConsecutiveCheckedOutRefsAllSkipped":                   swapsGitRunner,
	"core.TestPruneSupersededOrphans_OpenPRFlaggedButKept":                                          swapsGitRunner,
	"core.TestPruneSupersededOrphans_SupersededNoPRIsPruned":                                        swapsGitRunner,
	"core.TestPruneSupersededOrphans_WorktreeListFailureAbortsBeforeAnyDelete":                      swapsGitRunner,
	"core.TestRecordAuditBinding_AFailedHeadReadStopsBeforeAnyOtherGitOrBinding":                    swapsGitRunner,
	"core.TestRecoverBuildLeak_UsesInjectedGit":                                                     swapsGitRunner,
	"core.TestVerdictCacheCollisionRegression":                                                      swapsGitRunner,
	"core.TestWorktreeReuseBase_UnreadableAncestryRecordsHEADVerbatim":                              swapsGitRunner,
	"core.TestIdentityCarryForward_AnUnidentifiablePatchIsReauditedAfterTheGates":                   swapsIdentityPatchID,
	"core.TestReviewWithCorrections_ALiveFixRungSpendsItsBudgetWithoutRedispatching":                swapsNextCorrection,
	"core_test.TestRecoverFromShipError_TheFenceKeepsTestNamesTheFenceErrorWhenKeptIsEmpty":         swapsOsArgs,
	"core_test.TestRunCycle_InterruptCheckpointsActivePhaseWithoutRetrospective":                    swapsPhaseBoundaryCheckpointer,
	"core.TestPhaseBoundaryCheckpointer_FiresDuringRunCycle":                                        swapsPhaseBoundaryCheckpointer,
	"core.TestRecordPhasePlanKind_AMarshalFailureLedgersThePlanWithoutAnArtifact":                   swapsPhasePlanMarshal,
	"core.TestBootRecovery_AfterBuildRepin_ShipGateSeesNoMismatch":                                  swapsPostBuildRepinProvenanceFn,
	"core.TestBootRecovery_PostBuildRepin_NoBinaryIsNoOp":                                           swapsPostBuildRepinProvenanceFn,
	"core.TestBootRecovery_PostBuildRepin_UnverifiedProvenance_KeepsPin":                            swapsPostBuildRepinProvenanceFn,
	"core.TestBootRecovery_RepinsAfterBuildNotJustBoot":                                             swapsPostBuildRepinProvenanceFn,
	"core.TestCycleLoop_PostScoutHookFiresOncePreBuild":                                             swapsPostScoutReplanProbe,
	"core.TestListACSPredicates_NoResolvableModuleIsALoudNote":                                      swapsPredicateModuleDir,
	"core.TestDispatch_AllFamiliesExhausted_NoFailureLearning":                                      swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycleFromPhase_AWalledResumedDispatchNamesItsWalk":                                 swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycleFromPhase_WalledResumedCorrection_Defers":                                     swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_ALedgerFaultDoesNotLoseTheQuotaPause":                                        swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AWalledPauseHandsTheWalkedCLIsToTheCheckpoint":                               swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AllFamilies85_CheckpointsAndDefers":                                          swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro":                                   swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState":                             swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_FailedCorrectionRedispatch_IsNotADeferral":                                   swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_InterruptDuringQuotaCheckpointPreservesQuotaPause":                           swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_MixedExitCodes_NoDefer":                                                      swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_WalledCorrectionRedispatch_DefersLikeAFirstDispatch":                         swapsQuotaBoundaryCheckpointer,
	"core.TestRunCycle_WalledRemediationRerun_DefersLikeAFirstDispatch":                             swapsQuotaBoundaryCheckpointer,
	"core.TestAdvisorStderrAndStream_HappyPathAreSilent":                                            swapsStderr,
	"core.TestCoreFacades_ProjectTheLeaf":                                                           swapsStderr,
	"core.TestPhaseAdvisor_SeesTheSignalCenterAppliedThroughTheOption":                              swapsStderr,
	"core.TestLaneStartRef_RealLockOnTheBaseRefIsRetried":                                           swapsWorktreeAddRetrySleep,
}
