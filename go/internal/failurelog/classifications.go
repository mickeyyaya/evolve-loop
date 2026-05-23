// Package failurelog ports failure-classifications.sh + the bash
// record_failed_approach + cycle-state.sh:prune-expired-failures logic
// into Go. Three responsibilities:
//
//  1. Classification taxonomy (11 named failure classes, each with
//     a severity tier, age-out window, and retry policy).
//  2. Record: append a failed cycle's summary to
//     state.json:failedApproaches with FIFO cap 50 + atomic write +
//     advances lastCycleNumber so the next retry uses a fresh
//     workspace.
//  3. PruneExpired: walk failedApproaches at dispatcher start,
//     remove entries whose expiresAt is in the past (or whose
//     recordedAt + 1d default has passed for legacy entries with no
//     expiresAt).
//
// File split:
//
//   - classifications.go — taxonomy primitives
//   - record.go          — Record + state.json mutation
//   - prune.go           — PruneExpired + age-out filter
//
// Wire-up: cmd_loop calls Record on verify-fail with a recoverable
// classification; calls PruneExpired at dispatcher start (gated by
// EVOLVE_AUTO_PRUNE=1, default on).
package failurelog

import "time"

// Classification is the typed v8.22 taxonomy. Wire-compatible with the
// strings the bash record_failed_approach writes into
// state.json:failedApproaches[].classification.
type Classification string

const (
	InfrastructureTransient Classification = "infrastructure-transient"
	InfrastructureSystemic  Classification = "infrastructure-systemic"
	IntentMalformed         Classification = "intent-malformed"
	IntentRejected          Classification = "intent-rejected"
	CodeBuildFail           Classification = "code-build-fail"
	CodeAuditFail           Classification = "code-audit-fail"
	CodeAuditWarn           Classification = "code-audit-warn"
	ShipGateConfig          Classification = "ship-gate-config"
	HumanAbort              Classification = "human-abort"
	ExitTransportHang       Classification = "exit-transport-hang"
	IntegrityBreach         Classification = "integrity-breach"
	UnknownClassification   Classification = "unknown-classification"
)

// Severity buckets classifications by triage impact. Returned by Severity().
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityHigh     Severity = "high"
	SeverityTerminal Severity = "terminal"
	SeverityUnknown  Severity = "unknown"
)

// RetryPolicy tells the failure-adapter what to do with cycles in a
// classification bucket.
type RetryPolicy string

const (
	RetryYes          RetryPolicy = "yes"
	RetryNeedsOp      RetryPolicy = "needs-operator"
	RetryNo           RetryPolicy = "no"
	RetryUnknown      RetryPolicy = "unknown"
)

// AgeOutSeconds returns the retention window for a classification.
// Ports failure_age_out_seconds from failure-classifications.sh:66-86.
//
// After (recordedAt + AgeOutSeconds) the entry is considered expired
// and pruned at dispatcher start (or read-filtered by the failure-
// adapter, whichever fires first).
func AgeOutSeconds(c Classification) int64 {
	switch c {
	case InfrastructureTransient:
		return 86400 // 1 day
	case InfrastructureSystemic:
		return 604800 // 7 days
	case IntentMalformed:
		return 86400 // 1 day
	case IntentRejected:
		return 999999999 // effectively never
	case CodeBuildFail, CodeAuditFail:
		return 2592000 // 30 days
	case CodeAuditWarn:
		return 86400 // 1 day (v8.35 — WARN is not fail)
	case ShipGateConfig:
		return 86400 // 1 day (v8.27 — config issue, not systemic)
	case HumanAbort:
		return 3600 // 1 hour
	case ExitTransportHang:
		return 3600 // 1 hour (cycle shipped, just transport hang)
	case IntegrityBreach:
		return 604800 // 7 days
	default:
		return 86400 // unknown → 1 day default
	}
}

// SeverityOf returns the severity tier for a classification.
// Ports failure_severity_of from failure-classifications.sh:88-95.
func SeverityOf(c Classification) Severity {
	switch c {
	case InfrastructureTransient, IntentMalformed, HumanAbort,
		ShipGateConfig, CodeAuditWarn, ExitTransportHang:
		return SeverityLow
	case InfrastructureSystemic, CodeBuildFail, CodeAuditFail, IntegrityBreach:
		return SeverityHigh
	case IntentRejected:
		return SeverityTerminal
	default:
		return SeverityUnknown
	}
}

