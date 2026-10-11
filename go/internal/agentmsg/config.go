package agentmsg

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

var ErrConfig = errors.New("agentmsg: config")

var kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type Config struct {
	Rules           []policy.Rule   `json:"rules,omitempty"`
	Kinds           []envelope.Kind `json:"kinds,omitempty"`
	BodyBytes       *int            `json:"body_bytes,omitempty"`
	SubjectBytes    *int            `json:"subject_bytes,omitempty"`
	SenderPerMinute *int            `json:"per_sender_per_min,omitempty"`
	RepeatWindowS   *int            `json:"repeat_window_s,omitempty"`
	RecipientQueue  *int            `json:"recipient_queue,omitempty"`
	MaxHops         *int            `json:"max_hops,omitempty"`
	CycleBudget     *int            `json:"cycle_budget,omitempty"`
	TokenTTLS       *int            `json:"token_ttl_s,omitempty"`
}

type Settings struct {
	Policy   *policy.Engine
	Envelope envelope.Limits
	Limits   broker.Limits
	TokenTTL time.Duration
}

type bound struct {
	key   string
	value *int
	def   int
	floor int
	out   *int
}

func (c *Config) Resolve() (Settings, error) {
	if c == nil {
		c = &Config{}
	}
	var s Settings
	var repeatS, ttlS int
	bounds := []bound{
		{"body_bytes", c.BodyBytes, 8192, 1, &s.Envelope.BodyBytes},
		{"subject_bytes", c.SubjectBytes, 256, 1, &s.Envelope.SubjectBytes},
		{"per_sender_per_min", c.SenderPerMinute, 30, 1, &s.Limits.SenderPerMinute},
		{"repeat_window_s", c.RepeatWindowS, 60, 1, &repeatS},
		{"recipient_queue", c.RecipientQueue, 50, 1, &s.Limits.RecipientQueue},
		{"max_hops", c.MaxHops, 4, 0, &s.Limits.MaxHops},
		{"cycle_budget", c.CycleBudget, 200, 1, &s.Limits.CycleBudget},
		{"token_ttl_s", c.TokenTTLS, 43200, 1, &ttlS},
	}
	for _, b := range bounds {
		if err := b.resolve(); err != nil {
			return Settings{}, err
		}
	}
	s.Limits.RepeatWindow = time.Duration(repeatS) * time.Second
	s.TokenTTL = time.Duration(ttlS) * time.Second
	kinds, err := c.kinds()
	if err != nil {
		return Settings{}, err
	}
	s.Envelope.Kinds = kinds
	s.Policy, err = c.policy()
	return s, err
}

func (b bound) resolve() error {
	*b.out = b.def
	if b.value == nil {
		return nil
	}
	if *b.value < b.floor {
		return fmt.Errorf("%w: messaging.%s must be at least %d, got %d", ErrConfig, b.key, b.floor, *b.value)
	}
	*b.out = *b.value
	return nil
}

func (c *Config) kinds() ([]envelope.Kind, error) {
	kinds := envelope.DefaultKinds()
	for _, k := range c.Kinds {
		if !kindPattern.MatchString(string(k)) || slices.Contains(kinds, k) {
			return nil, fmt.Errorf("%w: messaging.kinds: %q is malformed or given twice", ErrConfig, k)
		}
		kinds = append(kinds, k)
	}
	return kinds, nil
}

func (c *Config) policy() (*policy.Engine, error) {
	rules := c.Rules
	if rules == nil {
		rules = policy.Defaults()
	}
	e, err := policy.Compile(rules)
	if err != nil {
		return nil, fmt.Errorf("%w: messaging.rules: %v", ErrConfig, err)
	}
	return e, nil
}
