package core

// retry_tier_escalation.go — ADR-0076 slice D: recurrence-driven tier
// escalation. An item that already failed routes its next BUILD to at least
// the deep tier: deep-tier audit reliably catches what balanced-tier build
// cannot finish on hard items, so retrying at the same tier re-fails
// identically (batches 6-8). Raise-only by policy.TierRank — an advisor "top"
// proposal is never lowered — and applied BEFORE ClampPlanModelRouting so the
// profile envelope Max still clamps the result down (no new clamp code).

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// retryEscalationTier is the floor a retried item's build is raised to.
const retryEscalationTier = "deep"

// raiseBuildTierForRetry raises the plan's build entry to the escalation tier
// when failureCount has reached threshold. threshold <= 0 disables (the
// policy escape hatch). nil plan (static routing) is a safe no-op. Reports
// whether a raise was applied so the caller can log it loudly.
func raiseBuildTierForRetry(plan *router.PhasePlan, failureCount, threshold int) bool {
	if plan == nil || threshold <= 0 || failureCount < threshold {
		return false
	}
	for i := range plan.Entries {
		if plan.Entries[i].Phase != string(PhaseBuild) {
			continue
		}
		if policy.TierRank(plan.Entries[i].Tier) >= policy.TierRank(retryEscalationTier) {
			return false // already at or above the floor — never lower
		}
		plan.Entries[i].Tier = retryEscalationTier
		return true
	}
	return false
}

// escalateRetryTier applies the retry escalation for a cycle: scopeCSV is the
// lane's committed item ids (ctxSnap["fleet_scope"], comma-joined); the max
// failure_count across them drives the raise. Empty scope (a sequential cycle
// before triage commits ids) or a nil reader (composition root did not wire
// one) is a documented no-op — escalation is an optimization, never a
// correctness dependency. A raise is logged loudly for the batch record.
func escalateRetryTier(plan *router.PhasePlan, scopeCSV string, failureCountFor func(id string) int, threshold, cycle int) bool {
	if failureCountFor == nil || strings.TrimSpace(scopeCSV) == "" {
		return false
	}
	maxCount := 0
	for _, id := range strings.Split(scopeCSV, ",") {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		if n := failureCountFor(id); n > maxCount {
			maxCount = n
		}
	}
	if !raiseBuildTierForRetry(plan, maxCount, threshold) {
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d: retry tier escalation — scope item failure_count=%d >= %d, build raised to %s (ADR-0076 D; envelope Max still clamps)\n", cycle, maxCount, threshold, retryEscalationTier)
	return true
}
