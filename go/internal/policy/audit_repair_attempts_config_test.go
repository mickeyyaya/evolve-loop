package policy

// audit_repair_attempts_config_test.go — the resolution contract for the
// in-cycle audit-repair cap (ADR-0092).
//
// The field is a *int rather than an int for one reason: an explicit 0 must be
// distinguishable from an absent key. Absent means "use the compiled default";
// 0 means "the operator disabled repair". With a plain int those two collapse
// and the off switch silently becomes unreachable — the feature could never be
// turned off without a code change, which is exactly the feature-flag shape the
// repo forbids.

import "testing"

func TestWorkflowConfig_MaxAuditRepairAttempts(t *testing.T) {
	zero, two := 0, 2
	five := 5

	tests := []struct {
		name string
		p    Policy
		want int
	}{
		{
			name: "absent workflow block falls back to the compiled default",
			p:    Policy{},
			want: DefaultMaxAuditRepairAttempts,
		},
		{
			// The distinction the pointer exists for.
			name: "an explicit 0 disables repair and is NOT read as absent",
			p:    Policy{Workflow: &WorkflowPolicy{MaxAuditRepairAttempts: &zero}},
			want: 0,
		},
		{
			name: "an explicit value equal to the default is honored",
			p:    Policy{Workflow: &WorkflowPolicy{MaxAuditRepairAttempts: &two}},
			want: 2,
		},
		{
			name: "an operator may raise the cap",
			p:    Policy{Workflow: &WorkflowPolicy{MaxAuditRepairAttempts: &five}},
			want: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.WorkflowConfig().MaxAuditRepairAttempts; got != tc.want {
				t.Errorf("MaxAuditRepairAttempts = %d, want %d", got, tc.want)
			}
		})
	}
}

// The compiled default is a product decision, not an accident: two attempts
// absorb the staged-index and narrative-fidelity rejections that dominated
// wave 3, while still reaching retro promptly on a genuinely broken cycle.
// Pinned so a change is deliberate and reviewed rather than incidental.
func TestDefaultMaxAuditRepairAttempts_IsTwo(t *testing.T) {
	if DefaultMaxAuditRepairAttempts != 2 {
		t.Errorf("DefaultMaxAuditRepairAttempts = %d, want 2", DefaultMaxAuditRepairAttempts)
	}
}
