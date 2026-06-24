package phasespec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// NOTE: these are white-box tests in `package phasespec`. They CANNOT import
// go/test/fixtures because that package (transitively) pulls in internal/core,
// and core imports phasespec — a white-box phasespec test importing fixtures
// would create an import cycle. Local helpers (writeUserPhase, writeRegistry)
// already defined in the sibling test files are reused instead.

// TestDiscoverUserSpecs_BadDirNameNoName covers the discover.go guard where a
// phase.json carries no "name" and its directory name is not valid kebab-case:
// the spec is skipped with a warning rather than admitting a malformed name.
func TestDiscoverUserSpecs_BadDirNameNoName(t *testing.T) {
	phasesDir := t.TempDir()
	// Directory name "Bad_Name" fails ^[a-z][a-z0-9-]*$ and the body has no name.
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

// TestCatalog_Merge_DuplicateUserPhase covers the Merge branch where two user
// specs share a name (neither clashes with a built-in): the first is kept and
// the second is dropped with a "duplicate user phase" warning.
func TestCatalog_Merge_DuplicateUserPhase(t *testing.T) {
	builtin := Catalog{} // empty built-in catalog: no built-in clash
	user := []PhaseSpec{
		{Name: "lint-pass", Optional: true, Model: "first"},
		{Name: "lint-pass", Optional: true, Model: "second"}, // duplicate → dropped
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

// TestCatalog_UserPhases covers UserPhases (0% before): only operator-overlay
// phases are returned, in registry-insertion order, and built-ins are excluded.
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
	// Insertion order from the user slice, NOT sorted.
	want := []string{"zeta-check", "alpha-check"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UserPhases = %v, want %v (insertion order, built-ins excluded)", got, want)
	}
}

// TestCatalog_UserPhases_NoneWhenAllBuiltin covers UserPhases returning an empty
// (non-nil) slice when the catalog has no operator overlays.
func TestCatalog_UserPhases_NoneWhenAllBuiltin(t *testing.T) {
	cat, err := Load(writeRegistry(t, fullRegistry))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.UserPhases(); len(got) != 0 {
		t.Errorf("UserPhases = %v, want empty for an all-built-in catalog", names(got))
	}
}

// TestLoad_EmptyNameSkipped covers the Load branch that drops a spec whose name
// is empty (it is silently skipped, no error, and absent from the catalog).
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

// TestApplyUserRouting_InitsNilTriggers covers the cfg.Triggers==nil init branch:
// a valid spec carrying Routing is applied to a config whose Triggers map is nil,
// so ApplyUserRouting must allocate the map before registering the trigger.
func TestApplyUserRouting_InitsNilTriggers(t *testing.T) {
	cfg := config.RoutingConfig{
		Order:    []string{"scout", "build", "audit", "ship"},
		Triggers: nil, // not yet allocated
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
	if cfg.Triggers == nil {
		t.Fatal("Triggers map should have been allocated")
	}
	if _, ok := cfg.Triggers["security-scan"]; !ok {
		t.Error("trigger for security-scan not registered after nil-map init")
	}
}

// TestSpliceAfter_NamePresentNoOp covers the spliceAfter guard that returns the
// order untouched when the phase name is already present (idempotent splice).
func TestSpliceAfter_NamePresentNoOp(t *testing.T) {
	order := []string{"scout", "security-scan", "audit"}
	got := spliceAfter(order, "security-scan", "scout")
	want := []string{"scout", "security-scan", "audit"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spliceAfter(present) = %v, want unchanged %v", got, want)
	}
}

// TestSpliceAfter_NoAnchorNoAudit covers the spliceAfter fallthrough where the
// anchor is absent AND "audit" is absent: the name is appended at the end.
func TestSpliceAfter_NoAnchorNoAudit(t *testing.T) {
	order := []string{"scout", "build"} // no audit
	got := spliceAfter(order, "x-check", "nonexistent-anchor")
	want := []string{"scout", "build", "x-check"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spliceAfter(no anchor, no audit) = %v, want appended %v", got, want)
	}
}

// contains is a tiny substring helper local to this test file.
func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
