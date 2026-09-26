package core

import (
	"regexp"
	"strings"
)

// BookkeepingRegradeReasonPrefix marks a granted regrade's reason; it keeps the deterministic grant above the router.
const BookkeepingRegradeReasonPrefix = "bookkeeping-regrade: "

// recoveryKeyBookkeepingRegrade is the RecoveryMap key through which catalog config may remap the regrade's PhaseAudit target.
const recoveryKeyBookkeepingRegrade = "bookkeeping-regrade"

// The producers in internal/phases/audit are pinned to these prefixes by bookkeeping_reason_singlesource_test.go.
const (
	bookkeepingDefectLedgerPrefix = "defect ledger: "
	bookkeepingClosureClaimPrefix = "closure claim without a citation: "
)

// bookkeepingConflictRE is anchored at the record start so a quoted occurrence inside another message cannot match.
var bookkeepingConflictRE = regexp.MustCompile(`^verdict-conflict: auditor narrative=(PASS|WARN)\b`)

// BookkeepingMetaAuditReason reports whether one audit fail reason is bookkeeping-class.
func BookkeepingMetaAuditReason(reason string) bool {
	return strings.HasPrefix(reason, bookkeepingDefectLedgerPrefix) ||
		strings.HasPrefix(reason, bookkeepingClosureClaimPrefix)
}

// BookkeepingConflictAuditReason reports whether one audit fail reason is a verdict conflict with a PASS or WARN narrative.
func BookkeepingConflictAuditReason(reason string) bool {
	return bookkeepingConflictRE.MatchString(reason)
}

// consumeBookkeepingRegradeGrant is the one primitive both branch surfaces call, so the per-cycle bound cannot drift.
func consumeBookkeepingRegradeGrant(cs *CycleState, reason string) {
	if strings.HasPrefix(reason, BookkeepingRegradeReasonPrefix) {
		cs.BookkeepingRegradeAttempted = true
	}
}

// BookkeepingRegradeEligible reports whether an audit FAIL's only reasons are a PASS/WARN verdict conflict and bookkeeping gates.
func BookkeepingRegradeEligible(reasons []string) bool {
	conflict, meta := false, false
	for _, r := range reasons {
		switch {
		case BookkeepingConflictAuditReason(r):
			conflict = true
		case BookkeepingMetaAuditReason(r):
			meta = true
		default:
			return false
		}
	}
	return conflict && meta
}
