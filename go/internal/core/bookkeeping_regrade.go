package core

// bookkeeping_regrade.go — the bookkeeping-regrade micro-cycle (2026-08-10
// three-perspective investigation; inbox bookkeeping-fail-regrade-microcycle).
//
// The class it repairs: an audit FAIL whose ONLY explanations are
// bookkeeping-contract gates (continuation-disposition preflight,
// closure-claim citations) while the auditor's own narrative was PASS/WARN.
// Measured cycles 1390-1429: 6 such cycles died to full continuation
// re-drives (~2M tokens each, 0/11 continuation pass rate) to author one JSON
// artifact. The regrade instead re-dispatches AUDIT once in the same cycle on
// the same snapshot — the auditor re-runs with its restored persona (#434),
// authors the bookkeeping artifact, and the deterministic gates re-evaluate.
//
// Placement in the decision stack (top outranks bottom):
//   ADR-0072 floor (verdict-incoherence / infra-systemic)  → HALT
//   bookkeeping regrade (this file)                        → retro→audit, once
//   routing strategy / failure adapter                     → tdd | ship | end
//
// Trust boundary: eligibility reads CycleState.AuditFailReasons — orchestrator
// memory, set at the recordFloorVerdictFailure chokepoint — never a workspace
// file. The once-per-cycle bound is CycleState.BookkeepingRegradeAttempted,
// same ownership. An agent can neither trigger a regrade (worst case: one
// extra audit dispatch, still gate-graded) nor unbound it.

import (
	"regexp"
	"strings"
)

// BookkeepingRegradeReasonPrefix is the RetroDecision reason-contract prefix
// for a granted regrade — the greppable operator string, and the marker
// decideAfterRetroRouted uses to keep the deterministic grant above the
// router (mirrors "retry-with-fallback:"/"proceed:" as contract strings).
const BookkeepingRegradeReasonPrefix = "bookkeeping-regrade: "

// recoveryKeyBookkeepingRegrade is the RecoveryMap key for the regrade's
// target phase (PA-DDK DDK-6): catalog config may remap it; the compiled
// fallback is PhaseAudit.
const recoveryKeyBookkeepingRegrade = "bookkeeping-regrade"

// The bookkeeping-class reason vocabulary. The classifier owns the prefixes;
// the producers (internal/phases/audit: defect_ledger.go, closure_claim.go,
// audit.go's verdict-conflict record) are pinned against these matchers by
// phases/audit/bookkeeping_reason_singlesource_test.go, which feeds REAL
// minted diagnostics through them — prefix drift on either side reds it.
const (
	bookkeepingDefectLedgerPrefix = "defect ledger: "
	bookkeepingClosureClaimPrefix = "closure claim without a citation: "
)

// bookkeepingConflictRE matches the audit verdict-conflict record ONLY when
// the auditor's narrative was PASS or WARN — the "work was graded good"
// half of the eligibility signature. Anchored to the record's start so a
// quoted occurrence inside another message cannot match.
var bookkeepingConflictRE = regexp.MustCompile(`^verdict-conflict: auditor narrative=(PASS|WARN)\b`)

// BookkeepingMetaAuditReason reports whether one audit fail reason is
// bookkeeping-class (exported for the producer-side singlesource pin).
func BookkeepingMetaAuditReason(reason string) bool {
	return strings.HasPrefix(reason, bookkeepingDefectLedgerPrefix) ||
		strings.HasPrefix(reason, bookkeepingClosureClaimPrefix)
}

// BookkeepingConflictAuditReason reports whether one audit fail reason is the
// verdict-conflict record with a non-FAIL narrative (exported for the
// producer-side singlesource pin).
func BookkeepingConflictAuditReason(reason string) bool {
	return bookkeepingConflictRE.MatchString(reason)
}

// consumeBookkeepingRegradeGrant marks the once-per-cycle slot used when the
// retro decision granted a regrade — the ONE primitive both branch surfaces
// call (recordAndBranch and the resume history branch), so the bound cannot
// drift out of one of them (the recordFloorVerdictFailure recurrence class:
// resume.go's floor-guard parity comment documents how a live-loop-only
// consumption silently reproduces the storm for resumed cycles).
func consumeBookkeepingRegradeGrant(cs *CycleState, reason string) {
	if strings.HasPrefix(reason, BookkeepingRegradeReasonPrefix) {
		cs.BookkeepingRegradeAttempted = true
	}
}

// BookkeepingRegradeEligible reports whether an audit FAIL is regrade-class:
// at least one verdict-conflict record with a PASS/WARN narrative, at least
// one bookkeeping-gate reason, and NOTHING else — any other explanation
// (EGPS red, CI-parity gate, audit CRITICAL) means real defects exist and
// the normal failure path owns the cycle.
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
