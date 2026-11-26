package policy

import (
	"errors"
	"fmt"
)

const (
	DefaultBlockerWaitSharePct = 50
	minBlockerWaitSharePct     = 20
	maxBlockerWaitSharePct     = 95
)

var ErrOutOfRange = errors.New("policy: value is outside its range")

type BlockerAttributionPolicy struct {
	WaitSharePct *int `json:"wait_share_pct,omitempty"`
}

type BlockerAttributionConfig struct {
	WaitSharePct int
}

func (w *WorkflowPolicy) validateBlockerAttribution() error {
	if w == nil || w.BlockerAttribution == nil || w.BlockerAttribution.WaitSharePct == nil {
		return nil
	}
	if v := *w.BlockerAttribution.WaitSharePct; v < minBlockerWaitSharePct || v > maxBlockerWaitSharePct {
		return fmt.Errorf("%w: workflow.blocker_attribution.wait_share_pct %d is outside %d to %d", ErrOutOfRange, v, minBlockerWaitSharePct, maxBlockerWaitSharePct)
	}
	return nil
}

func (p Policy) blockerAttributionConfig() BlockerAttributionConfig {
	c := BlockerAttributionConfig{WaitSharePct: DefaultBlockerWaitSharePct}
	if p.Workflow != nil && p.Workflow.BlockerAttribution != nil && p.Workflow.BlockerAttribution.WaitSharePct != nil {
		c.WaitSharePct = *p.Workflow.BlockerAttribution.WaitSharePct
	}
	return c
}
