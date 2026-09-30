package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// retryEscalationTier is the floor a retried item's build is raised to.
const retryEscalationTier = "deep"

// clampedRaise is the shared raise-only skeleton for both dispatch-time
// escalations: it clamps target through the phase profile's envelope
// (ClampPlanModelRouting) and returns it only when it still beats currentTier.
func (o *Orchestrator) clampedRaise(projectRoot string, phase Phase, currentTier, target string) (string, bool) {
	if policy.TierRank(currentTier) >= policy.TierRank(target) {
		return "", false // raise-only: never lower an equal/higher proposal
	}
	tmp := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: string(phase), Run: true, Tier: target}}}
	profileFor := func(p string) *profiles.Profile { return o.profileForModelRouting(projectRoot, p) }
	clamped, _ := router.ClampPlanModelRouting(tmp, profileFor, o.modelCatalogLookup)
	tier := clamped.Entries[0].Tier
	if policy.TierRank(tier) <= policy.TierRank(currentTier) {
		return "", false // envelope pulled the raise back — no effective gain
	}
	return tier, true
}

// escalatedBuildTier decides the dispatch-time raise for THIS cycle's build.
// currentTier is whatever the (mode-gated) plan projection already set — the
// raise never lowers it. Returns ("", false) when escalation does not apply:
// no reader wired, threshold disabled, no scoped items, counts below
// threshold, already at/above the floor, or the envelope clamp pulled the
// raise back to no gain.
func (cr *cycleRun) escalatedBuildTier(currentTier string) (string, bool) {
	threshold := cr.o.failurePolicy.Thresholds.BuildDeepEscalateAtFailures
	if cr.o.failureCountFor == nil || threshold <= 0 {
		return "", false
	}
	maxCount := 0
	for _, id := range cr.escalationScopeIDs() {
		if n := cr.o.failureCountFor(id); n > maxCount {
			maxCount = n
		}
	}
	if maxCount < threshold {
		return "", false
	}
	tier, raised := cr.o.clampedRaise(cr.req.ProjectRoot, PhaseBuild, currentTier, retryEscalationTier)
	if !raised {
		return "", false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d: retry tier escalation — scoped item failure_count=%d >= %d, build dispatched at %q (ADR-0076 D deterministic floor; envelope-clamped)\n", cr.cycle, maxCount, threshold, tier)
	return tier, true
}

// escalationScopeIDs returns the item ids driving this cycle: the lane scope
// (wave path / pinned sequential) plus any items already claimed into this
// cycle's processing dir (the sequential triage-claim path — on disk before
// build dispatches on both paths).
func (cr *cycleRun) escalationScopeIDs() []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range strings.Split(cr.ctxSnap["fleet_scope"], ",") {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range processingClaimIDs(cr.req.ProjectRoot, cr.cycle) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// processingClaimIDs reads the item ids claimed into
// <root>/.evolve/inbox/processing/cycle-<n>/ through the ONE inbox item reader
// (inboxbatch.LoadDir — a malformed claim is skipped, never breaks dispatch).
// inboxmover cannot be imported from core (it reaches core via the ledger
// adapter), so the leaf reader is the shared belief.
func processingClaimIDs(projectRoot string, cycle int) []string {
	items, _, _ := inboxbatch.LoadDir(inboxbatch.ProcessingCycleDir(filepath.Join(projectRoot, ".evolve", "inbox"), cycle))
	var ids []string
	for _, it := range items {
		if it.ID != "" {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

// auditRepairSituation is the model_tier_overrides key an in-cycle audit-repair
// round activates. builder.json and tdd-engineer.json have declared it since
// the override table landed; this is its producer.
const auditRepairSituation = "audit_retry_2plus"

func (o *Orchestrator) repairRoundTier(projectRoot string, phase Phase, cs CycleState, currentTier string) (string, bool) {
	if !cs.AuditRepairActive || !repairSeededPhase(phase) {
		return "", false
	}
	prof := o.profileForModelRouting(projectRoot, string(phase))
	if prof == nil {
		return "", false
	}
	target := strings.TrimSpace(prof.ModelTierOverrides[auditRepairSituation])
	if target == "" {
		return "", false
	}
	tier, raised := o.clampedRaise(projectRoot, phase, currentTier, target)
	if !raised {
		return "", false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d: audit-repair tier escalation — %s re-dispatched at %q (profile model_tier_overrides.%s, repair attempt %d; envelope-clamped)\n",
		cs.CycleID, phase, tier, auditRepairSituation, cs.AuditRepairAttempts)
	return tier, true
}
