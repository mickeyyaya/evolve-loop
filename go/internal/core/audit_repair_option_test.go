package core

// audit_repair_option_test.go — composition-root wiring for the audit-repair cap
// (ADR-0092).
//
// Two things can silently disable the repair loop without failing any behavioural
// test: the constructor forgetting the compiled default (leaving the zero value,
// which MEANS disabled), and the Option not reaching the field. Both would look
// exactly like "repair never fires" in production while every decision-level test
// stayed green — the same shape as the router eating the grant, which only a
// live-path test caught.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// The zero value of the field means DISABLED, so a constructor that forgets to
// seed the default ships the feature dark. This pins the seed itself.
func TestNewOrchestrator_SeedsTheCompiledRepairDefault(t *testing.T) {
	o := NewOrchestrator(nil, nil, nil)

	if o.maxAuditRepairAttempts != policy.DefaultMaxAuditRepairAttempts {
		t.Errorf("maxAuditRepairAttempts = %d, want the compiled default %d — an unseeded zero disables repair entirely",
			o.maxAuditRepairAttempts, policy.DefaultMaxAuditRepairAttempts)
	}
}

func TestWithMaxAuditRepairAttempts(t *testing.T) {
	tests := []struct {
		name string
		give int
	}{
		{name: "an operator may disable repair", give: 0},
		{name: "an operator may raise the cap", give: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(nil, nil, nil, WithMaxAuditRepairAttempts(tc.give))

			if o.maxAuditRepairAttempts != tc.give {
				t.Errorf("maxAuditRepairAttempts = %d, want %d — the option did not reach the field",
					o.maxAuditRepairAttempts, tc.give)
			}
		})
	}
}

// The option must beat the compiled default rather than being overwritten by it:
// options are applied AFTER the struct literal, and an ordering regression would
// silently restore 2 for an operator who asked for 0.
func TestWithMaxAuditRepairAttempts_OverridesTheCompiledDefault(t *testing.T) {
	o := NewOrchestrator(nil, nil, nil, WithMaxAuditRepairAttempts(0))

	if o.maxAuditRepairAttempts == policy.DefaultMaxAuditRepairAttempts {
		t.Error("the compiled default overwrote an explicit operator override; repair cannot be disabled")
	}
}
