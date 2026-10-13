package policy

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestWorkflowConfig_EveryOverrideReplacesItsDefault(t *testing.T) {
	pol, err := Parse([]byte(`{"workflow":{
		"max_consecutive_fails": 4,
		"max_cycles_cap": 9,
		"auto_prune": false,
		"backfill_enabled": false,
		"cycle_budget": "advisory",
		"allow_doc_delete": true,
		"diff_complexity_disable": true,
		"auditor_tier_override": "deep",
		"phase_enables": {"triage": "off"},
		"consensus_audit_enabled": false,
		"psmas_enabled": true,
		"strict_audit": true,
		"compact_prompts": false,
		"universal_fallback": false,
		"universal_fallback_exclude": ["codex"],
		"remediation_rounds": 3,
		"remediable_phases": ["coverage-gate", "docs-gate"],
		"build_floor": false,
		"size_budget_multipliers": {"large": 2.5, "medium": 0, "trivial": -1},
		"interactive_policy": "first",
		"interactive_policies": {"scout": "recommended_or_first"}
	}}`))
	if err != nil {
		t.Fatal(err)
	}

	c := pol.WorkflowConfig()

	checks := map[string]bool{
		"MaxConsecutiveFails":      c.MaxConsecutiveFails == 4,
		"MaxCyclesCap":             c.MaxCyclesCap == 9,
		"AutoPrune":                !c.AutoPrune,
		"BackfillEnabled":          !c.BackfillEnabled,
		"CycleBudget":              c.CycleBudget == "advisory",
		"AllowDocDelete":           c.AllowDocDelete,
		"DiffComplexityDisable":    c.DiffComplexityDisable,
		"AuditorTierOverride":      c.AuditorTierOverride == "deep",
		"PhaseEnables":             reflect.DeepEqual(c.PhaseEnables, map[string]string{"triage": "off"}),
		"ConsensusAuditEnabled":    !c.ConsensusAuditEnabled,
		"PSMASEnabled":             c.PSMASEnabled,
		"StrictAudit":              c.StrictAudit,
		"CompactPrompts":           !c.CompactPrompts,
		"UniversalFallback":        !c.UniversalFallback,
		"UniversalFallbackExclude": reflect.DeepEqual(c.UniversalFallbackExclude, []string{"codex"}),
		"RemediationRounds":        c.RemediationRounds == 3,
		"RemediablePhases":         reflect.DeepEqual(c.RemediablePhases, []string{"coverage-gate", "docs-gate"}),
		"BuildFloorEnforced":       !c.BuildFloorEnforced,
		"SizeBudget large":         c.SizeBudgetMultipliers["large"] == 2.5,
		"SizeBudget medium kept":   c.SizeBudgetMultipliers["medium"] == 1.25,
		"SizeBudget trivial kept":  c.SizeBudgetMultipliers["trivial"] == 1.0,
		"InteractivePolicy":        c.InteractivePolicy == "first",
		"InteractivePolicies":      reflect.DeepEqual(c.InteractivePolicies, map[string]string{"scout": "recommended_or_first"}),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("override %s was not applied: %+v", name, c)
		}
	}
}

