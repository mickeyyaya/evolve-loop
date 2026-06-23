package core

import "github.com/mickeyyaya/evolveloop/go/internal/shiperr"

// The structured ship→orchestrator error protocol now lives in the
// zero-dependency leaf internal/shiperr (so both the ship phase that CONSTRUCTS
// ShipErrors and core's orchestrator that MATCHES them import it one-directionally
// — no cycle either way). This file is a back-compat shim: type aliases + const
// re-declarations + thin func wrappers keep the ~141 existing core.ShipError /
// core.Code* / core.NewShipError / core.AsShipError call sites unchanged. New
// code may depend on internal/shiperr directly; the shim is removable in a later
// cleanup pass once call sites migrate.

type (
	ShipError      = shiperr.ShipError
	ShipErrorClass = shiperr.ShipErrorClass
	ShipStage      = shiperr.ShipStage
	ShipErrorCode  = shiperr.ShipErrorCode
)

// Severity vocabulary.
const (
	ShipClassTransient    = shiperr.ShipClassTransient
	ShipClassPrecondition = shiperr.ShipClassPrecondition
	ShipClassIntegrity    = shiperr.ShipClassIntegrity
	ShipClassConfig       = shiperr.ShipClassConfig
)

// Ship stages.
const (
	StageVerifySelfSHA = shiperr.StageVerifySelfSHA
	StageVerifyClass   = shiperr.StageVerifyClass
	StageAtomicShip    = shiperr.StageAtomicShip
	StagePostShip      = shiperr.StagePostShip
	StageArgs          = shiperr.StageArgs
)

// Precise failure identities (grouped by stage).
const (
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
	CodeWorktreeResolve        = shiperr.CodeWorktreeResolve
	CodeIntegrityTreeDrift     = shiperr.CodeIntegrityTreeDrift

	CodeArgs    = shiperr.CodeArgs
	CodeGitIO   = shiperr.CodeGitIO
	CodeStateIO = shiperr.CodeStateIO
	CodeUnknown = shiperr.CodeUnknown
)

// NewShipError re-exports shiperr.NewShipError (thin wrapper, immutable func).
func NewShipError(code ShipErrorCode, class ShipErrorClass, stage ShipStage, message string, debugKV ...string) *ShipError {
	return shiperr.NewShipError(code, class, stage, message, debugKV...)
}

// AsShipError re-exports shiperr.AsShipError.
func AsShipError(err error) (*ShipError, bool) { return shiperr.AsShipError(err) }
