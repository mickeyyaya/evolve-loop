package lanetime

import "time"

const (
	unknownCause = "unknown"
)

var idleCodes = map[string]bool{"LOOP_ORCH_POOL_IDLE": true, "LOOP_ORCH_POOL_UNDERFILLED": true}

func IdleUse(samples []Sample, from, to time.Time) map[string]float64 {
	out := map[string]float64{}
	in := within(samples, from, to)
	if len(in) == 0 {
		return out
	}
	end := windowEnd(in, to)
	for i, s := range in {
		free, hours := s.Target-s.Live, holdHours(in, i, end)
		if idleCodes[s.Code] && free > 0 && hours > 0 {
			out[s.Cause] += float64(free) * hours
		}
	}
	return out
}
