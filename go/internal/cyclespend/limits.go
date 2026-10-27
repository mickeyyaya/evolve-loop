package cyclespend

import "strconv"

const (
	SignalCode    = "CYCLE_SPEND_LIMIT"
	FieldDecision = "decision"

	ActionNone      = "none"
	ActionWouldStop = "would_stop"
	ActionStop      = "stop"
	ActionOver      = "over"

	LimitLLMMinutes      = "max_llm_minutes"
	LimitFreshTokens     = "max_fresh_tokens"
	LimitCacheReadTokens = "max_cache_read_tokens"
	LimitAuditRounds     = "max_audit_rounds"
)

type Limits struct {
	MaxLLMMinutes      int64
	MaxFreshTokens     int64
	MaxCacheReadTokens int64
	MaxAuditRounds     int64
}

type Decision struct {
	Action string
	Limit  string
	Value  string
	Max    string
}

func Judge(s Spend, l Limits, onOver string) Decision {
	limit, value, maxValue, over := l.firstOver(s)
	if !over {
		return Decision{Action: ActionNone}
	}
	return Decision{Action: onOver, Limit: limit, Value: value, Max: maxValue}
}

func (l Limits) firstOver(s Spend) (limit, value, maxValue string, over bool) {
	switch {
	case s.LLMMinutes() > float64(l.MaxLLMMinutes):
		return LimitLLMMinutes, strconv.FormatFloat(s.LLMMinutes(), 'f', 1, 64), strconv.FormatInt(l.MaxLLMMinutes, 10), true
	case s.FreshTokens > l.MaxFreshTokens:
		return LimitFreshTokens, strconv.FormatInt(s.FreshTokens, 10), strconv.FormatInt(l.MaxFreshTokens, 10), true
	case s.CacheReadTokens > l.MaxCacheReadTokens:
		return LimitCacheReadTokens, strconv.FormatInt(s.CacheReadTokens, 10), strconv.FormatInt(l.MaxCacheReadTokens, 10), true
	case int64(s.AuditRounds) > l.MaxAuditRounds:
		return LimitAuditRounds, strconv.Itoa(s.AuditRounds), strconv.FormatInt(l.MaxAuditRounds, 10), true
	}
	return "", "", "", false
}

func (d Decision) Summary() string { return d.Limit + " " + d.Value + " of " + d.Max }
