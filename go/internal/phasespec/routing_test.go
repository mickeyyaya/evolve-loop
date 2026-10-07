package phasespec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestApplyUserRouting_SplicesValidPhase(t *testing.T) {
	cfg := config.RoutingConfig{
		Order:       []string{"scout", "build", "audit", "ship"},
		Triggers:    map[string]config.RoutingBlock{},
		PhaseEnable: map[string]config.Enable{},
	}
	specs := []PhaseSpec{{
		Name:     "security-scan",
		Optional: true,
		After:    "build",
		Routing:  &config.RoutingBlock{InsertWhen: []config.Condition{{Field: "build.files_touched", Op: "gt", Value: 0}}},
	}}
	warns := ApplyUserRouting(&cfg, specs, Catalog{})
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	want := []string{"scout", "build", "security-scan", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want %v", cfg.Order, want)
	}
	if _, ok := cfg.Triggers["security-scan"]; !ok {
		t.Error("trigger not registered")
	}
	if cfg.PhaseEnable["security-scan"] != config.EnableContent {
		t.Error("phase should be content-routed")
	}
}

func TestApplyUserRouting_DefaultsBeforeAudit(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "build", "audit", "ship"}}
	ApplyUserRouting(&cfg, []PhaseSpec{{Name: "x-check", Optional: true}}, Catalog{})
	want := []string{"scout", "build", "x-check", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want x-check before audit %v", cfg.Order, want)
	}
}

func TestApplyUserRouting_SkipsInvalid(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "build", "audit", "ship"}}
	warns := ApplyUserRouting(&cfg, []PhaseSpec{{Name: "bad", Optional: false}}, Catalog{})
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want 1 (invalid skipped)", warns)
	}
	if orderIndexHas(cfg.Order, "bad") {
		t.Error("invalid phase must not enter the routing order")
	}
}

func orderIndexHas(order []string, name string) bool { return indexOfStr(order, name) >= 0 }

func TestApplyUserRouting_AnUnregisteredGrammarIsALoadErrorNotAnAgentCorrection(t *testing.T) {
	cfg := config.RoutingConfig{
		Order:       []string{"scout", "build", "audit", "ship"},
		Triggers:    map[string]config.RoutingBlock{},
		PhaseEnable: map[string]config.Enable{},
	}
	specs := []PhaseSpec{
		{Name: "typo-review", Optional: true, After: "build", Classify: &ClassifyRules{Grammars: []string{"code-review-reprot"}}},
		{Name: "real-review", Optional: true, After: "build", Classify: &ClassifyRules{Grammars: []string{GrammarCodeReviewReport}}},
	}

	warns := ApplyUserRouting(&cfg, specs, Catalog{})

	if len(warns) != 1 || !strings.Contains(warns[0], "typo-review") || !strings.Contains(warns[0], `"code-review-reprot"`) {
		t.Fatalf("warnings = %v, want one naming the phase and its unregistered grammar", warns)
	}
	if want := []string{"scout", "build", "real-review", "audit", "ship"}; !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want %v: the phase with an unregistered grammar is never loaded", cfg.Order, want)
	}
}

func TestGrammars_IsACopyACallerCannotRewrite(t *testing.T) {
	got := Grammars()
	if !reflect.DeepEqual(got, []string{GrammarCodeReviewReport}) {
		t.Fatalf("Grammars() = %v, want [%s]", got, GrammarCodeReviewReport)
	}

	got[0] = "rewritten"

	if again := Grammars(); again[0] != GrammarCodeReviewReport || len(ValidateUserSpec(PhaseSpec{Name: "real-review", Optional: true, Classify: &ClassifyRules{Grammars: []string{GrammarCodeReviewReport}}})) != 0 {
		t.Errorf("Grammars() after a caller's write = %v: the vocabulary must not be the caller's slice", again)
	}
}

func TestValidateUserSpec_AGrammarNameThatDiffersInCaseIsUnregistered(t *testing.T) {
	spec := PhaseSpec{Name: "real-review", Optional: true, Classify: &ClassifyRules{Grammars: []string{"Code-Review-Report"}}}

	if v := ValidateUserSpec(spec); len(v) != 1 || !strings.Contains(v[0], `"Code-Review-Report"`) {
		t.Errorf("violations = %v, want the case variant refused: the gate binds the exact name only", v)
	}
}
