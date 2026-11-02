package cyclespend

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

type Verdict struct {
	LLMMinutesToVerdict float64  `json:"llm_minutes_to_verdict"`
	FailPointMinutes    *float64 `json:"fail_point_minutes,omitempty"`
	WasteMinutes        *float64 `json:"waste_minutes,omitempty"`
	FailFastRatio       *float64 `json:"fail_fast_ratio,omitempty"`
}

func VerdictOf(calls []llmcalls.Record, failAt time.Time, shipped bool) Verdict {
	total := Sum(calls).LLMMinutes()
	v := Verdict{LLMMinutesToVerdict: total}
	if shipped || failAt.IsZero() {
		return v
	}
	var after int64
	for _, r := range calls {
		if r.DurationMS != nil && endedAfter(r, failAt) {
			after += *r.DurationMS
		}
	}
	waste := float64(after) / 60_000
	failPoint := total - waste
	v.FailPointMinutes, v.WasteMinutes = &failPoint, &waste
	if total > 0 {
		ratio := failPoint / total
		v.FailFastRatio = &ratio
	}
	return v
}

func endedAfter(r llmcalls.Record, at time.Time) bool {
	for _, stamp := range []string{r.EndedAt, r.TS} {
		if end, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			return end.After(at)
		}
	}
	return false
}
