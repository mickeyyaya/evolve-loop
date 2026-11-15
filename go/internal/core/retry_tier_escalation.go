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
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const retryEscalationTier = "deep"

func (o *Orchestrator) clampedRaise(projectRoot string, phase Phase, currentTier, target string) (string, bool) {
	if policy.TierRank(currentTier) >= policy.TierRank(target) {
		return "", false
	}
	tmp := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: string(phase), Run: true, Tier: target}}}
	profileFor := func(p string) *profiles.Profile { return o.profileForModelRouting(projectRoot, p) }
	clamped, _ := router.ClampPlanModelRouting(tmp, profileFor)
	tier := clamped.Entries[0].Tier
	if policy.TierRank(tier) <= policy.TierRank(currentTier) {
		return "", false
	}
	return tier, true
}

func composeBuildTier(ladderTarget, floorTier string) string {
	if policy.TierRank(floorTier) > policy.TierRank(ladderTarget) {
		return floorTier
	}
	return ladderTarget
}

func (cr *cycleRun) buildFloor(ids []string) (tier string, maxCount, threshold int) {
	threshold = cr.o.failurePolicy.Thresholds.BuildDeepEscalateAtFailures
	if cr.o.failureCountFor == nil || threshold <= 0 {
		return "", 0, threshold
	}
	for _, id := range ids {
		maxCount = max(maxCount, cr.o.failureCountFor(id))
	}
	if maxCount < threshold {
		return "", maxCount, threshold
	}
	return retryEscalationTier, maxCount, threshold
}

func (cr *cycleRun) ladderTarget(ids []string) string {
	class := worstScopedClass(cr.o.failureClassFor, ids)
	if class == "" {
		return ""
	}
	d, verdict := cr.nextItemDecision(class)
	cr.recordStartRung(d, verdict)
	if d.Action == retryladder.ActionRaise && cr.o.workflowConfig.RetryLadder.Enforced() {
		return d.Target.Tier
	}
	return ""
}

func (cr *cycleRun) escalatedBuildTier(currentTier string) (string, bool) {
	ids := cr.escalationScopeIDs()
	floorTier, maxCount, threshold := cr.buildFloor(ids)
	want := composeBuildTier(cr.ladderTarget(ids), floorTier)
	if want == "" {
		return "", false
	}
	tier, raised := cr.o.clampedRaise(cr.req.ProjectRoot, PhaseBuild, currentTier, want)
	if !raised {
		return "", false
	}
	if floorTier != "" {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d: retry tier escalation — scoped item failure_count=%d >= %d, build dispatched at %q (ADR-0076 D deterministic floor; envelope-clamped)\n", cr.cycle, maxCount, threshold, tier)
	}
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
