package agentmsg

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

func resolve(t *testing.T, raw string) Settings {
	t.Helper()
	var c *Config
	if raw != "" {
		c = &Config{}
		if err := json.Unmarshal([]byte(raw), c); err != nil {
			t.Fatalf("Unmarshal(%s) error = %v", raw, err)
		}
	}
	s, err := c.Resolve()
	if err != nil {
		t.Fatalf("Resolve(%s) error = %v", raw, err)
	}
	return s
}

func TestResolve_AnAbsentBlockGivesTheCompiledDefaults(t *testing.T) {
	for _, raw := range []string{"", "{}"} {
		s := resolve(t, raw)
		wantLimits := broker.Limits{SenderPerMinute: 30, RepeatWindow: time.Minute, RecipientQueue: 50, MaxHops: 4, CycleBudget: 200}
		if s.TokenTTL != 12*time.Hour {
			t.Fatalf("TokenTTL = %s, want 12h", s.TokenTTL)
		}
		if s.Limits != wantLimits {
			t.Fatalf("Limits = %+v, want %+v", s.Limits, wantLimits)
		}
		if s.Envelope.BodyBytes != 8192 || s.Envelope.SubjectBytes != 256 || !slices.Equal(s.Envelope.Kinds, envelope.DefaultKinds()) {
			t.Fatalf("Envelope = %+v, want 8192, 256 and the default kinds", s.Envelope)
		}
		build := identity.Principal{Cycle: 3, Phase: "build"}
		audit := identity.Principal{Cycle: 3, Phase: "audit"}
		if d := s.Policy.Authorize(policy.Request{Sender: build, Recipient: audit, Kind: envelope.KindNote}); d.Allow || d.Rule != "audit_independence" {
			t.Fatalf("the default policy decided %+v for build → audit, want audit_independence", d)
		}
	}
}

func TestResolve_EachKeyOverridesItsDefault(t *testing.T) {
	s := resolve(t, `{"body_bytes":100,"subject_bytes":10,"per_sender_per_min":5,"repeat_window_s":7,"recipient_queue":3,"max_hops":0,"cycle_budget":9,"token_ttl_s":90,"kinds":["question"]}`)
	if s.TokenTTL != 90*time.Second {
		t.Fatalf("TokenTTL = %s, want 90s", s.TokenTTL)
	}
	want := broker.Limits{SenderPerMinute: 5, RepeatWindow: 7 * time.Second, RecipientQueue: 3, MaxHops: 0, CycleBudget: 9}
	if s.Limits != want || s.Envelope.BodyBytes != 100 || s.Envelope.SubjectBytes != 10 {
		t.Fatalf("Settings = %+v, want %+v with body 100 and subject 10", s, want)
	}
	if !slices.Equal(s.Envelope.Kinds, append(envelope.DefaultKinds(), "question")) {
		t.Fatalf("Kinds = %v, want the defaults and question", s.Envelope.Kinds)
	}
}

func TestResolve_ConfiguredRulesReplaceTheDefaults(t *testing.T) {
	scout := identity.Principal{Cycle: 3, Phase: "scout"}
	build := identity.Principal{Cycle: 3, Phase: "build"}
	open := resolve(t, `{"rules":[{"name":"all","effect":"allow"}]}`)
	if d := open.Policy.Authorize(policy.Request{Sender: build, Recipient: identity.Principal{Cycle: 3, Phase: "audit"}}); !d.Allow {
		t.Fatalf("a configured allow-all decided %+v, want allow", d)
	}
	closed := resolve(t, `{"rules":[]}`)
	if d := closed.Policy.Authorize(policy.Request{Sender: scout, Recipient: build}); d.Allow {
		t.Fatalf("an empty rule list decided %+v, want deny: an operator can close messaging", d)
	}
}

func TestResolve_RefusesAValueOutOfRange(t *testing.T) {
	cases := map[string]string{
		"body_bytes":         `{"body_bytes":0}`,
		"subject_bytes":      `{"subject_bytes":-1}`,
		"per_sender_per_min": `{"per_sender_per_min":0}`,
		"repeat_window_s":    `{"repeat_window_s":0}`,
		"recipient_queue":    `{"recipient_queue":0}`,
		"max_hops":           `{"max_hops":-1}`,
		"cycle_budget":       `{"cycle_budget":0}`,
		"token_ttl_s":        `{"token_ttl_s":0}`,
		"a malformed kind":   `{"kinds":["Question"]}`,
		"a repeated kind":    `{"kinds":["note"]}`,
		"a broken rule":      `{"rules":[{"name":"x","effect":"maybe"}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			c := &Config{}
			if err := json.Unmarshal([]byte(raw), c); err != nil {
				t.Fatal(err)
			}
			if _, err := c.Resolve(); !errors.Is(err, ErrConfig) {
				t.Fatalf("Resolve(%s) error = %v, want ErrConfig", raw, err)
			}
		})
	}
}
