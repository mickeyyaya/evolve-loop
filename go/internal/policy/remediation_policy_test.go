package policy

import "testing"

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

func TestWorkflowConfig_MutatingResultDoesNotChangeNextResolve(t *testing.T) {
	p := Policy{Workflow: &WorkflowPolicy{
		PhaseEnables:     map[string]string{"tdd": "on"},
		RemediablePhases: []string{"coverage-gate", "build-floor"},
	}}
	first := p.WorkflowConfig()
	first.PhaseEnables["tdd"] = "off"
	first.PhaseEnables["injected"] = "on"
	first.RemediablePhases[0] = "audit"

	second := p.WorkflowConfig()
	if len(second.PhaseEnables) != 1 || second.PhaseEnables["tdd"] != "on" {
		t.Errorf("PhaseEnables leaked a caller's write into the next resolve: %v", second.PhaseEnables)
	}
	if len(p.Workflow.PhaseEnables) != 1 || p.Workflow.PhaseEnables["tdd"] != "on" {
		t.Errorf("PhaseEnables leaked a caller's write into the loaded policy: %v", p.Workflow.PhaseEnables)
	}
	if second.RemediablePhases[0] != "coverage-gate" || p.Workflow.RemediablePhases[0] != "coverage-gate" {
		t.Errorf("RemediablePhases leaked a caller's write: resolved %v, loaded %v", second.RemediablePhases, p.Workflow.RemediablePhases)
	}

	empty := Policy{Workflow: &WorkflowPolicy{RemediablePhases: []string{}}}.WorkflowConfig()
	if empty.RemediablePhases == nil || len(empty.RemediablePhases) != 0 {
		t.Errorf("an explicit empty remediable_phases must resolve to an empty, non-nil list: %#v", empty.RemediablePhases)
	}
	if unset := (Policy{Workflow: &WorkflowPolicy{}}).WorkflowConfig(); unset.PhaseEnables != nil {
		t.Errorf("an unset phase_enables must stay nil: %#v", unset.PhaseEnables)
	}
}
