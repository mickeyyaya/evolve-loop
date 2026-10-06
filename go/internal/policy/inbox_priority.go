package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	defaultInboxBaseFactor       = 0.45
	defaultInboxClassFactor      = 0.20
	defaultInboxUnblocksFactor   = 0.15
	defaultInboxRecurrenceFactor = 0.10
	defaultInboxAgeFactor        = 0.05
	defaultInboxGoalFactor       = 0.05
	defaultInboxAgeHalflifeDays  = 30
	defaultInboxUnblocksCap      = 3
	defaultInboxRecurrenceCap    = 5
	defaultInboxPreemptMargin    = 0.05
)

var defaultInboxClassOrder = []string{
	"correctness", "stability", "performance", "debuggability", "feature", "maintainability", "hygiene", "security",
}

var inboxFactorNames = []string{"base", "class", "unblocks", "recurrence", "age", "goal"}

type InboxPriorityPolicy struct {
	ClassOrder      []string              `json:"class_order,omitempty"`
	Factors         *InboxPriorityFactors `json:"factors,omitempty"`
	AgeHalflifeDays *float64              `json:"age_halflife_days,omitempty"`
	UnblocksCap     *int                  `json:"unblocks_cap,omitempty"`
	RecurrenceCap   *int                  `json:"recurrence_cap,omitempty"`
	ActiveCampaigns []string              `json:"active_campaigns,omitempty"`
	PreemptMargin   *float64              `json:"preempt_margin,omitempty"`
}

type InboxPriorityFactors struct {
	Base       float64 `json:"base"`
	Class      float64 `json:"class"`
	Unblocks   float64 `json:"unblocks"`
	Recurrence float64 `json:"recurrence"`
	Age        float64 `json:"age"`
	Goal       float64 `json:"goal"`
}

type InboxPriorityConfig struct {
	ClassOrder      []string
	Factors         InboxPriorityFactors
	AgeHalflifeDays float64
	UnblocksCap     int
	RecurrenceCap   int
	ActiveCampaigns []string
	PreemptMargin   float64
}

func (b *InboxPriorityPolicy) UnmarshalJSON(raw []byte) error {
	type block InboxPriorityPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("inbox_priority: %w", err)
	}
	parsed := InboxPriorityPolicy(decoded)
	if err := parsed.validate(); err != nil {
		return fmt.Errorf("inbox_priority: %w", err)
	}
	*b = parsed
	return nil
}

func (f *InboxPriorityFactors) UnmarshalJSON(raw []byte) error {
	var named map[string]json.RawMessage
	if err := json.Unmarshal(raw, &named); err != nil {
		return fmt.Errorf("factors: %w", err)
	}
	type factors InboxPriorityFactors
	var decoded factors
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("factors: %w", err)
	}
	for _, name := range inboxFactorNames {
		if _, ok := named[name]; !ok {
			return fmt.Errorf("factors: %q is missing; name every one of %s (0 turns a factor off)", name, strings.Join(inboxFactorNames, ", "))
		}
	}
	*f = InboxPriorityFactors(decoded)
	return nil
}

func (b InboxPriorityPolicy) validate() error {
	if b.ClassOrder != nil && len(b.ClassOrder) == 0 {
		return errors.New("class_order is empty; every class would rank alike")
	}
	if err := checkDistinctNames("class_order", b.ClassOrder); err != nil {
		return err
	}
	if err := checkDistinctNames("active_campaigns", b.ActiveCampaigns); err != nil {
		return err
	}
	if b.Factors != nil {
		if err := b.Factors.validate(); err != nil {
			return err
		}
	}
	return b.validateScalars()
}

func (b InboxPriorityPolicy) validateScalars() error {
	switch {
	case b.AgeHalflifeDays != nil && *b.AgeHalflifeDays <= 0:
		return fmt.Errorf("age_halflife_days must be above 0, got %v", *b.AgeHalflifeDays)
	case b.UnblocksCap != nil && *b.UnblocksCap < 1:
		return fmt.Errorf("unblocks_cap must be at least 1, got %d", *b.UnblocksCap)
	case b.RecurrenceCap != nil && *b.RecurrenceCap < 1:
		return fmt.Errorf("recurrence_cap must be at least 1, got %d", *b.RecurrenceCap)
	case b.PreemptMargin != nil && *b.PreemptMargin < 0:
		return fmt.Errorf("preempt_margin must not be negative, got %v", *b.PreemptMargin)
	}
	return nil
}

func checkDistinctNames(field string, names []string) error {
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
			return fmt.Errorf("%s: %q is blank or padded", field, name)
		}
		if seen[name] {
			return fmt.Errorf("%s: %q is listed twice", field, name)
		}
		seen[name] = true
	}
	return nil
}

func (f InboxPriorityFactors) validate() error {
	total := 0.0
	for i, weight := range f.weights() {
		if weight < 0 {
			return fmt.Errorf("factors: %q must not be negative, got %v", inboxFactorNames[i], weight)
		}
		total += weight
	}
	if total == 0 {
		return errors.New("factors: every factor is 0, so nothing would rank")
	}
	return nil
}

func (f InboxPriorityFactors) weights() []float64 {
	return []float64{f.Base, f.Class, f.Unblocks, f.Recurrence, f.Age, f.Goal}
}

func defaultInboxPriority() InboxPriorityConfig {
	return InboxPriorityConfig{
		ClassOrder: slices.Clone(defaultInboxClassOrder),
		Factors: InboxPriorityFactors{
			Base: defaultInboxBaseFactor, Class: defaultInboxClassFactor, Unblocks: defaultInboxUnblocksFactor,
			Recurrence: defaultInboxRecurrenceFactor, Age: defaultInboxAgeFactor, Goal: defaultInboxGoalFactor,
		},
		AgeHalflifeDays: defaultInboxAgeHalflifeDays,
		UnblocksCap:     defaultInboxUnblocksCap,
		RecurrenceCap:   defaultInboxRecurrenceCap,
		PreemptMargin:   defaultInboxPreemptMargin,
	}
}

func (p Policy) InboxPriorityConfig() InboxPriorityConfig {
	c := defaultInboxPriority()
	b := p.InboxPriority
	if b == nil {
		return c
	}
	if b.ClassOrder != nil {
		c.ClassOrder = slices.Clone(b.ClassOrder)
	}
	if b.Factors != nil {
		c.Factors = *b.Factors
	}
	c.ActiveCampaigns = slices.Clone(b.ActiveCampaigns)
	c.AgeHalflifeDays = valueOr(b.AgeHalflifeDays, c.AgeHalflifeDays)
	c.UnblocksCap = valueOr(b.UnblocksCap, c.UnblocksCap)
	c.RecurrenceCap = valueOr(b.RecurrenceCap, c.RecurrenceCap)
	c.PreemptMargin = valueOr(b.PreemptMargin, c.PreemptMargin)
	return c
}

func valueOr[T any](named *T, fallback T) T {
	if named == nil {
		return fallback
	}
	return *named
}
