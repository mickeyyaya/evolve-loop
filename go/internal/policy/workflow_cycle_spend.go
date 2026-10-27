package policy

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
)

var DefaultCycleSpendLimits = cyclespend.Limits{MaxLLMMinutes: 90, MaxFreshTokens: 2_500_000, MaxCacheReadTokens: 40_000_000, MaxAuditRounds: 2}

type CycleSpendPolicy struct {
	Stage              string `json:"stage,omitempty"`
	MaxLLMMinutes      *int64 `json:"max_llm_minutes,omitempty"`
	MaxFreshTokens     *int64 `json:"max_fresh_tokens,omitempty"`
	MaxCacheReadTokens *int64 `json:"max_cache_read_tokens,omitempty"`
	MaxAuditRounds     *int64 `json:"max_audit_rounds,omitempty"`
}

type CycleSpendConfig struct {
	Stage  config.Stage
	Limits cyclespend.Limits
}

type spendKey struct {
	name     string
	value    *int64
	min, max int64
}

func (b *CycleSpendPolicy) keys() []spendKey {
	return []spendKey{
		{cyclespend.LimitLLMMinutes, b.MaxLLMMinutes, 10, 600},
		{cyclespend.LimitFreshTokens, b.MaxFreshTokens, 100_000, 20_000_000},
		{cyclespend.LimitCacheReadTokens, b.MaxCacheReadTokens, 1_000_000, 500_000_000},
		{cyclespend.LimitAuditRounds, b.MaxAuditRounds, 1, 4},
	}
}

func (w *WorkflowPolicy) validateCycleSpend() error {
	if w == nil || w.CycleSpend == nil {
		return nil
	}
	b := w.CycleSpend
	if _, ok := config.GateStage(b.Stage); b.Stage != "" && !ok {
		return fmt.Errorf("workflow.cycle_spend.stage: %q is not 0, off, shadow or enforce", b.Stage)
	}
	for _, k := range b.keys() {
		if k.value != nil && (*k.value < k.min || *k.value > k.max) {
			return fmt.Errorf("workflow.cycle_spend.%s %d is outside %d to %d", k.name, *k.value, k.min, k.max)
		}
	}
	return nil
}

func (p Policy) cycleSpendConfig() CycleSpendConfig {
	c := CycleSpendConfig{Stage: config.StageShadow, Limits: DefaultCycleSpendLimits}
	if p.Workflow == nil || p.Workflow.CycleSpend == nil {
		return c
	}
	b := p.Workflow.CycleSpend
	if set, ok := config.GateStage(b.Stage); ok {
		c.Stage = set
	}
	c.Limits = cyclespend.Limits{
		MaxLLMMinutes:      valueOr(b.MaxLLMMinutes, c.Limits.MaxLLMMinutes),
		MaxFreshTokens:     valueOr(b.MaxFreshTokens, c.Limits.MaxFreshTokens),
		MaxCacheReadTokens: valueOr(b.MaxCacheReadTokens, c.Limits.MaxCacheReadTokens),
		MaxAuditRounds:     valueOr(b.MaxAuditRounds, c.Limits.MaxAuditRounds),
	}
	return c
}
