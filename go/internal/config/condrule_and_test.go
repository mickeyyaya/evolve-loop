package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCondRule_AndClauses(t *testing.T) {
	got, err := parseCondRule("cycle_size!=trivial && deliverable_kind!=document")
	if err != nil {
		t.Fatalf("parseCondRule: %v", err)
	}
	if got.Field != "cycle_size" || got.Op != "!=" || got.Value != "trivial" {
		t.Errorf("head clause = %+v, want {cycle_size != trivial}", got)
	}
	if len(got.And) != 1 || got.And[0].Field != "deliverable_kind" || got.And[0].Op != "!=" || got.And[0].Value != "document" {
		t.Errorf("And = %+v, want [{deliverable_kind != document}]", got.And)
	}
	single, err := parseCondRule("cycle_size!=trivial")
	if err != nil || single.And != nil {
		t.Errorf("single clause must keep And nil: %+v, %v", single, err)
	}
	if _, err := parseCondRule("cycle_size!=trivial && nonsense"); err == nil {
		t.Errorf("a malformed AND clause must fail the whole rule")
	}
	cfg, _ := Load(filepath.Join(t.TempDir(), "absent.json"), map[string]string{
		"EVOLVE_CONDITIONAL_MANDATORY": "tdd:cycle_size!=trivial&&deliverable_kind!=document",
	})
	if r := cfg.Conditional["tdd"]; len(r.And) != 1 || r.And[0].Field != "deliverable_kind" {
		t.Errorf("env form: Conditional[tdd] = %+v, want the AND clause", r)
	}
}

func TestLoad_RegistryTddRule_ReleasesDocumentCycles(t *testing.T) {
	cfg, ws := Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"), map[string]string{})
	for _, w := range ws {
		if w.Code == "unknown-value" {
			t.Errorf("registry warning: %s", w.Message)
		}
	}
	rule, ok := cfg.Conditional["tdd"]
	if !ok || rule.Field != "cycle_size" || rule.Value != "trivial" {
		t.Fatalf("Conditional[tdd] = %+v (ok=%v), want head {cycle_size != trivial}", rule, ok)
	}
	found := false
	for _, c := range rule.And {
		if c.Field == "deliverable_kind" && c.Op == "!=" && c.Value == "document" {
			found = true
		}
	}
	if !found {
		t.Errorf("Conditional[tdd].And = %+v, want a `deliverable_kind != document` release clause (ADR-0099)", rule.And)
	}
	for _, k := range []string{"strategy-options", "business-plan", "partnership-deal"} {
		if len(cfg.GoalRecipes[k]) == 0 {
			t.Errorf("goal_recipes[%s] missing — the advisor needs a recipe row for solution cycles", k)
		}
	}
}

func TestDefaults_TddRuleMatchesRegistry(t *testing.T) {
	want := DefaultTddRule()
	cfg, _ := Load(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"), map[string]string{})
	got := cfg.Conditional["tdd"]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("registry tdd rule %+v != compiled default %+v (DefaultTddRuleExpr=%q)", got, want, DefaultTddRuleExpr)
	}
	if !reflect.DeepEqual(defaults().Conditional["tdd"], want) {
		t.Errorf("defaults().Conditional[tdd] = %+v, want DefaultTddRule()", defaults().Conditional["tdd"])
	}
}

func TestCondRule_Clauses(t *testing.T) {
	r := DefaultTddRule()
	cs := r.Clauses()
	if len(cs) != 2 || cs[0].Field != "cycle_size" || cs[1].Field != "deliverable_kind" {
		t.Errorf("Clauses() = %+v, want [cycle_size, deliverable_kind]", cs)
	}
	if got := (CondRule{Field: "a", Op: "==", Value: "b"}).Clauses(); len(got) != 1 {
		t.Errorf("single-clause Clauses() = %+v, want one clause", got)
	}
}
