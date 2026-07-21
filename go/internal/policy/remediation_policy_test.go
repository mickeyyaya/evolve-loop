package policy

import "testing"

// TestWorkflowConfig_RemediationDefaults pins the graduated-remediation
// compiled defaults (2026-07-21): ON at 1 round for coverage-gate only, with
// policy.json workflow overrides honored — including explicit 0 to disable.
func TestWorkflowConfig_RemediationDefaults(t *testing.T) {
	c := Policy{}.WorkflowConfig()
	if c.RemediationRounds != 1 {
		t.Fatalf("default RemediationRounds=%d, want 1", c.RemediationRounds)
	}
	if len(c.RemediablePhases) != 1 || c.RemediablePhases[0] != "coverage-gate" {
		t.Fatalf("default RemediablePhases=%v, want [coverage-gate]", c.RemediablePhases)
	}
	zero := 0
	over := Policy{Workflow: &WorkflowPolicy{RemediationRounds: &zero, RemediablePhases: []string{"coverage-gate", "type-safety-audit"}}}.WorkflowConfig()
	if over.RemediationRounds != 0 {
		t.Fatalf("override rounds=0 must disable; got %d", over.RemediationRounds)
	}
	if len(over.RemediablePhases) != 2 {
		t.Fatalf("override phases not honored: %v", over.RemediablePhases)
	}
	if !c.BuildFloorEnforced {
		t.Fatal("BuildFloorEnforced must default true")
	}
	off := false
	offCfg := Policy{Workflow: &WorkflowPolicy{BuildFloor: &off}}.WorkflowConfig()
	if offCfg.BuildFloorEnforced {
		t.Fatal("build_floor=false override must disable")
	}
}
