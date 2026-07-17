package policy

import (
	"encoding/json"
	"testing"
)

// ADR-0072 S1: the failure_policy decision block. Absent ⇒ compiled defaults;
// the two floor categories (verdict-incoherence, infra-systemic) are
// non-negotiable — the resolver forces floor=true even if operator policy tries
// to unset them (mirrors ShipFloor always re-appending "audit").

func TestFailurePolicyConfig_AbsentBlock_CompiledDefaults(t *testing.T) {
	p := Policy{}
	fp, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All seven canonical categories present.
	want := []string{
		"verdict-incoherence", "infra-systemic", "transport-hang", "non-progress",
		"code-build-fail", "code-audit-fail", "intent-malformed",
	}
	for _, k := range want {
		if _, ok := fp.Categories[k]; !ok {
			t.Errorf("default categories missing %q", k)
		}
	}
	// Floor categories.
	if !fp.Categories["verdict-incoherence"].Floor {
		t.Error("verdict-incoherence must be floor=true by default")
	}
	if !fp.Categories["infra-systemic"].Floor {
		t.Error("infra-systemic must be floor=true by default")
	}
	// Levels.
	if fp.Categories["verdict-incoherence"].Level != "system" {
		t.Errorf("verdict-incoherence level = %q, want system", fp.Categories["verdict-incoherence"].Level)
	}
	if fp.Categories["code-build-fail"].Level != "task" {
		t.Errorf("code-build-fail level = %q, want task", fp.Categories["code-build-fail"].Level)
	}
	// Actions.
	if fp.Categories["non-progress"].Action != "halt-and-diagnose" {
		t.Errorf("non-progress action = %q, want halt-and-diagnose", fp.Categories["non-progress"].Action)
	}
	if fp.Categories["code-audit-fail"].Action != "retry-with-fix" {
		t.Errorf("code-audit-fail action = %q, want retry-with-fix", fp.Categories["code-audit-fail"].Action)
	}
	// Thresholds.
	if fp.Thresholds.RepeatCeiling <= 0 || fp.Thresholds.VerifiedNotLandedCeiling <= 0 || fp.Thresholds.TaskRetryCeiling <= 0 {
		t.Errorf("default thresholds must be positive, got %+v", fp.Thresholds)
	}
	if fp.OnSystemLevel != "halt-loop-and-escalate" {
		t.Errorf("on_system_level = %q, want halt-loop-and-escalate", fp.OnSystemLevel)
	}
}

func TestFailurePolicyConfig_FloorInvariant_CannotBeUnset(t *testing.T) {
	// Operator policy tries to demote a floor category to a retry. The resolver
	// must refuse: floor categories are Go-enforced halts, non-negotiable.
	raw := `{
	  "failure_policy": {
	    "categories": {
	      "verdict-incoherence": { "level":"task", "action":"retry-with-fix", "floor":false }
	    }
	  }
	}`
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fp, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := fp.Categories["verdict-incoherence"]
	if !c.Floor {
		t.Error("floor invariant breached: verdict-incoherence.floor was overridden to false")
	}
	if c.Level != "system" {
		t.Errorf("floor invariant breached: verdict-incoherence.level = %q, want system", c.Level)
	}
	if c.Action != "halt-and-diagnose" {
		t.Errorf("floor invariant breached: verdict-incoherence.action = %q, want halt-and-diagnose", c.Action)
	}
}

func TestFailurePolicyConfig_PartialOverride_MergesDefaults(t *testing.T) {
	raw := `{
	  "failure_policy": {
	    "thresholds": { "task_retry_ceiling": 3 },
	    "categories": {
	      "code-audit-fail": { "level":"task", "action":"retry-with-fix", "fix_type":"address-audit-findings", "max_retries":5 }
	    }
	  }
	}`
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fp, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp.Thresholds.TaskRetryCeiling != 3 {
		t.Errorf("task_retry_ceiling = %d, want 3 (override)", fp.Thresholds.TaskRetryCeiling)
	}
	// Un-overridden threshold keeps its default.
	if fp.Thresholds.RepeatCeiling <= 0 {
		t.Errorf("repeat_ceiling lost its default: %d", fp.Thresholds.RepeatCeiling)
	}
	if fp.Categories["code-audit-fail"].MaxRetries != 5 {
		t.Errorf("code-audit-fail max_retries = %d, want 5", fp.Categories["code-audit-fail"].MaxRetries)
	}
	// A category not mentioned in the override survives from defaults.
	if _, ok := fp.Categories["non-progress"]; !ok {
		t.Error("non-progress category lost when override supplied a partial category map")
	}
}

func TestFailurePolicyConfig_RejectsUnknownLevel(t *testing.T) {
	raw := `{"failure_policy":{"categories":{"custom-x":{"level":"weird","action":"retry-with-fix"}}}}`
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := p.FailurePolicyConfig(); err == nil {
		t.Error("expected error for unknown level 'weird', got nil")
	}
}
