package capability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspect_MissingManifestDefaultsToSupported(t *testing.T) {
	insp, err := Inspect(t.TempDir(), "no-such-adapter")
	if err != nil {
		t.Fatalf("expected nil error for missing manifest, got %v", err)
	}
	if !insp.Manifest.BudgetNative || !insp.Manifest.PermissionScoping {
		t.Errorf("missing manifest should default both supports=true, got %+v", insp.Manifest)
	}
	if len(insp.Warns) != 0 {
		t.Errorf("missing manifest should emit no warns, got %d", len(insp.Warns))
	}
}

func TestInspect_PermissionDeniedSurfacesError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root cannot trigger permission denied")
	}
	tmp := t.TempDir()
	manifest := filepath.Join(tmp, "claude.capabilities.json")
	if err := os.WriteFile(manifest, []byte(`{"supports":{}}`), 0o000); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(manifest, 0o600) })
	_, err := Inspect(tmp, "claude")
	if err == nil {
		t.Fatalf("expected permission error")
	}
}

func TestInspect_FullSupportNoWarns(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "claude.capabilities.json"),
		[]byte(`{"supports":{"budget_cap_native":true,"permission_scoping":true}}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	insp, err := Inspect(tmp, "claude")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !insp.Manifest.BudgetNative || !insp.Manifest.PermissionScoping {
		t.Errorf("expected both supports true, got %+v", insp.Manifest)
	}
	if len(insp.Warns) != 0 {
		t.Errorf("expected no warns, got %v", insp.Warns)
	}
}

func TestInspect_DegradedSupportEmitsWarns(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "agy.capabilities.json"),
		[]byte(`{"supports":{"budget_cap_native":false,"permission_scoping":false}}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	insp, _ := Inspect(tmp, "agy")
	if insp.Manifest.BudgetNative || insp.Manifest.PermissionScoping {
		t.Errorf("expected both supports false, got %+v", insp.Manifest)
	}
	if len(insp.Warns) != 2 {
		t.Fatalf("expected 2 warns, got %d: %v", len(insp.Warns), insp.Warns)
	}
	for _, w := range insp.Warns {
		if !strings.HasPrefix(w, "[adapter-cap] WARN cli=agy ") {
			t.Errorf("malformed warn: %s", w)
		}
	}
	if !strings.Contains(insp.Warns[0], "budget_cap_native") || !strings.Contains(insp.Warns[0], "wall_clock_timeout") {
		t.Errorf("first warn missing budget context: %s", insp.Warns[0])
	}
	if !strings.Contains(insp.Warns[1], "permission_scoping") || !strings.Contains(insp.Warns[1], "kernel_role_gate_only") {
		t.Errorf("second warn missing permission context: %s", insp.Warns[1])
	}
}

func TestInspectBytes_MissingSupportsBlockDefaults(t *testing.T) {
	insp := inspectBytes([]byte(`{"other":{}}`), "x")
	if !insp.Manifest.BudgetNative || !insp.Manifest.PermissionScoping {
		t.Errorf("missing .supports should default true")
	}
	if len(insp.Warns) != 0 {
		t.Errorf("missing .supports should emit no warns")
	}
}

func TestInspectBytes_NullValuesDefault(t *testing.T) {
	// bash: `if . == null then "true" else tostring`
	insp := inspectBytes([]byte(`{"supports":{"budget_cap_native":null,"permission_scoping":null}}`), "x")
	if !insp.Manifest.BudgetNative || !insp.Manifest.PermissionScoping {
		t.Errorf("null supports values should default true, got %+v", insp.Manifest)
	}
}

func TestInspectBytes_OnlyOneSupportDegraded(t *testing.T) {
	insp := inspectBytes([]byte(`{"supports":{"budget_cap_native":true,"permission_scoping":false}}`), "codex")
	if !insp.Manifest.BudgetNative {
		t.Errorf("budget should be true")
	}
	if insp.Manifest.PermissionScoping {
		t.Errorf("permission should be false")
	}
	if len(insp.Warns) != 1 {
		t.Fatalf("expected 1 warn, got %d", len(insp.Warns))
	}
	if !strings.Contains(insp.Warns[0], "permission_scoping") {
		t.Errorf("expected permission warn, got %s", insp.Warns[0])
	}
}

