package phasespec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestDiscoverUserSpecs_BadDirNameNoName(t *testing.T) {
	phasesDir := t.TempDir()
	writeUserPhase(t, phasesDir, "Bad_Name", `{"optional":true}`)

	specs, warnings := DiscoverUserSpecs(phasesDir)

	if len(specs) != 0 {
		t.Errorf("specs = %v, want none (bad dir name + no name → skipped)", names(specs))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
	if got := warnings[0]; !contains(got, "not valid kebab-case") {
		t.Errorf("warning = %q, want it to mention kebab-case", got)
	}
}

func TestCatalog_Merge_DuplicateUserPhase(t *testing.T) {
	builtin := Catalog{}
	user := []PhaseSpec{
		{Name: "lint-pass", Optional: true, Model: "first"},
		{Name: "lint-pass", Optional: true, Model: "second"},
	}

	merged, warnings := builtin.Merge(user)

	got, ok := merged.Get("lint-pass")
	if !ok {
		t.Fatal("lint-pass missing from merged catalog")
	}
	if got.Model != "first" {
		t.Errorf("Model = %q, want \"first\" (first kept)", got.Model)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want 1 (duplicate)", warnings)
	}
	if !contains(warnings[0], "duplicate user phase lint-pass") {
		t.Errorf("warning = %q, want duplicate-user-phase message", warnings[0])
	}
}

func TestCatalog_UserPhases(t *testing.T) {
	builtin, err := Load(writeRegistry(t, fullRegistry)) // scout, security-scan (built-in)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	merged, _ := builtin.Merge([]PhaseSpec{
		{Name: "zeta-check", Optional: true},
		{Name: "alpha-check", Optional: true},
	})

	got := names(merged.UserPhases())
	want := []string{"zeta-check", "alpha-check"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UserPhases = %v, want %v (insertion order, built-ins excluded)", got, want)
	}
}

func TestCatalog_UserPhases_NoneWhenAllBuiltin(t *testing.T) {
	cat, err := Load(writeRegistry(t, fullRegistry))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.UserPhases(); len(got) != 0 {
		t.Errorf("UserPhases = %v, want empty for an all-built-in catalog", names(got))
	}
}

func TestLoad_EmptyNameSkipped(t *testing.T) {
	body := `{ "phases": [
		{ "name": "" , "model": "ghost" },
		{ "name": "real-phase", "optional": true }
	] }`
	cat, err := Load(writeRegistry(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	all := cat.All()
	if len(all) != 1 || all[0].Name != "real-phase" {
		t.Errorf("All = %v, want only [real-phase] (empty-name dropped)", names(all))
	}
}

func TestApplyUserRouting_InitsNilTriggers(t *testing.T) {
	cfg := config.RoutingConfig{
		Order:    []string{"scout", "build", "audit", "ship"},
		Triggers: nil,
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
	if cfg.Triggers == nil {
		t.Fatal("Triggers map should have been allocated")
	}
	if _, ok := cfg.Triggers["security-scan"]; !ok {
		t.Error("trigger for security-scan not registered after nil-map init")
	}
}

func TestSpliceAfter_NamePresentNoOp(t *testing.T) {
	order := []string{"scout", "security-scan", "audit"}
	got := spliceAfter(order, "security-scan", "scout")
	want := []string{"scout", "security-scan", "audit"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spliceAfter(present) = %v, want unchanged %v", got, want)
	}
}

func TestSpliceAfter_NoAnchorNoAudit(t *testing.T) {
	order := []string{"scout", "build"}
	got := spliceAfter(order, "x-check", "nonexistent-anchor")
	want := []string{"scout", "build", "x-check"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spliceAfter(no anchor, no audit) = %v, want appended %v", got, want)
	}
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
