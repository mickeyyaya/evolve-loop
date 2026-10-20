package policy

import "time"

type ResumeConfig struct {
	MaxCheckpointAgeH int `json:"max_checkpoint_age_h,omitempty"`
}

const defaultMaxCheckpointAge = 192 * time.Hour

func (p Policy) ResumeConfig() ResumeConfig {
	if p.Resume == nil {
		return ResumeConfig{}
	}
	return *p.Resume
}

func (c ResumeConfig) MaxCheckpointAge() time.Duration {
	if c.MaxCheckpointAgeH <= 0 {
		return defaultMaxCheckpointAge
	}
	return time.Duration(c.MaxCheckpointAgeH) * time.Hour
}