func TestExtractBool_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		field string
		def   bool
		want  bool
	}{
		{"no supports block", `{}`, "x", true, true},
		{"supports without field", `{"supports":{"other":true}}`, "x", false, false},
		{"true literal", `{"supports":{"x":true}}`, "x", false, true},
		{"false literal", `{"supports":{"x":false}}`, "x", true, false},
		{"whitespace around colon", `{"supports":{"x" : true}}`, "x", false, true},
		{"trailing comma + whitespace", `{"supports":{"x":  false ,"y":1}}`, "x", true, false},
		{"non-bool value defaults", `{"supports":{"x":"yes"}}`, "x", true, true},
		{"missing colon defaults", `{"supports":{"x"true}}`, "x", true, true},
		{"trailing only-field whitespace", `{"supports":{"x": }}`, "x", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBool(tc.body, tc.field, tc.def)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractObject_NestedBraces(t *testing.T) {
	body := `{"supports":{"nested":{"deep":1},"flat":2}}`
	inner, ok := extractObject(body, "supports")
	if !ok {
		t.Fatalf("expected supports to extract")
	}
	if !strings.Contains(inner, "nested") || !strings.Contains(inner, "flat") {
		t.Errorf("inner missing keys: %s", inner)
	}
}

func TestExtractObject_NotPresent(t *testing.T) {
	if _, ok := extractObject(`{"only":"string"}`, "supports"); ok {
		t.Errorf("expected absent")
	}
	if _, ok := extractObject(`{"supports":"not-an-object"}`, "supports"); ok {
		t.Errorf("string value should not match object extractor")
	}
	if _, ok := extractObject(`{"supports" }`, "supports"); ok {
		t.Errorf("malformed should not match")
	}
}

func TestPlanJSON_AllTrueNoWarns(t *testing.T) {
	p := DispatchPlan{
		CLI:                "claude",
		Model:              "opus",
		CLIResolutionSrc:   "profile",
		CapBudgetNative:    true,
		CapPermissionScope: true,
	}
	got := p.PlanJSON()
	want := `{"cli":"claude","model":"opus","cli_resolution_source":"profile","cap_budget_native":true,"cap_permission_scoping":true,"capability_warns":[]}`
	if got != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestPlanJSON_FalseBoolsAndWarns(t *testing.T) {
	p := DispatchPlan{
		CLI:                "agy",
		Model:              "sonnet",
		CLIResolutionSrc:   "llm_config",
		CapBudgetNative:    false,
		CapPermissionScope: false,
		Warns: []string{
			"[adapter-cap] WARN cli=agy missing=budget_cap_native substitute=wall_clock_timeout",
			"[adapter-cap] WARN cli=agy missing=permission_scoping substitute=kernel_role_gate_only",
		},
	}
	got := p.PlanJSON()
	want := `{"cli":"agy","model":"sonnet","cli_resolution_source":"llm_config","cap_budget_native":false,"cap_permission_scoping":false,"capability_warns":["cli=agy missing=budget_cap_native substitute=wall_clock_timeout","cli=agy missing=permission_scoping substitute=kernel_role_gate_only"]}`
	if got != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestPlanJSON_QuotesInValuesAreEscaped(t *testing.T) {
	p := DispatchPlan{
		CLI:                `claude"weird`,
		Model:              "opus",
		CLIResolutionSrc:   "profile",
		CapBudgetNative:    true,
		CapPermissionScope: true,
	}
	got := p.PlanJSON()
	if !strings.Contains(got, `"cli":"claude\"weird"`) {
		t.Errorf("quote not escaped: %s", got)
	}
}

func TestPlanJSON_BackslashEscaped(t *testing.T) {
	p := DispatchPlan{
		CLI:                `c:\path`,
		Model:              "opus",
		CLIResolutionSrc:   "profile",
		CapBudgetNative:    true,
		CapPermissionScope: true,
	}
	if !strings.Contains(p.PlanJSON(), `"cli":"c:\\path"`) {
		t.Errorf("backslash not escaped: %s", p.PlanJSON())
	}
}

func TestBoolToken(t *testing.T) {
	if boolToken(true) != "true" || boolToken(false) != "false" {
		t.Errorf("boolToken broken")
	}
}
