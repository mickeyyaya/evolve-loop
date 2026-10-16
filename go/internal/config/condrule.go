package config

import (
	"fmt"
	"strings"
)

const DefaultTddRuleExpr = "cycle_size!=trivial && deliverable_kind!=document"

func DefaultTddRule() CondRule { return mustCondRule(DefaultTddRuleExpr) }

func mustCondRule(expr string) CondRule {
	r, err := parseCondRule(expr)
	if err != nil {
		panic("config: DefaultTddRuleExpr does not parse: " + err.Error())
	}
	return r
}

type CondRule struct {
	Field string
	Op    string
	Value string
	And   []CondRule
}

func (r CondRule) Clauses() []CondRule {
	out := make([]CondRule, 0, 1+len(r.And))
	out = append(out, CondRule{Field: r.Field, Op: r.Op, Value: r.Value})
	return append(out, r.And...)
}

func (r CondRule) String() string {
	clauses := r.Clauses()
	parts := make([]string, len(clauses))
	for i, c := range clauses {
		parts[i] = c.Field + c.Op + c.Value
	}
	return strings.Join(parts, " && ")
}

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
