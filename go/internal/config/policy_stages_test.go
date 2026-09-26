package config

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// sentinelStages differs from everything the positional fixture resolves to, so a dropped
// assignment is observable.
func sentinelStages() RoutingConfig {
	cfg := defaults()
	cfg.ContractGate, cfg.EvalGate, cfg.TriageCapGate, cfg.TopNGate, cfg.ReviewGate = StageAdvisory, StageAdvisory, StageAdvisory, StageAdvisory, StageAdvisory
	cfg.PhaseRecovery, cfg.SpineFloor, cfg.FatalPane = StageAdvisory, StageAdvisory, StageAdvisory
	cfg.RouterReplan, cfg.ParallelEvaluate = StageOff, StageEnforce
	cfg.ParallelEvaluateConcurrency, cfg.RePlanMaxDepth = -1, -1
	cfg.RoutingJudge, cfg.ReconDigest = false, false
	return cfg
}

func TestApplyPolicyStages_ProjectsEveryDialAndWarnsOnTypos(t *testing.T) {
	l, events := observed(t)
	in := sentinelStages()
	before := sentinelStages()
	ps := PolicyStages{"enforce", "enfroce", "shadow", "off", "enforce", "shadow", "enforce", "off", "advisory", "shadow", 5, true, true, 2}
	got, ws := l.ApplyPolicyStages(in, ps)
	want := sentinelStages()
	want.ContractGate, want.EvalGate, want.TriageCapGate, want.TopNGate, want.ReviewGate = StageEnforce, StageOff, StageShadow, StageOff, StageEnforce
	want.PhaseRecovery, want.SpineFloor, want.FatalPane = StageShadow, StageEnforce, StageOff
	want.RouterReplan, want.ParallelEvaluate = StageAdvisory, StageShadow
	want.ParallelEvaluateConcurrency, want.RePlanMaxDepth = 5, 2
	want.RoutingJudge, want.ReconDigest = true, true
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved:\n got %+v\nwant %+v", got.RolloutStages, want.RolloutStages)
	}
	if !reflect.DeepEqual(in, before) {
		t.Errorf("the input cfg is a value: %+v", in.RolloutStages)
	}
	wantFields := map[string]string{"key": "gates.eval_gate", "value": "enfroce", "default": "off", "step": "policy", "source": "policy"}
	if len(ws) != 1 || ws[0].Code != "unknown-value" || ws[0].Message != `gates.eval_gate="enfroce" unknown (want off|shadow|enforce), defaulting to off` || !reflect.DeepEqual(ws[0].Fields, wantFields) {
		t.Fatalf("warnings: %+v", ws)
	}
	if len(*events) != 1 || (*events)[0].Origin != "Loader.ApplyPolicyStages" || (*events)[0].Code != CodeUnknownValue || (*events)[0].Cycle != 0 {
		t.Fatalf("event: %+v", *events)
	}
}

func TestApplyPolicyStages_GateLadderRejectsAdvisoryRouterLadderAcceptsIt(t *testing.T) {
	ps := PolicyStages{"advisory", "enforce", "enforce", "enforce", "off", "shadow", "enforce", "enforce", "advisory", "off", 3, false, false, 1}
	got, ws := New().ApplyPolicyStages(defaults(), ps)
	if got.ContractGate != StageOff || got.RouterReplan != StageAdvisory {
		t.Errorf("ContractGate=%v (want Off) RouterReplan=%v (want Advisory)", got.ContractGate, got.RouterReplan)
	}
	if len(ws) != 1 || ws[0].Fields["key"] != "gates.contract_gate" {
		t.Errorf("exactly the gate warns: %+v", ws)
	}
	var _ signalcenter.Code = CodeUnknownValue
}
