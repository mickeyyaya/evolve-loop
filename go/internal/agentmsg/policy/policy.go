package policy

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
)

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

type Scope string

const (
	ScopeAny        Scope = ""
	ScopeSameCycle  Scope = "same_cycle"
	ScopeCrossCycle Scope = "cross_cycle"
)

const DefaultRule = "default"

var ErrRule = errors.New("policy: rule")

type Match struct {
	Scope      Scope           `json:"scope,omitempty"`
	FromPhase  []string        `json:"from_phase,omitempty"`
	FromRole   []string        `json:"from_role,omitempty"`
	ToPhase    []string        `json:"to_phase,omitempty"`
	ToRole     []string        `json:"to_role,omitempty"`
	Kind       []envelope.Kind `json:"kind,omitempty"`
	ExceptKind []envelope.Kind `json:"except_kind,omitempty"`
	ReplyGrant bool            `json:"reply_grant,omitempty"`
}

type Rule struct {
	Name   string `json:"name"`
	Effect Effect `json:"effect"`
	When   Match  `json:"when"`
}

type Request struct {
	Sender     identity.Principal
	Recipient  identity.Principal
	Kind       envelope.Kind
	ReplyGrant bool
}

type Decision struct {
	Allow  bool
	Rule   string
	Reason string
}

type Authorizer interface {
	Authorize(r Request) Decision
}

type Engine struct {
	rules []Rule
}

func Defaults() []Rule {
	return []Rule{
		{Name: "audit_independence", Effect: EffectDeny, When: Match{
			FromPhase:  []string{"build", "tdd"},
			ToPhase:    []string{"audit"},
			ExceptKind: []envelope.Kind{envelope.KindEvidence},
		}},
		{Name: "reply_right", Effect: EffectAllow, When: Match{ReplyGrant: true}},
		{Name: "same_cycle", Effect: EffectAllow, When: Match{Scope: ScopeSameCycle}},
	}
}

func Compile(rules []Rule) (*Engine, error) {
	seen := make(map[string]bool, len(rules))
	compiled := make([]Rule, 0, len(rules))
	for i, r := range rules {
		if err := r.check(seen); err != nil {
			return nil, fmt.Errorf("%w %d: %v", ErrRule, i, err)
		}
		seen[r.Name] = true
		compiled = append(compiled, r.clone())
	}
	return &Engine{rules: compiled}, nil
}

func (r Rule) check(seen map[string]bool) error {
	switch {
	case r.Name == "" || r.Name == DefaultRule:
		return fmt.Errorf("the name %q is empty or reserved", r.Name)
	case seen[r.Name]:
		return fmt.Errorf("the name %q is given twice", r.Name)
	case r.Effect != EffectAllow && r.Effect != EffectDeny:
		return fmt.Errorf("rule %q: effect %q is not allow or deny", r.Name, r.Effect)
	case r.When.Scope != ScopeAny && r.When.Scope != ScopeSameCycle && r.When.Scope != ScopeCrossCycle:
		return fmt.Errorf("rule %q: scope %q is not same_cycle or cross_cycle", r.Name, r.When.Scope)
	}
	return nil
}

func (r Rule) clone() Rule {
	r.When.FromPhase = slices.Clone(r.When.FromPhase)
	r.When.FromRole = slices.Clone(r.When.FromRole)
	r.When.ToPhase = slices.Clone(r.When.ToPhase)
	r.When.ToRole = slices.Clone(r.When.ToRole)
	r.When.Kind = slices.Clone(r.When.Kind)
	r.When.ExceptKind = slices.Clone(r.When.ExceptKind)
	return r
}

func (e *Engine) Authorize(r Request) Decision {
	if deny, found := e.first(EffectDeny, r); found {
		return Decision{Rule: deny.Name, Reason: "the rule " + deny.Name + " denies it"}
	}
	if allow, found := e.first(EffectAllow, r); found {
		return Decision{Allow: true, Rule: allow.Name, Reason: "the rule " + allow.Name + " allows it"}
	}
	return Decision{Rule: DefaultRule, Reason: "no rule allows it"}
}

func (e *Engine) first(effect Effect, r Request) (Rule, bool) {
	for _, rule := range e.rules {
		if rule.Effect == effect && rule.When.matches(r) {
			return rule, true
		}
	}
	return Rule{}, false
}

func (m Match) matches(r Request) bool {
	return m.scopeHolds(r) &&
		anyOf(m.FromPhase, r.Sender.Phase) &&
		anyOf(m.FromRole, r.Sender.Role) &&
		anyOf(m.ToPhase, r.Recipient.Phase) &&
		anyOf(m.ToRole, r.Recipient.Role) &&
		anyOf(m.Kind, r.Kind) &&
		!slices.Contains(m.ExceptKind, r.Kind) &&
		(!m.ReplyGrant || r.ReplyGrant)
}

func (m Match) scopeHolds(r Request) bool {
	same := r.Sender.Cycle == r.Recipient.Cycle
	switch m.Scope {
	case ScopeSameCycle:
		return same
	case ScopeCrossCycle:
		return !same
	}
	return true
}

func anyOf[T comparable](allowed []T, value T) bool {
	return len(allowed) == 0 || slices.Contains(allowed, value)
}
