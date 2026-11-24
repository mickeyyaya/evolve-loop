package policy_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestHealingRepairCycle_DefaultsToShadowAndReadsOff(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{}`:                              policy.HealingStageShadow,
		`{"healing":{"repair_cycle":{}}}`: policy.HealingStageShadow,
		`{"healing":{"repair_cycle":{"stage":"off"}}}`:    policy.HealingStageOff,
		`{"healing":{"repair_cycle":{"stage":"shadow"}}}`: policy.HealingStageShadow,
	}
	for body, want := range cases {
		p, err := policy.Parse([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got := p.HealingRepairCycle(); got != (policy.HealingRepairCyclePolicy{Stage: policy.RequestStage(want)}) {
			t.Errorf("%s: %+v, want stage %q", body, got, want)
		}
	}
}

func TestHealingRepairCycle_EnforceIsRefusedUntilTheLaneRouteShips(t *testing.T) {
	t.Parallel()
	_, err := policy.Parse([]byte(`{"healing":{"repair_cycle":{"stage":"enforce"}}}`))
	if !errors.Is(err, policy.ErrRepairCycleEnforceBlocked) {
		t.Fatalf("err = %v, want ErrRepairCycleEnforceBlocked", err)
	}
}

func TestHealingRepairCycle_RefusesAnUnknownStageAndAnUnknownKey(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"healing":{"repair_cycle":{"stage":"loud"}}}`, `{"healing":{"repair_cycle":{"stgae":"shadow"}}}`} {
		if _, err := policy.Parse([]byte(body)); err == nil || errors.Is(err, policy.ErrRepairCycleEnforceBlocked) {
			t.Errorf("%s: err = %v, want a plain refusal", body, err)
		}
	}
}
