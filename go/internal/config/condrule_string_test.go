package config

import "testing"

func TestCondRuleString_ReproducesTheRegistryExpression(t *testing.T) {
	if got := DefaultTddRule().String(); got != DefaultTddRuleExpr {
		t.Errorf("DefaultTddRule().String() = %q, want the registry expression %q", got, DefaultTddRuleExpr)
	}
	if got := (CondRule{Field: "triage.unified_size", Op: "!=", Value: ""}).String(); got != "triage.unified_size!=" {
		t.Errorf("single clause = %q, want triage.unified_size!=", got)
	}
}
