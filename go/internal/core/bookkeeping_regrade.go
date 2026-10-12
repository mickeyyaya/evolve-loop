package core

import (
	"regexp"
	"strings"
)

const BookkeepingRegradeReasonPrefix = "bookkeeping-regrade: "

const recoveryKeyBookkeepingRegrade = "bookkeeping-regrade"

const (
	bookkeepingDefectLedgerPrefix = "defect ledger: "
	bookkeepingClosureClaimPrefix = "closure claim without a citation: "
)

var bookkeepingConflictRE = regexp.MustCompile(`^verdict-conflict: auditor narrative=(PASS|WARN)\b`)

func BookkeepingMetaAuditReason(reason string) bool {
	return strings.HasPrefix(reason, bookkeepingDefectLedgerPrefix) ||
		strings.HasPrefix(reason, bookkeepingClosureClaimPrefix)
}

func BookkeepingConflictAuditReason(reason string) bool {
	return bookkeepingConflictRE.MatchString(reason)
}

func consumeBookkeepingRegradeGrant(cs *CycleState, reason string) {
	if strings.HasPrefix(reason, BookkeepingRegradeReasonPrefix) {
		cs.BookkeepingRegradeAttempted = true
	}
}

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
