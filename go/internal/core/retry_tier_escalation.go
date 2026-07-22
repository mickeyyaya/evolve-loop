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
	if policy.TierRank(currentTier) >= policy.TierRank(retryEscalationTier) {
		return "", false // raise-only: never lower an equal/higher proposal
	}
	// Clamp through the one true guardrail: a single-entry plan through
	// ClampPlanModelRouting applies the build profile's envelope (Max wins).
	tmp := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: string(PhaseBuild), Run: true, Tier: retryEscalationTier}}}
	profileFor := func(phase string) *profiles.Profile {
		return cr.o.profileForModelRouting(cr.req.ProjectRoot, phase)
	}
	clamped, _ := router.ClampPlanModelRouting(tmp, profileFor, cr.o.modelCatalogLookup)
	tier := clamped.Entries[0].Tier
	if policy.TierRank(tier) <= policy.TierRank(currentTier) {
		return "", false // envelope pulled the raise back — no effective gain
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
