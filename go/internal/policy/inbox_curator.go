package policy

import (
	"errors"
	"fmt"
	"slices"
)

const (
	inboxCuratorShadow          = "shadow"
	inboxCuratorEnforce         = "enforce"
	defaultInboxOverlapRefuse   = 0.5
	defaultInboxOverlapWarn     = 0.3
	defaultInboxConsumeMinShare = 0.5
)

var defaultInboxKinds = []string{
	"bug", "docs", "feature", "fix", "hardening", "maintenance", "pipeline-integrity", "pipeline-repair",
	"refactor", "research", "security", "sweep", "task", "techdebt", "test",
}

type InboxPolicy struct {
	Curator *InboxCuratorPolicy `json:"curator,omitempty"`
}

type InboxCuratorPolicy struct {
	Stage           string   `json:"stage,omitempty"`
	OverlapRefuse   *float64 `json:"overlap_refuse,omitempty"`
	OverlapWarn     *float64 `json:"overlap_warn,omitempty"`
	ConsumeMinShare *float64 `json:"consume_min_share,omitempty"`
	Kinds           []string `json:"kinds,omitempty"`
}

type InboxCuratorConfig struct {
	Stage           string
	OverlapRefuse   float64
	OverlapWarn     float64
	ConsumeMinShare float64
	Kinds           []string
}

func (c InboxCuratorConfig) Enforced() bool { return c.Stage == inboxCuratorEnforce }

func (b *InboxCuratorPolicy) UnmarshalJSON(raw []byte) error {
	type block InboxCuratorPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("inbox.curator: %w", err)
	}
	parsed := InboxCuratorPolicy(decoded)
	if err := parsed.validate(); err != nil {
		return fmt.Errorf("inbox.curator: %w", err)
	}
	*b = parsed
	return nil
}

func (b InboxCuratorPolicy) validate() error {
	refuse := valueOr(b.OverlapRefuse, defaultInboxOverlapRefuse)
	warn := valueOr(b.OverlapWarn, min(defaultInboxOverlapWarn, refuse))
	switch {
	case b.Stage != "" && b.Stage != inboxCuratorShadow && b.Stage != inboxCuratorEnforce:
		return fmt.Errorf("stage %q is not %q or %q", b.Stage, inboxCuratorShadow, inboxCuratorEnforce)
	case refuse <= 0 || refuse > 1:
		return fmt.Errorf("overlap_refuse must be in (0, 1], got %v", refuse)
	case warn < 0 || warn > refuse:
		return fmt.Errorf("overlap_warn must be in [0, overlap_refuse %v], got %v", refuse, warn)
	case b.ConsumeMinShare != nil && (*b.ConsumeMinShare <= 0 || *b.ConsumeMinShare > 1):
		return fmt.Errorf("consume_min_share must be in (0, 1], got %v", *b.ConsumeMinShare)
	case b.Kinds != nil && len(b.Kinds) == 0:
		return errors.New("kinds is empty; every new item would be refused")
	}
	return checkDistinctNames("kinds", b.Kinds)
}

func (p Policy) InboxCuratorConfig() InboxCuratorConfig {
	c := InboxCuratorConfig{
		Stage: inboxCuratorShadow, OverlapRefuse: defaultInboxOverlapRefuse, OverlapWarn: defaultInboxOverlapWarn,
		ConsumeMinShare: defaultInboxConsumeMinShare, Kinds: slices.Clone(defaultInboxKinds),
	}
	if p.Inbox == nil || p.Inbox.Curator == nil {
		return c
	}
	b := p.Inbox.Curator
	if b.Stage != "" {
		c.Stage = b.Stage
	}
	if b.Kinds != nil {
		c.Kinds = slices.Clone(b.Kinds)
	}
	c.OverlapRefuse = valueOr(b.OverlapRefuse, c.OverlapRefuse)
	c.OverlapWarn = valueOr(b.OverlapWarn, min(c.OverlapWarn, c.OverlapRefuse))
	c.ConsumeMinShare = valueOr(b.ConsumeMinShare, c.ConsumeMinShare)
	return c
}