// RetryPolicyOf returns the retry policy for a classification.
// Ports failure_retry_policy from failure-classifications.sh:97-108.
func RetryPolicyOf(c Classification) RetryPolicy {
	switch c {
	case InfrastructureTransient, IntentMalformed, HumanAbort,
		ShipGateConfig, CodeAuditWarn, ExitTransportHang:
		return RetryYes
	case InfrastructureSystemic, IntegrityBreach:
		return RetryNeedsOp
	case IntentRejected:
		return RetryNo
	case CodeBuildFail, CodeAuditFail:
		// Bare classification doesn't carry the task-context needed to
		// decide retry; report conservative default.
		return RetryNeedsOp
	default:
		return RetryUnknown
	}
}

// NormalizeLegacy maps both the v8.22 taxonomy and pre-v8.22 strings
// (free-form classifications, orchestrator verdicts) to the canonical
// Classification. Ports failure_normalize_legacy from
// failure-classifications.sh:114-141.
//
// Empty or unrecognized inputs return UnknownClassification. The
// caller may want to log/skip those rather than persist them.
func NormalizeLegacy(raw string) Classification {
	switch raw {
	// Canonical taxonomy values pass through unchanged.
	case string(InfrastructureTransient), string(InfrastructureSystemic),
		string(IntentMalformed), string(IntentRejected),
		string(CodeBuildFail), string(CodeAuditFail), string(CodeAuditWarn),
		string(HumanAbort), string(IntegrityBreach), string(ShipGateConfig),
		string(ExitTransportHang):
		return Classification(raw)

	// Legacy dispatcher classifications.
	case "infrastructure":
		return InfrastructureTransient
	case "audit-fail":
		return CodeAuditFail
	case "build-fail":
		return CodeBuildFail
	case "ship-gate-rejection":
		return ShipGateConfig

	// v8.N alternate casings for exit-transport-hang.
	case "EXIT_TRANSPORT_HANG", "exit_transport_hang":
		return ExitTransportHang

	// Legacy orchestrator verdicts.
	case "FAIL":
		return CodeAuditFail
	case "WARN":
		return CodeAuditWarn // v8.35 — WARN distinct from FAIL
	case "SHIP_GATE_DENIED":
		return ShipGateConfig // v8.27
	case "WARN-NO-AUDIT":
		return InfrastructureSystemic
	case "BLOCKED-RECURRING-AUDIT-FAIL":
		return CodeAuditFail
	case "BLOCKED-RECURRING-BUILD-FAIL":
		return CodeBuildFail
	case "BLOCKED-SYSTEMIC":
		return InfrastructureSystemic
	case "SCOPE-REJECTED":
		return IntentRejected

	case "", "null":
		return UnknownClassification
	default:
		return UnknownClassification
	}
}

// ComputeExpiresAt returns the ISO-8601 timestamp (UTC, second
// precision) at which an entry of the given classification expires.
//
// Ports failure_compute_expires_at from failure-classifications.sh:174-196.
// The bash version had a v8.23.1 bug where jq fromdateiso8601 failed
// silently on unquoted ISO strings, producing epoch+1day expiry. Go's
// time.Time arithmetic is structurally immune to that class of bug.
//
// If `now` is the zero value, time.Now().UTC() is used.
func ComputeExpiresAt(c Classification, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expires := now.Add(time.Duration(AgeOutSeconds(c)) * time.Second)
	return expires.UTC().Format(time.RFC3339)
}

// KnownClassifications returns the canonical taxonomy list (excludes
// UnknownClassification). Used by tests + operator-facing diagnostics.
func KnownClassifications() []Classification {
	return []Classification{
		InfrastructureTransient,
		InfrastructureSystemic,
		IntentMalformed,
		IntentRejected,
		CodeBuildFail,
		CodeAuditFail,
		CodeAuditWarn,
		ShipGateConfig,
		HumanAbort,
		ExitTransportHang,
		IntegrityBreach,
	}
}

