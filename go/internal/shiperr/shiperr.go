package shiperr

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type ShipError struct {
	Code    ShipErrorCode
	Class   ShipErrorClass
	Stage   ShipStage
	Message string
	Debug   map[string]string
}

type ShipErrorClass string

const (
	ShipClassTransient    ShipErrorClass = "transient"
	ShipClassPrecondition ShipErrorClass = "precondition"
	ShipClassIntegrity    ShipErrorClass = "integrity"
	ShipClassConfig       ShipErrorClass = "config"
)

type ShipStage string

const (
	StageVerifyExplanation ShipStage = "verify-explanation"
	StageVerifySelfSHA     ShipStage = "verify-self-sha"
	StageVerifyClass       ShipStage = "verify-class"
	StageAtomicShip        ShipStage = "atomic-ship"
	StagePostShip          ShipStage = "post-ship"
	StageArgs              ShipStage = "args"
)

type ShipErrorCode string

const (
	CodeExplanationDocumentation ShipErrorCode = "EXPLANATION_DOCUMENTATION"

	CodeSelfSHATampered ShipErrorCode = "SELF_SHA_TAMPERED"
	CodeSelfSHAIO       ShipErrorCode = "SELF_SHA_IO"

	CodeAuditBindingHeadMoved       ShipErrorCode = "AUDIT_BINDING_HEAD_MOVED"
	CodeAuditBindingTreeMismatch    ShipErrorCode = "AUDIT_BINDING_TREE_MISMATCH"
	CodeAuditBindingArtifactSHA     ShipErrorCode = "AUDIT_BINDING_ARTIFACT_SHA"
	CodeAuditBindingArtifactMissing ShipErrorCode = "AUDIT_BINDING_ARTIFACT_MISSING"
	CodeAuditBindingVerdictFail     ShipErrorCode = "AUDIT_BINDING_VERDICT_FAIL"
	CodeAuditBindingVerdictWarn     ShipErrorCode = "AUDIT_BINDING_VERDICT_WARN_STRICT"
	CodeAuditBindingMalformed       ShipErrorCode = "AUDIT_BINDING_MALFORMED_VERDICT"
	CodeAuditBindingDualVerdict     ShipErrorCode = "AUDIT_BINDING_DUAL_VERDICT"
	CodeAuditBindingStale           ShipErrorCode = "AUDIT_BINDING_STALE"
	CodeAuditBindingNoAuditor       ShipErrorCode = "AUDIT_BINDING_NO_AUDITOR"
	CodeAuditBindingAuditorExit     ShipErrorCode = "AUDIT_BINDING_AUDITOR_EXIT"
	CodeAuditBindingNoLedger        ShipErrorCode = "AUDIT_BINDING_NO_LEDGER"

	CodeEGPSRedCount ShipErrorCode = "EGPS_RED_COUNT"

	CodeControlPlaneViolation ShipErrorCode = "CONTROL_PLANE_VIOLATION"

	CodeInvalidClass         ShipErrorCode = "INVALID_CLASS"
	CodeManualNotTTY         ShipErrorCode = "MANUAL_NOT_TTY"
	CodeManualDeclined       ShipErrorCode = "MANUAL_DECLINED"
	CodeCommitGateMissing    ShipErrorCode = "COMMIT_GATE_MISSING"
	CodeCommitGateStale      ShipErrorCode = "COMMIT_GATE_STALE"
	CodeCommitGateMalformed  ShipErrorCode = "COMMIT_GATE_MALFORMED"
	CodeTrivialNotTrivial    ShipErrorCode = "TRIVIAL_NOT_TRIVIAL"
	CodeTrivialCriticalPaths ShipErrorCode = "TRIVIAL_CRITICAL_PATHS"

	CodeGitDetachedHead        ShipErrorCode = "GIT_DETACHED_HEAD"
	CodeGitStageFailed         ShipErrorCode = "GIT_STAGE_FAILED"
	CodeGitCommitFailed        ShipErrorCode = "GIT_COMMIT_FAILED"
	CodeGitFFMergeDiverged     ShipErrorCode = "GIT_FF_MERGE_DIVERGED"
	CodeGitFleetRebaseNeeded   ShipErrorCode = "GIT_FLEET_REBASE_NEEDED"
	CodeGitFleetRebaseConflict ShipErrorCode = "GIT_FLEET_REBASE_CONFLICT"
	CodeGitPushRejected        ShipErrorCode = "GIT_PUSH_REJECTED"
	CodeCommitPrefixGate       ShipErrorCode = "COMMIT_PREFIX_GATE"
	CodeManifestGate           ShipErrorCode = "MANIFEST_GATE"
	CodeRepoContractGate       ShipErrorCode = "REPO_CONTRACT_GATE"
	CodeRepoContractInfra      ShipErrorCode = "REPO_CONTRACT_INFRA"
	CodeCommittedDocDeleted    ShipErrorCode = "COMMITTED_DOC_DELETED"
	CodeWorktreeResolve        ShipErrorCode = "WORKTREE_RESOLVE"
	CodeIntegrityTreeDrift     ShipErrorCode = "INTEGRITY_TREE_DRIFT"

	CodeGitLaneNotOnOrigin       ShipErrorCode = "GIT_LANE_NOT_ON_ORIGIN"
	CodeGitLandingUnwindDeclined ShipErrorCode = "GIT_LANDING_UNWIND_DECLINED"
	CodeGitPushPolicyRefused     ShipErrorCode = "GIT_PUSH_POLICY_REFUSED"
	CodeForeignPredicatePackage  ShipErrorCode = "FOREIGN_PREDICATE_PACKAGE"

	CodeArgs    ShipErrorCode = "ARGS"
	CodeGitIO   ShipErrorCode = "GIT_IO"
	CodeStateIO ShipErrorCode = "STATE_IO"
	CodeUnknown ShipErrorCode = "UNKNOWN"
)

func NewShipError(code ShipErrorCode, class ShipErrorClass, stage ShipStage, message string, debugKV ...string) *ShipError {
	d := map[string]string{}
	for i := 0; i < len(debugKV); i += 2 {
		if i+1 < len(debugKV) {
			d[debugKV[i]] = debugKV[i+1]
		} else {
			d[debugKV[i]] = ""
		}
	}
	return &ShipError{Code: code, Class: class, Stage: stage, Message: message, Debug: d}
}

func (e *ShipError) Error() string {
	if e == nil {
		return "<nil ShipError>"
	}
	return fmt.Sprintf("[%s/%s @%s] %s", e.Code, e.Class, e.Stage, e.Message)
}

func (e *ShipError) DebugString() string {
	if e == nil || len(e.Debug) == 0 {
		return ""
	}
	keys := make([]string, 0, len(e.Debug))
	for k := range e.Debug {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s=%s", k, e.Debug[k])
	}
	return b.String()
}

func AsShipError(err error) (*ShipError, bool) {
	var se *ShipError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
