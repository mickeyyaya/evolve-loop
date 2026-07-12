package policy

import "fmt"

// CIWatchPolicy configures the post-push GitHub CI watch and the release
// preflight CI hard-gate (cycle-748, push-ci-watch-remote-parity). All knobs
// live in the policy.json `ci_watch` block — zero env flags by design.
// Pointer fields preserve the distinction between an omitted value and an
// explicit zero/false override.
type CIWatchPolicy struct {
	// Enabled turns the post-push CI watch on/off. Compiled default: true
	// (gates default ON as compiled Go defaults, observer/fleet pattern).
	Enabled *bool `json:"enabled,omitempty"`
	// TimeoutS bounds how long a watch waits for the pushed SHA's CI run to
	// complete. Compiled default: 900.
	TimeoutS *int `json:"timeout_s,omitempty"`
	// PollS is the poll interval while the run is queued/in progress.
	// Compiled default: 30.
	PollS *int `json:"poll_s,omitempty"`
}

// CIWatchConfig returns the ci_watch knobs with compiled defaults resolved;
// returned pointer fields are always non-nil. An absent block yields the
// compiled defaults (enabled, 900s timeout, 30s poll). Malformed values
// (non-positive timeout or poll interval) are rejected explicitly — never
// silently zeroed or clamped.
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
