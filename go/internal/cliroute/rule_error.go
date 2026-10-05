package cliroute

import "fmt"

type RuleErrorKind string

const (
	RuleLeak  RuleErrorKind = "leak"
	RuleEmpty RuleErrorKind = "empty"
)

type RuleError struct {
	Rule    string
	Kind    RuleErrorKind
	Dropped []string
	Allowed []string
}

func (e *RuleError) Error() string {
	if e.Kind == RuleLeak {
		return fmt.Sprintf("%s lists %v outside the allowed set %v", e.Rule, e.Dropped, e.Allowed)
	}
	return fmt.Sprintf("rule %s leaves no allowed CLI: %v dropped, allowed %v", e.Rule, e.Dropped, e.Allowed)
}

type CeilingError struct {
	Rule    string
	Tiers   []string
	Chain   []string
	Ceiling map[string][]string
}

func (e *CeilingError) Error() string {
	return fmt.Sprintf("rule %s: the tier ceiling leaves no CLI of %v at tiers %v (ceiling %v)", e.Rule, e.Chain, e.Tiers, e.Ceiling)
}
