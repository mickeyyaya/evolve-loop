package config_test

// defaults_parity_test.go — the two-source gate defaults (config.defaults()
// vs policy's compiled accessors, applied unconditionally over config's at the
// root) are a replicated belief acs/cycle34 records in prose; this is the
// consumer pin that turns it into a tested one (ADR-0103 unit 08 test 36; F6
// unifies them once the operator picks the winner). External test package so
// it can import policy (policy imports config).

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// jsonTag is the name half of a struct field's `json:"name,omitempty"` tag —
// the spelling .evolve/policy.json actually carries for that field.
func jsonTag(t *testing.T, typ reflect.Type, field string) string {
	t.Helper()
	f, ok := typ.FieldByName(field)
	if !ok {
		t.Fatalf("%s has no field %s", typ, field)
	}
	return strings.Split(f.Tag.Get("json"), ",")[0]
}

// TestPolicyStages_KeysMatchPolicyJSONTags — the nine policy keys
// ApplyPolicyStages names in CONFIG_UNKNOWN_VALUE (fields.key and the message)
// are a SECOND home of policy.json's spellings; this is their consumer pin:
// each must equal <section tag on policy.Policy>.<field tag on the block>
// (architecture-review fold, ADR-0103 unit 08). A policy rename that leaves
// config's label behind would otherwise point a triage at a key that no
// longer exists.
func TestPolicyStages_KeysMatchPolicyJSONTags(t *testing.T) {
	policyType := reflect.TypeOf(policy.Policy{})
	dials := []struct {
		stagesField, section, blockField string
		block                            reflect.Type
	}{
		{"ContractGate", "Gates", "ContractGate", reflect.TypeOf(policy.GatesPolicy{})},
		{"EvalGate", "Gates", "EvalGate", reflect.TypeOf(policy.GatesPolicy{})},
		{"TriageCapGate", "Gates", "TriageCapGate", reflect.TypeOf(policy.GatesPolicy{})},
		{"TopNGate", "Gates", "TopNGate", reflect.TypeOf(policy.GatesPolicy{})},
		{"ReviewGate", "Gates", "ReviewGate", reflect.TypeOf(policy.GatesPolicy{})},
		{"PhaseRecovery", "Recovery", "PhaseRecovery", reflect.TypeOf(policy.RecoveryPolicy{})},
		{"SpineFloor", "Recovery", "SpineFloor", reflect.TypeOf(policy.RecoveryPolicy{})},
		{"FatalPane", "Recovery", "FatalPane", reflect.TypeOf(policy.RecoveryPolicy{})},
		{"RouterReplan", "Router", "RouterReplan", reflect.TypeOf(policy.RouterPolicy{})},
		{"ParallelEvaluate", "ParallelEvaluate", "Stage", reflect.TypeOf(policy.ParallelEvaluatePolicy{})},
	}
	for _, d := range dials {
		wantKey := jsonTag(t, policyType, d.section) + "." + jsonTag(t, d.block, d.blockField)
		// Every stage word valid ("off") except the one under test, so exactly
		// one warning fires and it must carry policy.json's own spelling.
		ps := config.PolicyStages{ContractGate: "off", EvalGate: "off", TriageCapGate: "off", TopNGate: "off", ReviewGate: "off", PhaseRecovery: "off", SpineFloor: "off", FatalPane: "off", RouterReplan: "off", ParallelEvaluate: "off"}
		reflect.ValueOf(&ps).Elem().FieldByName(d.stagesField).SetString("typo")
		_, ws := config.New().ApplyPolicyStages(config.RoutingConfig{}, ps)
		if len(ws) != 1 || ws[0].Fields["key"] != wantKey || !strings.HasPrefix(ws[0].Message, wantKey+`="typo"`) {
			t.Errorf("PolicyStages.%s: want key %q (policy.json's tag), got warnings %+v", d.stagesField, wantKey, ws)
		}
	}
}

func TestDefaults_GateDialsMatchPolicyCompiledDefaults(t *testing.T) {
	cfg, ws := config.Load("/nonexistent/phase-registry.json", nil)
	if len(ws) != 0 {
		t.Fatalf("unexpected warnings: %+v", ws)
	}
	var pol policy.Policy
	gates, recovery, router, pe := pol.GatesConfig(), pol.RecoveryConfig(), pol.RouterConfig(), pol.ParallelEvaluateConfig()
	gate := func(v string) config.Stage { s, _ := config.GateStage(v); return s }
	route := func(v string) config.Stage { s, _ := config.RouterStage(v); return s }
	for name, pair := range map[string][2]config.Stage{
		"ContractGate":     {cfg.ContractGate, gate(gates.ContractGate)},
		"EvalGate":         {cfg.EvalGate, gate(gates.EvalGate)},
		"TriageCapGate":    {cfg.TriageCapGate, gate(gates.TriageCapGate)},
		"TopNGate":         {cfg.TopNGate, gate(gates.TopNGate)},
		"ReviewGate":       {cfg.ReviewGate, gate(gates.ReviewGate)},
		"PhaseRecovery":    {cfg.PhaseRecovery, gate(recovery.PhaseRecovery)},
		"SpineFloor":       {cfg.SpineFloor, gate(recovery.SpineFloor)},
		"FatalPane":        {cfg.FatalPane, gate(recovery.FatalPane)},
		"RouterReplan":     {cfg.RouterReplan, route(router.RouterReplan)},
		"ParallelEvaluate": {cfg.ParallelEvaluate, route(pe.Stage)},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s: config default %v != policy compiled default %v — the two-source belief drifted (F6)", name, pair[0], pair[1])
		}
	}
	if cfg.ParallelEvaluateConcurrency != pe.Concurrency || cfg.ParallelEvaluateConcurrency != 3 || cfg.RePlanMaxDepth != router.ReplanDepth {
		t.Errorf("concurrency %d/%d, replan depth %d/%d", cfg.ParallelEvaluateConcurrency, pe.Concurrency, cfg.RePlanMaxDepth, router.ReplanDepth)
	}
}
