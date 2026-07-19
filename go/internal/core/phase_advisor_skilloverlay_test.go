package core

import "testing"

// phase_advisor_skilloverlay_test.go — RED test for AC2 of
// overlay-injection-dormant-wire-fable-deep (cycle-954, inbox
// 2026-07-14T00-50-00Z): the inbox item's Hypothesis 3 claims Engine.Launch is
// "the single chokepoint shared by phase + advisor dispatches — avoids
// duplicating the compose logic at every call site". Verified FALSE by
// inspection: PhaseAdvisor.advisorLaunch (phase_advisor.go:278) builds its own
// BridgeRequest independently of phases/runner/runner.go and never sets
// Skills, so a deep/top-tier advisor dispatch never receives the fable
// overlay a deep/top-tier PHASE dispatch already gets (see
// phases/runner/runner_skilloverlay_test.go, shipped commit daf993e8/#333).
//
// This is the one real remaining gap in an otherwise fully-shipped feature —
// AC1/3/4/5/7 and the AC6 policy-layer contract are pre-existing GREEN
// (test-report.md documents each). AC2's "phase dispatch" half is GREEN; only
// its "non-phase advisor Launch" half is RED here.
//
// Fix convention: mirror runner.go:678 exactly — pass identity.Model as BOTH
// the Model and the Tier argument to policy.DispatchFromPhaseRequest ("the
// tier string flows straight into BridgeRequest.Model ... passing the tier is
// as literal as passing plan.Model", runner.go:660-663). WithProposerModel
// already accepts tier-shaped values ("deep"/"top"; see
// phase_advisor_tier_test.go's sanitizeAdvisorTier vocabulary), so no new
// field is needed on AgentIdentity.
func TestAdvisorLaunch_DeepModelResolvesFableOverlay(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	adv := NewPhaseAdvisor(fb, WithProposerModel("deep"))
	if _, err := adv.Plan(baseRouteInput()); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(fb.gotReq.Skills) != 1 || fb.gotReq.Skills[0] != "fable" {
		t.Errorf("gotReq.Skills=%v, want [fable] — a deep-tier advisor dispatch must resolve the compiled overlay, same as a deep-tier phase dispatch", fb.gotReq.Skills)
	}
}

// TestAdvisorLaunch_OpusModelNoOverlay is the negative twin: the DEFAULT
// identity.Model ("opus", NewPhaseAdvisor's fallback) does not match the
// compiled default's tiers:[deep,top] selector, so the advisor dispatch must
// carry no overlay — proves the fix doesn't unconditionally inject fable into
// every advisor Launch regardless of tier.
func TestAdvisorLaunch_OpusModelNoOverlay(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}
	adv := NewPhaseAdvisor(fb) // default identity.Model = "opus"
	if _, err := adv.Plan(baseRouteInput()); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(fb.gotReq.Skills) != 0 {
		t.Errorf("gotReq.Skills=%v, want none — the compiled default only matches deep/top tiers, not a raw model name", fb.gotReq.Skills)
	}
}
