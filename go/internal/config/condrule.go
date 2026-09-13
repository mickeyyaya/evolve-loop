package config

// condrule.go — the conditional-mandatory rule grammar: the compiled tdd
// default, the CondRule shape and its parser. Pure; the package's only panic
// (a compiled expression that does not parse) lives in mustCondRule so it is
// reachable from a test.

import (
	"fmt"
	"strings"
)

// DefaultTddRuleExpr is the ONE expression of the compiled tdd conditional —
// the same words docs/architecture/phase-registry.json:config.conditional_mandatory
// carries (TestDefaults_TddRuleMatchesRegistry pins them equal). tdd is the
// code-specific phase: pinned for every non-trivial CODE cycle, released for a
// trivial cycle or a document deliverable (ADR-0099). Registry-less projects
// (no docs/architecture/phase-registry.json, or EVOLVE_USE_PHASE_REGISTRY=0)
// run on this default, so it must never lag the registry.
const DefaultTddRuleExpr = "cycle_size!=trivial && deliverable_kind!=document"

// DefaultTddRule is DefaultTddRuleExpr parsed — the value defaults() installs
// and the routing test harnesses reuse instead of retyping the clauses.
func DefaultTddRule() CondRule { return mustCondRule(DefaultTddRuleExpr) }

// mustCondRule parses a compiled-in expression; a compiled expression that
// does not parse is a programming error, so it panics with the documented
// prefix (the package's only panic).
func mustCondRule(expr string) CondRule {
	r, err := parseCondRule(expr)
	if err != nil {
		panic("config: DefaultTddRuleExpr does not parse: " + err.Error())
	}
	return r
}

// CondRule is a parsed conditional-mandatory predicate, e.g. cycle_size != trivial.
// A rule may AND further clauses (`a!=b && c!=d`, ADR-0099): the head clause
// keeps the single-clause shape (And nil) so every existing consumer reads it
// unchanged; the extra clauses ride in And and the rule holds only when ALL
// clauses hold.
type CondRule struct {
	Field string
	Op    string
	Value string
	And   []CondRule
}

// Clauses returns the rule as a flat clause list — the head followed by And —
// so consumers that reason per clause (the evaluator, the advisor rubric) share
// one projection instead of each flattening head+tail by hand.
func (r CondRule) Clauses() []CondRule {
	out := make([]CondRule, 0, 1+len(r.And))
	out = append(out, CondRule{Field: r.Field, Op: r.Op, Value: r.Value})
	return append(out, r.And...)
}

// parseCondRule parses one or more `&&`-joined clauses of the form
// "field<op>value" (op one of != == >= <= > <). The first clause is the head;
// the rest land in And (ADR-0099). Any malformed clause fails the WHOLE rule —
// a rule can never silently shrink to fewer conditions than the operator wrote.
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

// parseCondClause parses a single "field<op>value" clause. Tolerates
// surrounding whitespace. Two-char ops are matched before one-char.
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
