package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func shippedMemoPinAndProfile(t *testing.T) (policy.Pin, *profiles.Profile) {
	t.Helper()
	pol, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load shipped policy.json: %v", err)
	}
	pin, ok := pol.PinFor("memo")
	if !ok {
		t.Skip("no pins.memo in shipped policy (preset default in force) — the envelope contract applies only to an existing pin")
	}
	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	prof, err := loader.Get("memo")
	if err != nil {
		t.Fatalf("load shipped memo profile: %v", err)
	}
	return pin, &prof
}

func TestMemoPin_WithinShippedEnvelope(t *testing.T) {
	pin, prof := shippedMemoPinAndProfile(t)
	if prof.ModelTierEnvelope == nil {
		t.Fatalf("memo profile has no model_tier_envelope — the drift this task fixes assumes one exists")
	}
	if err := policy.ValidatePin("memo", pin, prof); err != nil {
		t.Errorf("shipped memo pin outside its profile envelope (config drift, config-only fix): %v", err)
	}
}

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

func TestValidatePin_StillRejectsOutOfEnvelope(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "balanced"}}
	if err := policy.ValidatePin("memo", policy.Pin{Model: "fast"}, prof); err == nil {
		t.Errorf("ValidatePin must still reject a fast pin under a balanced-only envelope; got nil (enforcement gutted)")
	}
}
