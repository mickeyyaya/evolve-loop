package phasespec

import (
	"reflect"
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
	warns := ApplyUserRouting(&cfg, specs)
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
	ApplyUserRouting(&cfg, []PhaseSpec{{Name: "x-check", Optional: true}}) // no After
	want := []string{"scout", "build", "x-check", "audit", "ship"}
	if !reflect.DeepEqual(cfg.Order, want) {
		t.Errorf("Order = %v, want x-check before audit %v", cfg.Order, want)
	}
}

func TestApplyUserRouting_SkipsInvalid(t *testing.T) {
	cfg := config.RoutingConfig{Order: []string{"scout", "build", "audit", "ship"}}
	// not optional → floor violation → must NOT be spliced/routed
	warns := ApplyUserRouting(&cfg, []PhaseSpec{{Name: "bad", Optional: false}})
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want 1 (invalid skipped)", warns)
	}
	if orderIndexHas(cfg.Order, "bad") {
		t.Error("invalid phase must not enter the routing order")
	}
}

func orderIndexHas(order []string, name string) bool { return indexOfStr(order, name) >= 0 }
