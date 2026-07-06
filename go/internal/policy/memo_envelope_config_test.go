package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// memo_envelope_config_test.go — RED contract for cycle-573 Task 1
// (memo-phase-tier-envelope, inbox weight 0.95 critical). The shipped config
// carries a split: .evolve/policy.json pins the memo phase to model "fast"
// (tier rank 1) while .evolve/profiles/memo.json declares a
// model_tier_envelope of [balanced..balanced] (rank 2). Every PASS cycle where
// memo actually dispatches records the abnormal
//
//	policy: pin for phase "memo": model "fast" (tier rank 1) outside envelope [balanced..balanced]
//
// Per phase_settings_from_config_not_code, the fix is config-only: align the
// pin's tier to the profile envelope (or vice-versa) in the shipped JSON — no
// Go literal changes. These tests read the SHIPPED files (same locator the
// profile routing tests use: filepath.Join("..","..","..",".evolve",...)) so
// they pin the on-disk contract, not a fixture.
//
// RED today: TestMemoPin_WithinShippedEnvelope fails because ValidatePin
// returns the "outside envelope" error for fast-vs-balanced. GREEN once the
// config drift is resolved.

func shippedMemoPinAndProfile(t *testing.T) (policy.Pin, *profiles.Profile) {
	t.Helper()
	pol, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load shipped policy.json: %v", err)
	}
	pin, ok := pol.PinFor("memo")
	if !ok {
		t.Fatalf("shipped policy.json has no pins.memo entry — Task 1 assumes memo is pinned")
	}
	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	prof, err := loader.Get("memo")
	if err != nil {
		t.Fatalf("load shipped memo profile: %v", err)
	}
	return pin, &prof
}

// TestMemoPin_WithinShippedEnvelope — AC-1a: the memo phase's pinned model tier,
// as shipped, must satisfy the memo profile's model_tier_envelope. This is the
// exact check the loader runs before dispatch; a non-nil error here is the
// "outside envelope" abnormal that ends otherwise-PASS cycles. Exercises the
// real resolver (ValidatePin) against the real shipped config.
func TestMemoPin_WithinShippedEnvelope(t *testing.T) {
	pin, prof := shippedMemoPinAndProfile(t)
	if prof.ModelTierEnvelope == nil {
		t.Fatalf("memo profile has no model_tier_envelope — the drift this task fixes assumes one exists")
	}
	if err := policy.ValidatePin("memo", pin, prof); err != nil {
		t.Errorf("shipped memo pin outside its profile envelope (config drift, config-only fix): %v", err)
	}
}

// TestMemoPin_TierRankMatchesEnvelope — AC-1b (semantic, distinct behaviour):
// beyond "no error", assert the pinned tier rank actually lands inside the
// envelope's [min..max] rank band. Guards against a fix that satisfies
// ValidatePin by a loophole (e.g. an unclassifiable model string, rank 0, which
// ValidatePin skips) rather than by genuinely aligning the tiers.
func TestMemoPin_TierRankMatchesEnvelope(t *testing.T) {
	pin, prof := shippedMemoPinAndProfile(t)
	if pin.Model == "" || prof.ModelTierEnvelope == nil {
		t.Fatalf("memo pin model / profile envelope missing — cannot assert tier alignment")
	}
	rank := policy.TierRank(pin.Model)
	minR := policy.TierRank(prof.ModelTierEnvelope.Min)
	maxR := policy.TierRank(prof.ModelTierEnvelope.Max)
	if rank == 0 {
		t.Errorf("memo pin model %q is unclassifiable (rank 0) — a real tier alignment, not an envelope-skip loophole, is required", pin.Model)
	}
	if rank < minR || rank > maxR {
		t.Errorf("memo pin model %q (rank %d) outside envelope rank band [%d..%d] (%s..%s)",
			pin.Model, rank, minR, maxR, prof.ModelTierEnvelope.Min, prof.ModelTierEnvelope.Max)
	}
}

// TestValidatePin_StillRejectsOutOfEnvelope — AC-1c (negative / anti-no-op):
// the config fix MUST NOT be achieved by gutting envelope enforcement. A
// fabricated fast pin against a balanced-only envelope must still error. This
// stays GREEN before and after the fix; it fails only if someone "resolves" the
// drift by weakening ValidatePin instead of aligning config.
func TestValidatePin_StillRejectsOutOfEnvelope(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "balanced"}}
	if err := policy.ValidatePin("memo", policy.Pin{Model: "fast"}, prof); err == nil {
		t.Errorf("ValidatePin must still reject a fast pin under a balanced-only envelope; got nil (enforcement gutted)")
	}
}
