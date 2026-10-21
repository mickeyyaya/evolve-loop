package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const retryEscalationTier = "deep"

func (o *Orchestrator) clampedRaise(projectRoot string, phase Phase, currentTier, target string) (string, bool) {
	if policy.TierRank(currentTier) >= policy.TierRank(target) {
		return "", false
	}
	tmp := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: string(phase), Run: true, Tier: target}}}
	profileFor := func(p string) *profiles.Profile { return o.profileForModelRouting(projectRoot, p) }
	clamped, _ := router.ClampPlanModelRouting(tmp, profileFor, o.modelCatalogLookup)
	tier := clamped.Entries[0].Tier
	if policy.TierRank(tier) <= policy.TierRank(currentTier) {
		return "", false
	}
	return tier, true
}

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

func (cr *cycleRun) escalationScopeIDs() []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range strings.Split(cr.ctxSnap["fleet_scope"], ",") {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range processingClaimIDs(cr.req.ProjectRoot, cr.cycle, cr.o.errOut()) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func processingClaimIDs(projectRoot string, cycle int, warn io.Writer) []string {
	scan, err := inboxbatch.ScanDir(inboxbatch.ProcessingCycleDir(filepath.Join(projectRoot, ".evolve", "inbox"), cycle))
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] cycle %d: WARN claims unreadable: %v\n", cycle, err)
	}
	for _, w := range scan.Warnings {
		fmt.Fprintf(warn, "[orchestrator] cycle %d: %s\n", cycle, w)
	}
	var ids []string
	for _, it := range scan.Items {
		if it.ID != "" {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

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
