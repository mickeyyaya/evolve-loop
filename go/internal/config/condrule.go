package config

import (
	"fmt"
	"strings"
)

// DefaultTddRuleExpr is the compiled tdd conditional. Registry-less projects run on it, so it
// must match the registry's rule (TestDefaults_TddRuleMatchesRegistry).
const DefaultTddRuleExpr = "cycle_size!=trivial && deliverable_kind!=document"

// DefaultTddRule is DefaultTddRuleExpr parsed.
func DefaultTddRule() CondRule { return mustCondRule(DefaultTddRuleExpr) }

// mustCondRule panics on a compiled expression that does not parse: the package's only panic.
func mustCondRule(expr string) CondRule {
	r, err := parseCondRule(expr)
	if err != nil {
		panic("config: DefaultTddRuleExpr does not parse: " + err.Error())
	}
	return r
}

// CondRule is a parsed conditional-mandatory predicate: the head clause, plus And clauses that must all hold.
type CondRule struct {
	Field string
	Op    string
	Value string
	And   []CondRule
}

// Clauses returns the head clause followed by the And clauses.
func (r CondRule) Clauses() []CondRule {
	out := make([]CondRule, 0, 1+len(r.And))
	out = append(out, CondRule{Field: r.Field, Op: r.Op, Value: r.Value})
	return append(out, r.And...)
}

// parseCondRule parses `&&`-joined "field<op>value" clauses. A malformed clause fails the whole
// rule, so a rule never silently shrinks.
func parseCondRule(expr string) (CondRule, error) {
	parts := strings.Split(expr, "&&")
	head, err := parseCondClause(parts[0])
	if err != nil {
		return CondRule{}, err
	}
	for _, p := range parts[1:] {
		c, err := parseCondClause(p)
		if err != nil {
			return CondRule{}, err
		}
		head.And = append(head.And, c)
	}
	return head, nil
}

// parseCondClause matches two-character operators before one-character ones.
func parseCondClause(expr string) (CondRule, error) {
	for _, op := range []string{"!=", "==", ">=", "<="} {
		if i := strings.Index(expr, op); i >= 0 {
			return CondRule{
				Field: strings.TrimSpace(expr[:i]),
				Op:    op,
				Value: strings.TrimSpace(expr[i+2:]),
			}, nil
		}
	}
	for _, op := range []string{">", "<"} {
		if i := strings.Index(expr, op); i >= 0 {
			return CondRule{
				Field: strings.TrimSpace(expr[:i]),
				Op:    op,
				Value: strings.TrimSpace(expr[i+1:]),
			}, nil
		}
	}
	return CondRule{}, fmt.Errorf("no comparison operator in %q", expr)
}
