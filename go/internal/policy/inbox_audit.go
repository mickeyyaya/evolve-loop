package policy

import "fmt"

const (
	defaultInboxAuditMinAgeDays   = 14
	defaultInboxAuditTriggerCount = 40
	defaultInboxAuditMaxItems     = 20
)

type InboxAuditPolicy struct {
	MinAgeDays   *int `json:"min_age_days,omitempty"`
	TriggerCount *int `json:"trigger_count,omitempty"`
	MaxItems     *int `json:"max_items,omitempty"`
}

type InboxAuditConfig struct {
	MinAgeDays   int
	TriggerCount int
	MaxItems     int
}

func (b *InboxAuditPolicy) UnmarshalJSON(raw []byte) error {
	type block InboxAuditPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	for _, v := range []struct {
		key   string
		value *int
	}{{"min_age_days", decoded.MinAgeDays}, {"trigger_count", decoded.TriggerCount}, {"max_items", decoded.MaxItems}} {
		if v.value != nil && *v.value <= 0 {
			return fmt.Errorf("audit: %s must be a positive count, got %d", v.key, *v.value)
		}
	}
	*b = InboxAuditPolicy(decoded)
	return nil
}

func (p Policy) InboxAuditConfig() InboxAuditConfig {
	c := InboxAuditConfig{MinAgeDays: defaultInboxAuditMinAgeDays, TriggerCount: defaultInboxAuditTriggerCount, MaxItems: defaultInboxAuditMaxItems}
	if p.Inbox == nil || p.Inbox.Curator == nil || p.Inbox.Curator.Audit == nil {
		return c
	}
	b := p.Inbox.Curator.Audit
	c.MinAgeDays = valueOr(b.MinAgeDays, c.MinAgeDays)
	c.TriggerCount = valueOr(b.TriggerCount, c.TriggerCount)
	c.MaxItems = valueOr(b.MaxItems, c.MaxItems)
	return c
}
