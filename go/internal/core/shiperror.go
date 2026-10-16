package core

import "github.com/mickeyyaya/evolve-loop/go/internal/shiperr"

type (
	ShipError      = shiperr.ShipError
	ShipErrorClass = shiperr.ShipErrorClass
	ShipStage      = shiperr.ShipStage
	ShipErrorCode  = shiperr.ShipErrorCode
)

const (
	ShipClassTransient    = shiperr.ShipClassTransient
	ShipClassPrecondition = shiperr.ShipClassPrecondition
	ShipClassIntegrity    = shiperr.ShipClassIntegrity
	ShipClassConfig       = shiperr.ShipClassConfig
)

const (
	StageVerifyExplanation = shiperr.StageVerifyExplanation
	StageVerifySelfSHA     = shiperr.StageVerifySelfSHA
	StageVerifyClass       = shiperr.StageVerifyClass
	StageAtomicShip        = shiperr.StageAtomicShip
	StagePostShip          = shiperr.StagePostShip
	StageArgs              = shiperr.StageArgs
)

const (
	CodeExplanationDocumentation = shiperr.CodeExplanationDocumentation

	CodeSelfSHATampered = shiperr.CodeSelfSHATampered
	CodeSelfSHAIO       = shiperr.CodeSelfSHAIO

	CodeAuditBindingHeadMoved       = shiperr.CodeAuditBindingHeadMoved
	CodeAuditBindingTreeMismatch    = shiperr.CodeAuditBindingTreeMismatch
	CodeAuditBindingArtifactSHA     = shiperr.CodeAuditBindingArtifactSHA
	CodeAuditBindingArtifactMissing = shiperr.CodeAuditBindingArtifactMissing
	CodeAuditBindingVerdictFail     = shiperr.CodeAuditBindingVerdictFail
	CodeAuditBindingVerdictWarn     = shiperr.CodeAuditBindingVerdictWarn
	CodeAuditBindingMalformed       = shiperr.CodeAuditBindingMalformed
	CodeAuditBindingDualVerdict     = shiperr.CodeAuditBindingDualVerdict
	CodeAuditBindingStale           = shiperr.CodeAuditBindingStale
	CodeAuditBindingNoAuditor       = shiperr.CodeAuditBindingNoAuditor
	CodeAuditBindingAuditorExit     = shiperr.CodeAuditBindingAuditorExit
	CodeAuditBindingNoLedger        = shiperr.CodeAuditBindingNoLedger

	CodeEGPSRedCount = shiperr.CodeEGPSRedCount

	CodeControlPlaneViolation = shiperr.CodeControlPlaneViolation

	CodeInvalidClass         = shiperr.CodeInvalidClass
	CodeManualNotTTY         = shiperr.CodeManualNotTTY
	CodeManualDeclined       = shiperr.CodeManualDeclined
	CodeCommitGateMissing    = shiperr.CodeCommitGateMissing
	CodeCommitGateStale      = shiperr.CodeCommitGateStale
	CodeCommitGateMalformed  = shiperr.CodeCommitGateMalformed
	CodeTrivialNotTrivial    = shiperr.CodeTrivialNotTrivial
	CodeTrivialCriticalPaths = shiperr.CodeTrivialCriticalPaths

	CodeGitDetachedHead        = shiperr.CodeGitDetachedHead
	CodeGitStageFailed         = shiperr.CodeGitStageFailed
	CodeGitCommitFailed        = shiperr.CodeGitCommitFailed
	CodeGitFFMergeDiverged     = shiperr.CodeGitFFMergeDiverged
	CodeGitFleetRebaseNeeded   = shiperr.CodeGitFleetRebaseNeeded
	CodeGitFleetRebaseConflict = shiperr.CodeGitFleetRebaseConflict
	CodeGitPushRejected        = shiperr.CodeGitPushRejected
	CodeCommitPrefixGate       = shiperr.CodeCommitPrefixGate
	CodeManifestGate           = shiperr.CodeManifestGate
	CodeRepoContractGate       = shiperr.CodeRepoContractGate
	CodeWorktreeResolve        = shiperr.CodeWorktreeResolve
	CodeIntegrityTreeDrift     = shiperr.CodeIntegrityTreeDrift

	CodeGitLaneNotOnOrigin       = shiperr.CodeGitLaneNotOnOrigin
	CodeGitLandingUnwindDeclined = shiperr.CodeGitLandingUnwindDeclined
	CodeGitPushPolicyRefused     = shiperr.CodeGitPushPolicyRefused

	CodeArgs    = shiperr.CodeArgs
	CodeGitIO   = shiperr.CodeGitIO
	CodeStateIO = shiperr.CodeStateIO
	CodeUnknown = shiperr.CodeUnknown
)

func NewShipError(code ShipErrorCode, class ShipErrorClass, stage ShipStage, message string, debugKV ...string) *ShipError {
	return shiperr.NewShipError(code, class, stage, message, debugKV...)
}

func AsShipError(err error) (*ShipError, bool) { return shiperr.AsShipError(err) }
