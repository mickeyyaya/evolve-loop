package policy

import "fmt"

type CIWatchPolicy struct {
	Enabled  *bool `json:"enabled,omitempty"`
	TimeoutS *int  `json:"timeout_s,omitempty"`
	PollS    *int  `json:"poll_s,omitempty"`
}

func (p Policy) CIWatchConfig() (CIWatchPolicy, error) {
	enabled, timeoutS, pollS := true, 900, 30
	out := CIWatchPolicy{Enabled: &enabled, TimeoutS: &timeoutS, PollS: &pollS}
	c := p.CIWatch
	if c == nil {
		return out, nil
	}
	if c.Enabled != nil {
		out.Enabled = c.Enabled
	}
	if c.TimeoutS != nil {
		if *c.TimeoutS <= 0 {
			return CIWatchPolicy{}, fmt.Errorf("policy: ci_watch.timeout_s must be > 0, got %d", *c.TimeoutS)
		}
		out.TimeoutS = c.TimeoutS
	}
	if c.PollS != nil {
		if *c.PollS <= 0 {
			return CIWatchPolicy{}, fmt.Errorf("policy: ci_watch.poll_s must be > 0, got %d", *c.PollS)
		}
		out.PollS = c.PollS
	}
	return out, nil
}
