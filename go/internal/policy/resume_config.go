package policy

import (
	"fmt"
	"time"
)

type ResumeConfig struct {
	MaxCheckpointAgeH int `json:"max_checkpoint_age_h,omitempty"`
}

const defaultMaxCheckpointAge = 192 * time.Hour

func (c *ResumeConfig) UnmarshalJSON(raw []byte) error {
	var decoded struct {
		MaxCheckpointAgeH *int `json:"max_checkpoint_age_h"`
	}
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	if decoded.MaxCheckpointAgeH == nil {
		*c = ResumeConfig{}
		return nil
	}
	if *decoded.MaxCheckpointAgeH <= 0 {
		return fmt.Errorf("resume: max_checkpoint_age_h must be positive, got %d; remove the key for the default of %s", *decoded.MaxCheckpointAgeH, defaultMaxCheckpointAge)
	}
	*c = ResumeConfig{MaxCheckpointAgeH: *decoded.MaxCheckpointAgeH}
	return nil
}

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
