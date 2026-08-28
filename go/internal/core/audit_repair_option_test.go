package core

// audit_repair_option_test.go — WIRING PROOF for the audit-repair cap.
//
// The first version of this file tested a dedicated Orchestrator field fed by a
// dedicated Option. Both reviewers found the same defect independently: that
// Option was never called from the production composition root, so
// `.evolve/policy.json:workflow.max_audit_repair_attempts` — including the
// documented "0 disables repair" off-switch — never reached the running
// orchestrator. The tests passed because they called the Option themselves.
//
// The knob now rides the SINGLE workflow-config surface every sibling knob uses
// (StrictAudit, PSMASEnabled, BackfillEnabled, PhaseEnables), which cmd_cycle.go
// already injects unconditionally via WithWorkflowConfig. Two sources of truth
// for one knob was the bug; this file pins that there is now one.

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// The cap the decision actually consults must be the one policy resolved — not a
// parallel field a composition root has to remember to set.
func TestAuditRepairCap_ComesFromResolvedWorkflowConfig(t *testing.T) {
	tests := []struct {
		name string
		give int
	}{
		{name: "an operator may disable repair entirely", give: 0},
		{name: "an operator may raise the cap", give: 5},
		{name: "the compiled default rides the same surface", give: policy.DefaultMaxAuditRepairAttempts},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := policy.WorkflowConfig{MaxAuditRepairAttempts: tc.give}
			o := NewOrchestrator(nil, nil, nil, WithWorkflowConfig(cfg))

			if got := o.auditRepairCap(); got != tc.give {
				t.Errorf("auditRepairCap() = %d, want %d — the resolved policy value did not reach the decision", got, tc.give)
			}
		})
	}
}

// An absent workflow block must still yield the compiled default rather than 0,
// because 0 MEANS disabled — a constructor that leaves the zero value would ship
// the feature dark while every decision test stayed green.
func TestAuditRepairCap_AbsentWorkflowBlockUsesCompiledDefault(t *testing.T) {
	o := NewOrchestrator(nil, nil, nil, WithWorkflowConfig(policy.Policy{}.WorkflowConfig()))

	if got := o.auditRepairCap(); got != policy.DefaultMaxAuditRepairAttempts {
		t.Errorf("auditRepairCap() = %d, want the compiled default %d", got, policy.DefaultMaxAuditRepairAttempts)
	}
}

// There must be no second way to set this knob. A dedicated Option was the
// original defect: it existed, it worked in tests, and production never called
// it. This fails if one is reintroduced.
func TestAuditRepairCap_HasNoParallelOption(t *testing.T) {
	for _, tc := range []struct{ file, symbol string }{
		{"orchestrator.go", "WithMaxAuditRepairAttempts"},
		{"orchestrator.go", "maxAuditRepairAttempts"},
		{"decision_branch.go", "o.maxAuditRepairAttempts"},
	} {
		body, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		if strings.Contains(string(body), tc.symbol) {
			t.Errorf("%s reintroduces %q — a second source of truth for the repair cap is the original defect: it works in tests and production never sets it",
				tc.file, tc.symbol)
		}
	}
}
