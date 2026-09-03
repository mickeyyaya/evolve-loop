package core

// retry_tier_escalation.go — ADR-0076 slice D: recurrence-driven tier
// escalation as a DETERMINISTIC DISPATCH FLOOR. An item that already failed
// routes its next BUILD to at least the deep tier: deep-tier audit reliably
// catches what balanced-tier build cannot finish on hard items, so retrying
// at the same tier re-fails identically (batches 6-8).
//
// Design (adversarial review 2026-07-23, findings D1/D2): applied at BUILD
// DISPATCH, independent of the model_routing mode gate — a policy-driven
// floor must not depend on advisory routing to take effect (the live registry
// runs static). The raise is clamped through the SAME envelope guardrail the
// routing clamp uses (a single-entry ClampPlanModelRouting pass — never a
// second clamp implementation), so a profile's envelope Max still wins.
// Raise-only: a proposal already at or above deep is never touched.
//
// ADR-0096 adds a second producer of the same raise (repairRoundTier); both
// share clampedRaise so the raise-only rule and the clamp live once.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// retryEscalationTier is the floor a retried item's build is raised to.
const retryEscalationTier = "deep"

// clampedRaise is the shared raise-only skeleton of both dispatch-time
// escalations: a proposal at or above target is untouched; otherwise target
// is clamped through the ONE envelope guardrail (a single-entry
// ClampPlanModelRouting pass applying the phase profile's envelope, Max wins)
// and returned only when it still beats the proposal. Callers own the reason
// and the log line.
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
// <root>/.evolve/inbox/processing/cycle-<n>/ — tolerant of malformed files
// (a bad claim never breaks dispatch, the inbox reader convention).
func processingClaimIDs(projectRoot string, cycle int) []string {
	dir := filepath.Join(projectRoot, ".evolve", "inbox", "processing", "cycle-"+strconv.Itoa(cycle))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var doc inboxbatch.Item
		if json.Unmarshal(raw, &doc) != nil || doc.ID == "" {
			continue
		}
		ids = append(ids, doc.ID)
	}
	return ids
}

// auditRepairSituation is the model_tier_overrides key an in-cycle audit-repair
// round activates. builder.json and tdd-engineer.json have declared it since
// the override table landed; this is its producer.
const auditRepairSituation = "audit_retry_2plus"

// repairRoundTier decides the dispatch-time raise for a tdd/build re-entry
// INSIDE an audit-repair round (CycleState.AuditRepairActive — the same
// persisted flag the repair brief derives from, so the live loop and the
// crash-resume path cannot diverge). The target tier is the phase profile's
// declared model_tier_overrides[audit_retry_2plus]; no declaration ⇒ the rule
// is inert (config decides, not Go). Raise-only and envelope-clamped through
// clampedRaise, exactly as the ADR-0076 D floor.
//
// Why: repair rounds re-dispatched at the identical tier and effort do not
// converge — cycles 1595–1605 ran balanced/balanced/balanced and ship
// probability by audit-round count fell 100 % → 50 % → 17 % → 0 % (research:
// docs/research/ship-rate-harness-reliability-2026-09-02.md, R1). Effort
// follows the tier via the profile's effort_overrides at the bridge launch.
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
