package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestHandoffPolicy_ExportNamed(t *testing.T) {
	var hp policy.HandoffPolicy
	if err := hp.UnmarshalJSON([]byte(`{"tdd-build": {"stage": "shadow"}}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	p := policy.Policy{Handoff: &hp}
	if p.Handoff == nil {
		t.Fatal("Handoff field must not be nil")
	}
	cfg := p.HandoffConfig()
	if cfg["tdd-build"].Stage != "shadow" {
		t.Fatalf("stage = %q, want shadow", cfg["tdd-build"].Stage)
	}
}
