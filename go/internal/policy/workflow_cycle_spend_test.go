package policy

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

func TestParse_TheCycleSpendDefaultsAreTheADRValuesAtShadow(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"workflow": {}}`, `{"workflow": {"cycle_spend": {}}}`} {
		p, err := Parse([]byte(body))
		c := p.WorkflowConfig().CycleSpend
		if err != nil || c.Stage != config.StageShadow || c.Limits != (cyclespend.Limits{MaxLLMMinutes: 90, MaxFreshTokens: 2_500_000, MaxCacheReadTokens: 40_000_000, MaxAuditRounds: 2}) {
			t.Errorf("%s: config = %+v, %v", body, c, err)
		}
	}
}

func TestParse_TheCycleSpendKeysSetTheLimitsAndTheStage(t *testing.T) {
	t.Parallel()
	p, err := Parse([]byte(`{"workflow": {"cycle_spend": {"stage": "enforce", "max_llm_minutes": 600, "max_fresh_tokens": 100000, "max_cache_read_tokens": 500000000, "max_audit_rounds": 1}}}`))
	c := p.WorkflowConfig().CycleSpend
	if err != nil || c.Stage != config.StageEnforce || c.Limits != (cyclespend.Limits{MaxLLMMinutes: 600, MaxFreshTokens: 100_000, MaxCacheReadTokens: 500_000_000, MaxAuditRounds: 1}) {
		t.Errorf("config = %+v, %v", c, err)
	}
}

func TestParse_EachCycleSpendKeyRefusesAValueOutsideItsRange(t *testing.T) {
	t.Parallel()
	for key, values := range map[string][2]string{
		"max_llm_minutes": {"9", "601"}, "max_fresh_tokens": {"99999", "20000001"},
		"max_cache_read_tokens": {"999999", "500000001"}, "max_audit_rounds": {"0", "5"},
	} {
		for _, v := range values {
			_, err := Parse([]byte(`{"workflow": {"cycle_spend": {"` + key + `": ` + v + `}}}`))
			if err == nil || !strings.Contains(err.Error(), "workflow.cycle_spend."+key) {
				t.Errorf("%s = %s: err = %v, want a refusal that names the key", key, v, err)
			}
		}
	}
	if _, err := Parse([]byte(`{"workflow": {"cycle_spend": {"stage": "advisory"}}}`)); err == nil || !strings.Contains(err.Error(), "workflow.cycle_spend.stage") {
		t.Errorf("stage advisory: err = %v", err)
	}
}

func TestCycleSpendConfig_ABuiltPolicyWithAnUnknownStageHoldsShadow(t *testing.T) {
	t.Parallel()
	p := Policy{Workflow: &WorkflowPolicy{CycleSpend: &CycleSpendPolicy{Stage: "advisory"}}}
	if got := p.cycleSpendConfig().Stage; got != config.StageShadow {
		t.Errorf("stage = %v, want shadow", got)
	}
}
