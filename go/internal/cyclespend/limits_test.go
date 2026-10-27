package cyclespend

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

var defaultLimits = Limits{MaxLLMMinutes: 90, MaxFreshTokens: 2_500_000, MaxCacheReadTokens: 40_000_000, MaxAuditRounds: 2}

func TestJudge_Cycle2035PassesItsLimitOfLLMMinutesAfterAuditRoundTwo(t *testing.T) {
	t.Parallel()
	res, err := llmcalls.Read(filepath.Join("testdata", "cycle-2035", llmcalls.Filename))
	if err != nil {
		t.Fatal(err)
	}
	limits := Limits{MaxLLMMinutes: 90, MaxFreshTokens: 1 << 40, MaxCacheReadTokens: 1 << 40, MaxAuditRounds: 99}
	firstStop, audits := -1, 0
	for i := range res.Records {
		if res.Records[i].Phase == "audit" {
			audits++
		}
		d := Judge(Sum(res.Records[:i+1]), limits, ActionStop)
		if d.Action != ActionNone && firstStop < 0 {
			firstStop = i
			if d.Action != ActionStop || d.Limit != LimitLLMMinutes || audits != 2 {
				t.Errorf("first stop after call %d = %+v with %d audits, want a stop on llm_minutes after audit round 2", i, d, audits)
			}
		}
	}
	if firstStop < 0 {
		t.Fatal("the 160.1 LLM minutes of cycle 2035 never passed the limit of 90")
	}
}

func TestJudge_EachLimitStopsAloneAndNamesItsValue(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		spend Spend
		limit string
		value string
	}{
		"minutes":    {Spend{LLMMillis: 91 * 60_000}, LimitLLMMinutes, "91.0"},
		"fresh":      {Spend{FreshTokens: 2_500_001}, LimitFreshTokens, "2500001"},
		"cache read": {Spend{CacheReadTokens: 40_000_001}, LimitCacheReadTokens, "40000001"},
		"audits":     {Spend{AuditRounds: 3}, LimitAuditRounds, "3"},
	} {
		d := Judge(tc.spend, defaultLimits, ActionStop)
		if d.Action != ActionStop || d.Limit != tc.limit || d.Value != tc.value {
			t.Errorf("%s: decision = %+v, want a stop on %s at %s", name, d, tc.limit, tc.value)
		}
	}
}

func TestJudge_ShadowWouldStopAndAtTheLimitNothingStops(t *testing.T) {
	t.Parallel()
	if d := Judge(Spend{AuditRounds: 3}, defaultLimits, ActionWouldStop); d.Action != ActionWouldStop {
		t.Errorf("shadow = %+v, want would_stop", d)
	}
	atLimit := Spend{LLMMillis: 90 * 60_000, FreshTokens: 2_500_000, CacheReadTokens: 40_000_000, AuditRounds: 2}
	if d := Judge(atLimit, defaultLimits, ActionStop); d.Action != ActionNone || d.Limit != "" {
		t.Errorf("at the limit = %+v, want none", d)
	}
}

func TestDecision_SummaryNamesTheLimitTheValueAndTheMax(t *testing.T) {
	t.Parallel()
	d := Judge(Spend{AuditRounds: 3}, defaultLimits, ActionStop)
	if got := d.Summary(); got != "max_audit_rounds 3 of 2" {
		t.Errorf("Summary = %q", got)
	}
	all := Spend{LLMMillis: 91 * 60_000, FreshTokens: 2_500_001, CacheReadTokens: 40_000_001, AuditRounds: 3}
	if d := Judge(all, defaultLimits, ActionStop); d.Limit != LimitLLMMinutes {
		t.Errorf("Judge(all over) = %+v, want %s first", d, LimitLLMMinutes)
	}
	all.LLMMillis = 0
	if d := Judge(all, defaultLimits, ActionStop); d.Limit != LimitFreshTokens {
		t.Errorf("Judge(tokens over) = %+v, want %s before the cache reads", d, LimitFreshTokens)
	}
	all.FreshTokens = 0
	if d := Judge(all, defaultLimits, ActionStop); d.Limit != LimitCacheReadTokens {
		t.Errorf("Judge(cache reads over) = %+v, want %s before the audit rounds", d, LimitCacheReadTokens)
	}
}
