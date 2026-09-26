package looppreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_PhaseRoutingWarnings_NoWarnings_Pass(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.PhaseRoutingWarnings = func() []string { return nil }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level != LevelPass {
		t.Fatalf("want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
	if r.Halted() {
		t.Fatalf("expected no halt, got OverallLevel=%s checks=%+v", r.OverallLevel, r.Checks)
	}
}

func TestRun_PhaseRoutingWarnings_WarningsPresent_Warn(t *testing.T) {
	opts := goodPipelineOptions(t)
	const wantWarning = `phase widget not routed (invalid): user phase must be optional:true`
	opts.PhaseRoutingWarnings = func() []string { return []string{wantWarning} }
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, wantWarning) {
		t.Fatalf("Detail must carry the full warning text (proving it is no longer silently swallowed to stderr); got %q", c.Detail)
	}
	if r.OverallLevel != LevelWarn {
		t.Fatalf("Result.OverallLevel = %s, want LevelWarn (the warning must surface at the top-level verdict)", r.OverallLevel)
	}
}

func TestRun_PhaseRoutingWarnings_DoesNotHalt(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.PhaseRoutingWarnings = func() []string {
		return []string{
			"phase widget not routed (invalid): name must be multi-word kebab-case",
			"phase audit clashes with a built-in — built-in kept, user definition ignored",
			"skipped .evolve/phases/broken/phase.json: malformed JSON",
		}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("phase-routing warnings must never halt the batch; got OverallLevel=%s", r.OverallLevel)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level == LevelHalt {
		t.Fatalf("checkPhaseRoutingWarnings must never return LevelHalt; got %s", c.Level)
	}
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn for 3 accumulated warnings, got %s", c.Level)
	}
}

// An optional overlay named after the non-optional built-in "audit" is dropped with a clash warning.
func TestRun_PhaseRoutingWarnings_DefaultUsesRealMergedCatalog(t *testing.T) {
	root := t.TempDir()
	registryDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll registry dir: %v", err)
	}
	registry := `{"phases":[{"name":"audit","optional":false},{"name":"build","optional":false}]}`
	if err := os.WriteFile(filepath.Join(registryDir, "phase-registry.json"), []byte(registry), 0o644); err != nil {
		t.Fatalf("write phase-registry.json: %v", err)
	}
	overlayDir := filepath.Join(root, ".evolve", "phases", "audit")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatalf("MkdirAll overlay dir: %v", err)
	}
	overlay := `{"name":"audit","optional":true,"agent":"evolve-hijack"}`
	if err := os.WriteFile(filepath.Join(overlayDir, "phase.json"), []byte(overlay), 0o644); err != nil {
		t.Fatalf("write phase.json: %v", err)
	}

	opts := goodPipelineOptions(t)
	opts.ProjectRoot = root
	opts.PhaseRoutingWarnings = nil

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "phase-routing-warnings")
	if c.Level != LevelWarn {
		t.Fatalf("want LevelWarn from the real audit-name-hijack overlay, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "audit") || !strings.Contains(c.Detail, "clashes with a built-in") {
		t.Fatalf("Detail must name the real clash warning from phasespec.MergedCatalog; got %q", c.Detail)
	}
	if r.Halted() {
		t.Fatalf("a dropped hijack overlay must warn, not halt; got OverallLevel=%s", r.OverallLevel)
	}
}
