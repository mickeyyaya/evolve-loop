package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ShipError is the structured ship→orchestrator error protocol. The ship
// phase is a PURE EXECUTOR: it verifies the process is right and executes
// (commit / ff-merge / push), but it has NO power to reject a cycle's changes
// — that decision was already made by audit. When ship cannot execute, it
// returns a ShipError carrying a precise Code, a severity Class, the Stage it
// failed at, and a Debug map with enough content to diagnose the problem. The
// orchestrator (not ship) decides what to do with it: route to the debugger
// phase, re-run an upstream phase, or block.
//
// It lives in core (not the ship package) so the orchestrator can errors.As
// it without an import cycle — ship imports core, so the protocol type must
// be the shared dependency. Ship constructs it; the orchestrator matches it.
//
// Why a structured envelope: the old boundary collapsed ~50 distinct ship
// failure points into one generic "ship: native exit=N" + joined logs, so the
// orchestrator could not tell a transient push race from a stale audit-binding
// from a real tampering breach. Each is now a distinct Code with its own Class
// and Debug context (cycle-148/150/151 incident family).
type ShipError struct {
	Code    ShipErrorCode     // the precise failure identity
	Class   ShipErrorClass    // severity vocabulary (drives recursion + framing)
	Stage   ShipStage         // which ship stage failed
	Message string            // human-readable summary
	Debug   map[string]string // expected/actual SHAs, paths, git stderr, exit codes, …
}

// ShipErrorClass is the severity vocabulary. It informs how the debugger frames
// the problem and the bounded-recursion policy; it does NOT let ship reject —
// every class still routes to the orchestrator's resolver.
type ShipErrorClass string

const (
	// ShipClassTransient: a retry/relaunch may succeed (network, push race).
	ShipClassTransient ShipErrorClass = "transient"
	// ShipClassPrecondition: an upstream precondition is stale/missing but
	// re-establishable (audit-binding HEAD moved, tree mismatch, stale audit).
	ShipClassPrecondition ShipErrorClass = "precondition"
	// ShipClassIntegrity: a genuine integrity breach (tamper, tree drift).
	// The debugger defaults these to BLOCK unless it can prove safety.
	ShipClassIntegrity ShipErrorClass = "integrity"
	// ShipClassConfig: operator/config error (bad class, missing attestation).
	ShipClassConfig ShipErrorClass = "config"
)

// ShipStage names the ship stage a failure occurred in (for triage + debug).
type ShipStage string

const (
	StageVerifySelfSHA ShipStage = "verify-self-sha"
	StageVerifyClass   ShipStage = "verify-class"
	StageAtomicShip    ShipStage = "atomic-ship"
	StagePostShip      ShipStage = "post-ship"
	StageArgs          ShipStage = "args"
)

// ShipErrorCode is the precise, debuggable failure identity. Grouped by stage;
// names are stable (ledger, the debugger persona, and tests key off them).
type ShipErrorCode string

const (
	// verify-self-sha
	CodeSelfSHATampered ShipErrorCode = "SELF_SHA_TAMPERED"
	CodeSelfSHAIO       ShipErrorCode = "SELF_SHA_IO"

	// verify-class — audit binding (cycle class)
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

	// verify-class — EGPS gate
	CodeEGPSRedCount ShipErrorCode = "EGPS_RED_COUNT"

	// verify-class — manual / trivial / commit-gate
	CodeInvalidClass         ShipErrorCode = "INVALID_CLASS"
	CodeManualNotTTY         ShipErrorCode = "MANUAL_NOT_TTY"
	CodeManualDeclined       ShipErrorCode = "MANUAL_DECLINED"
	CodeCommitGateMissing    ShipErrorCode = "COMMIT_GATE_MISSING"
	CodeCommitGateStale      ShipErrorCode = "COMMIT_GATE_STALE"
	CodeCommitGateMalformed  ShipErrorCode = "COMMIT_GATE_MALFORMED"
	CodeTrivialNotTrivial    ShipErrorCode = "TRIVIAL_NOT_TRIVIAL"
	CodeTrivialCriticalPaths ShipErrorCode = "TRIVIAL_CRITICAL_PATHS"

	// atomic-ship — git
	CodeGitDetachedHead    ShipErrorCode = "GIT_DETACHED_HEAD"
	CodeGitStageFailed     ShipErrorCode = "GIT_STAGE_FAILED"
	CodeGitCommitFailed    ShipErrorCode = "GIT_COMMIT_FAILED"
	CodeGitFFMergeDiverged ShipErrorCode = "GIT_FF_MERGE_DIVERGED"
	// CodeGitFleetRebaseNeeded (ADR-0049 S5b): under fleet mode a ff-merge
	// divergence is EXPECTED — a peer cycle moved main while this cycle was
	// mid-pipeline. NOT a terminal failure: the cycle must rebase onto the new
	// main and re-verify the merged tree (test-the-merged-tree / merge-queue
	// pattern) before re-shipping. Transient so the failure floor routes it to
	// recovery rather than aborting the cycle.
	CodeGitFleetRebaseNeeded ShipErrorCode = "GIT_FLEET_REBASE_NEEDED"
	CodeGitPushRejected      ShipErrorCode = "GIT_PUSH_REJECTED"
	CodeCommitPrefixGate     ShipErrorCode = "COMMIT_PREFIX_GATE"
	CodeWorktreeResolve      ShipErrorCode = "WORKTREE_RESOLVE"
	CodeIntegrityTreeDrift   ShipErrorCode = "INTEGRITY_TREE_DRIFT"

	// generic / fallthrough
	CodeArgs    ShipErrorCode = "ARGS"
	CodeGitIO   ShipErrorCode = "GIT_IO"
	CodeStateIO ShipErrorCode = "STATE_IO"
	CodeUnknown ShipErrorCode = "UNKNOWN"
)

// NewShipError builds a ShipError. debug pairs are flattened key,value,key,value.
// An odd trailing key (caller miscount) is preserved with an empty value rather
// than silently dropped — the Debug map is the diagnostic signal, so losing a
// key is worse than recording a blank one. Nil-safe.
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

// Error renders a single-line summary: "[CODE/class @stage] message".
func (e *ShipError) Error() string {
	if e == nil {
		return "<nil ShipError>"
	}
	return fmt.Sprintf("[%s/%s @%s] %s", e.Code, e.Class, e.Stage, e.Message)
}

// DebugString renders the Debug map deterministically (sorted keys) for ledger
// summaries and the debugger prompt.
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

// AsShipError recovers a *ShipError from anywhere in an error chain. Returns
// (nil, false) when none is present.
func AsShipError(err error) (*ShipError, bool) {
	var se *ShipError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
