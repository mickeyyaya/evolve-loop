// Package failurelog ports failure-classifications.sh + the bash
// record_failed_approach + cycle-state.sh:prune-expired-failures logic into
// Go.
package failurelog

import (
	"strings"
	"time"
)

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
	OperatorReset           Classification = "operator-reset"
	LoopFatal               Classification = "loop-fatal"
	UnknownClassification   Classification = "unknown-classification"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityHigh     Severity = "high"
	SeverityTerminal Severity = "terminal"
	SeverityUnknown  Severity = "unknown"
)

type RetryPolicy string

const (
	RetryYes     RetryPolicy = "yes"
	RetryNeedsOp RetryPolicy = "needs-operator"
	RetryNo      RetryPolicy = "no"
	RetryUnknown RetryPolicy = "unknown"
)

func AgeOutSeconds(c Classification) int64 {
	switch c {
	case InfrastructureTransient:
		return 86400
	case InfrastructureSystemic:
		return 604800
	case IntentMalformed:
		return 86400
	case IntentRejected:
		return 999999999
	case CodeBuildFail, CodeAuditFail:
		return 2592000
	case CodeAuditWarn:
		return 86400
	case ShipGateConfig:
		return 86400
	case HumanAbort:
		return 3600
	case ExitTransportHang:
		return 3600
	case IntegrityBreach:
		return 604800
	case OperatorReset:
		return 3600
	case LoopFatal:
		return 604800
	default:
		return 86400
	}
}

func SeverityOf(c Classification) Severity {
	switch c {
	case InfrastructureTransient, IntentMalformed, HumanAbort,
		ShipGateConfig, CodeAuditWarn, ExitTransportHang, OperatorReset:
		return SeverityLow
	case InfrastructureSystemic, CodeBuildFail, CodeAuditFail, IntegrityBreach,
		LoopFatal:
		return SeverityHigh
	case IntentRejected:
		return SeverityTerminal
	default:
		return SeverityUnknown
	}
}

func RetryPolicyOf(c Classification) RetryPolicy {
	switch c {
	case InfrastructureTransient, IntentMalformed, HumanAbort,
		ShipGateConfig, CodeAuditWarn, ExitTransportHang, OperatorReset:
		return RetryYes
	case InfrastructureSystemic, IntegrityBreach, LoopFatal:
		return RetryNeedsOp
	case IntentRejected:
		return RetryNo
	case CodeBuildFail, CodeAuditFail:
		return RetryNeedsOp
	default:
		return RetryUnknown
	}
}

func NormalizeLegacy(raw string) Classification {
	switch raw {
	case string(InfrastructureTransient), string(InfrastructureSystemic),
		string(IntentMalformed), string(IntentRejected),
		string(CodeBuildFail), string(CodeAuditFail), string(CodeAuditWarn),
		string(HumanAbort), string(IntegrityBreach), string(ShipGateConfig),
		string(ExitTransportHang), string(OperatorReset), string(LoopFatal):
		return Classification(raw)

	case "infrastructure":
		return InfrastructureTransient
	case "audit-fail":
		return CodeAuditFail
	case "build-fail":
		return CodeBuildFail
	case "ship-gate-rejection":
		return ShipGateConfig

	case "EXIT_TRANSPORT_HANG", "exit_transport_hang":
		return ExitTransportHang

	case "FAIL":
		return CodeAuditFail
	case "WARN":
		return CodeAuditWarn
	case "SHIP_GATE_DENIED":
		return ShipGateConfig
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

func ComputeExpiresAt(c Classification, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expires := now.Add(time.Duration(AgeOutSeconds(c)) * time.Second)
	return expires.UTC().Format(time.RFC3339)
}

func VocabularyList() string {
	known := KnownClassifications()
	parts := make([]string, len(known))
	for i, c := range known {
		parts[i] = string(c)
	}
	return strings.Join(parts, ", ")
}

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
		OperatorReset,
		LoopFatal,
	}
}
