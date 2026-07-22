package core

// retry_tier_escalation_test.go — RED contract for ADR-0076 slice D: an inbox
// item with failure_count >= threshold routes its NEXT attempt's BUILD phase
// to the deep tier. Raise-only via policy.TierRank (an advisor "top" proposal
// is never lowered); the existing ClampPlanModelRouting envelope-Max clamp
// runs AFTER the raise and still wins (pinned here through the real clamp).
// Batches 6-8 evidence: hard items re-failed at the same tier across
// attempts; deep-tier audit reliably caught what balanced-tier build could
// not finish (ADR-0076 §context).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func planWithBuild(tier string) *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "build", Run: true, Tier: tier},
		{Phase: "audit", Run: true, Tier: "top"},
	}}
}

func TestRaiseBuildTierForRetry_RaisedAtThreshold(t *testing.T) {
	plan := planWithBuild("balanced")
	raised := raiseBuildTierForRetry(plan, 1, 1)
	if !raised {
		t.Fatal("failure_count == threshold must raise")
	}
	if got := plan.Entries[1].Tier; got != "deep" {
		t.Fatalf("build tier must be raised to deep, got %q", got)
	}
	if plan.Entries[0].Tier != "" || plan.Entries[2].Tier != "top" {
		t.Fatal("non-build entries must be untouched")
	}
}

func TestRaiseBuildTierForRetry_BelowThresholdUnchanged(t *testing.T) {
	plan := planWithBuild("balanced")
	if raised := raiseBuildTierForRetry(plan, 0, 1); raised {
		t.Fatal("count below threshold must not raise")
	}
	if plan.Entries[1].Tier != "balanced" {
		t.Fatal("plan must be unchanged below threshold")
	}
}

func TestRaiseBuildTierForRetry_NeverLowersTopProposal(t *testing.T) {
	plan := planWithBuild("top")
	raiseBuildTierForRetry(plan, 3, 1)
	if plan.Entries[1].Tier != "top" {
		t.Fatalf("an advisor top proposal must never be lowered, got %q", plan.Entries[1].Tier)
	}
}

func TestRaiseBuildTierForRetry_EmptyTierRaised(t *testing.T) {
	// A static plan carries no tier (omitempty wire form) — the raise must
	// still apply so the retry benefits regardless of routing mode.
	plan := planWithBuild("")
	raiseBuildTierForRetry(plan, 2, 1)
	if plan.Entries[1].Tier != "deep" {
		t.Fatalf("empty tier must raise to deep, got %q", plan.Entries[1].Tier)
	}
}

func TestRaiseBuildTierForRetry_ZeroThresholdDisables(t *testing.T) {
	plan := planWithBuild("balanced")
	if raised := raiseBuildTierForRetry(plan, 5, 0); raised {
		t.Fatal("threshold 0 disables escalation (policy escape hatch)")
	}
}

func TestRaiseBuildTierForRetry_NilPlanSafe(t *testing.T) {
	if raised := raiseBuildTierForRetry(nil, 2, 1); raised {
		t.Fatal("nil plan (static routing) must be a safe no-op")
	}
}

// The raise composes with the REAL envelope clamp: a profile whose Max is
// balanced clamps the raised deep back down — Max still wins (ADR-0076 D).
func TestRaiseBuildTierForRetry_EnvelopeMaxStillClamps(t *testing.T) {
	plan := planWithBuild("balanced")
	raiseBuildTierForRetry(plan, 1, 1)
	profileFor := func(phase string) *profiles.Profile {
		if phase != "build" {
			return nil
		}
		return &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Default: "balanced", Max: "balanced"}}
	}
	clamped, _ := router.ClampPlanModelRouting(plan, profileFor, nil)
	if got := clamped.Entries[1].Tier; got != "balanced" {
		t.Fatalf("envelope Max must clamp the raise back down, got %q", got)
	}
}

func TestEscalateRetryTier_MaxCountAcrossScopeIDs(t *testing.T) {
	plan := planWithBuild("balanced")
	reader := func(id string) int {
		return map[string]int{"item-a": 0, "item-b": 2}[id]
	}
	if raised := escalateRetryTier(plan, "item-a,item-b", reader, 1, 42); !raised {
		t.Fatal("max failure_count across the scope must drive the raise")
	}
	if plan.Entries[1].Tier != "deep" {
		t.Fatalf("got %q", plan.Entries[1].Tier)
	}
}

func TestEscalateRetryTier_EmptyScopeOrNilReaderNoop(t *testing.T) {
	plan := planWithBuild("balanced")
	if escalateRetryTier(plan, "", func(string) int { return 9 }, 1, 42) {
		t.Fatal("empty scope (sequential path without ids) must be a no-op")
	}
	if escalateRetryTier(plan, "item-a", nil, 1, 42) {
		t.Fatal("nil reader (option not wired) must be a no-op")
	}
	if plan.Entries[1].Tier != "balanced" {
		t.Fatal("plan must be untouched")
	}
}

func TestWithFailureCountReader_SetsAndIgnoresNil(t *testing.T) {
	o := &Orchestrator{}
	WithFailureCountReader(func(string) int { return 7 })(o)
	if o.failureCountFor == nil || o.failureCountFor("x") != 7 {
		t.Fatal("reader not injected")
	}
	prev := o.failureCountFor
	WithFailureCountReader(nil)(o)
	if o.failureCountFor == nil {
		t.Fatal("nil must be ignored, keeping the prior reader")
	}
	_ = prev
}
