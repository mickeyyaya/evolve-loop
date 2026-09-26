package policy

import "fmt"

// CIWatchPolicy is the "ci_watch" block for the post-push CI watch and the release preflight CI gate.
type CIWatchPolicy struct {
	// Enabled defaults to true.
	Enabled *bool `json:"enabled,omitempty"`
	// TimeoutS bounds the wait for the pushed SHA's CI run; default 900.
	TimeoutS *int `json:"timeout_s,omitempty"`
	// PollS is the poll interval while the run is queued or in progress; default 30.
	PollS *int `json:"poll_s,omitempty"`
}

// CIWatchConfig resolves ci_watch with non-nil pointers and rejects a non-positive timeout or poll interval.
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
