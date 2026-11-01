package policy

import "fmt"

const (
	defaultContinuationMaxAgeCycles = 100
	minContinuationMaxAgeCycles     = 1
	maxContinuationMaxAgeCycles     = 10000
)

type ContinuationPolicy struct {
	MaxAgeCycles *int `json:"max_age_cycles,omitempty"`
}

type ContinuationConfig struct {
	MaxAgeCycles int
}

func (b *ContinuationPolicy) UnmarshalJSON(raw []byte) error {
	type block ContinuationPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("continuation: %w", err)
	}
	if v := decoded.MaxAgeCycles; v != nil && (*v < minContinuationMaxAgeCycles || *v > maxContinuationMaxAgeCycles) {
		return fmt.Errorf("continuation: max_age_cycles must be in [%d, %d], got %d", minContinuationMaxAgeCycles, maxContinuationMaxAgeCycles, *v)
	}
	*b = ContinuationPolicy(decoded)
	return nil
}

func (p Policy) ContinuationConfig() ContinuationConfig {
	c := ContinuationConfig{MaxAgeCycles: defaultContinuationMaxAgeCycles}
	if p.Continuation != nil && p.Continuation.MaxAgeCycles != nil {
		c.MaxAgeCycles = *p.Continuation.MaxAgeCycles
	}
	return c
}