func TestWorkflowConfig_ReturnedSlicesAndMapsAreCopies(t *testing.T) {
	pol, err := Parse([]byte(`{"workflow":{"universal_fallback_exclude":["codex"],"remediable_phases":["a"],"phase_enables":{"x":"on"},"interactive_policies":{"s":"first"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := pol.WorkflowConfig()
	c.UniversalFallbackExclude[0] = "mutated"
	c.RemediablePhases[0] = "mutated"
	c.PhaseEnables["x"] = "mutated"
	c.InteractivePolicies["s"] = "mutated"

	again := pol.WorkflowConfig()

	if again.UniversalFallbackExclude[0] != "codex" || again.RemediablePhases[0] != "a" || again.PhaseEnables["x"] != "on" || again.InteractivePolicies["s"] != "first" {
		t.Fatalf("mutating a returned config leaked into the policy: %+v", again)
	}
}

func TestWorkflowConfig_NoWorkflowBlockYieldsTheCompiledDefaults(t *testing.T) {
	c := Policy{}.WorkflowConfig()
	if c.MaxConsecutiveFails != 1 || c.MaxCyclesCap != 25 || c.CycleBudget != "enforce" || !c.AutoPrune || !c.BackfillEnabled ||
		!c.ConsensusAuditEnabled || !c.CompactPrompts || !c.UniversalFallback || !c.BuildFloorEnforced || c.InteractivePolicy != "recommended_or_first" {
		t.Fatalf("defaults = %+v", c)
	}
}

func TestFailurePolicyConfig_OverridesMergeButFloorCategoriesAreRestamped(t *testing.T) {
	def := DefaultSystemFailurePolicy()
	pol, err := Parse([]byte(`{"failure_policy":{
		"categories": {
			"code-build-fail": {"level":"task","action":"defer-or-quarantine","max_retries":5},
			"` + CategoryInfraSystemic + `": {"level":"task","action":"retry-with-fix"}
		},
		"thresholds": {"repeat_ceiling": 7},
		"on_task_retry_ceiling": "defer",
		"on_system_level": "halt-quietly"
	}}`))
	if err != nil {
		t.Fatal(err)
	}

	got, err := pol.FailurePolicyConfig()

	if err != nil {
		t.Fatal(err)
	}
	if row := got.Categories["code-build-fail"]; row.Action != ActionDeferOrQuarantine || row.MaxRetries != 5 {
		t.Errorf("override row = %+v", row)
	}
	if got.Categories[CategoryInfraSystemic] != def.Categories[CategoryInfraSystemic] {
		t.Errorf("a floor category must be re-stamped from the defaults: %+v", got.Categories[CategoryInfraSystemic])
	}
	if got.OnTaskRetryCeiling != "defer" || got.OnSystemLevel != "halt-quietly" {
		t.Errorf("scalar overrides lost: %+v", got)
	}
	if got.Thresholds.RepeatCeiling != 7 || got.Thresholds.TaskRetryCeiling != def.Thresholds.TaskRetryCeiling {
		t.Errorf("thresholds = %+v", got.Thresholds)
	}
}

func TestFailurePolicyConfig_EmptyScalarOverridesKeepTheDefaults(t *testing.T) {
	def := DefaultSystemFailurePolicy()
	got, err := Policy{SystemFailurePolicy: &SystemFailurePolicy{}}.FailurePolicyConfig()
	if err != nil || got.OnTaskRetryCeiling != def.OnTaskRetryCeiling || got.OnSystemLevel != def.OnSystemLevel {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestPatchBlocks_EncodingFailuresAreNamedAndNeverClobber(t *testing.T) {
	existing := []byte(`{"keep":{"a":1}}`)

	t.Run("a value that cannot be encoded", func(t *testing.T) {
		out, err := PatchBlocks(existing, map[string]any{"bad": math.NaN()})
		if err == nil || !strings.Contains(err.Error(), "policy: encoding bad") || out != nil {
			t.Fatalf("out=%s err=%v", out, err)
		}
	})
	t.Run("the final document cannot be encoded", func(t *testing.T) {
		orig := patchBlocksMarshalIndent
		t.Cleanup(func() { patchBlocksMarshalIndent = orig })
		boom := errors.New("encoder down")
		patchBlocksMarshalIndent = func(any, string, string) ([]byte, error) { return nil, boom }
		out, err := PatchBlocks(existing, map[string]any{"fleet": map[string]any{"count": 3}})
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "policy: encoding policy.json") || out != nil {
			t.Fatalf("out=%s err=%v", out, err)
		}
	})
	t.Run("a nil value deletes its block and keeps the others", func(t *testing.T) {
		out, err := PatchBlocks(existing, map[string]any{"gone": nil, "keep": nil, "new": 1})
		if err != nil || strings.Contains(string(out), "keep") || !strings.Contains(string(out), `"new": 1`) || !strings.HasSuffix(string(out), "}\n") {
			t.Fatalf("out=%s err=%v", out, err)
		}
	})
}
