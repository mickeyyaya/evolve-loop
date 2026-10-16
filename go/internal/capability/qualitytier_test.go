package capability

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestQualityTier_GoldenParityWithBashManifests(t *testing.T) {
	adaptersDir := filepath.Join("..", "..", "..", "adapters")
	if _, err := os.Stat(adaptersDir); err != nil {
		t.Skipf("adapters dir not reachable from package cwd: %v", err)
	}
	allTrue := Probe(func(string) bool { return true })
	allFalse := Probe(func(string) bool { return false })

	cases := []struct {
		cli       string
		wantTrue  string
		wantFalse string
	}{
		{"claude", "full", "full"},
		{"claude-tmux", "none", "none"},
		{"codex", "hybrid", "none"},
		{"gemini", "hybrid", "none"},
		{"agy", "hybrid", "none"},
		{"antigravity", "hybrid", "none"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.cli, func(t *testing.T) {
			gotT, err := QualityTier(adaptersDir, tc.cli, allTrue)
			if err != nil {
				t.Fatalf("probe-true QualityTier(%s): %v", tc.cli, err)
			}
			if gotT != tc.wantTrue {
				t.Errorf("probe-true %s: tier=%q want %q", tc.cli, gotT, tc.wantTrue)
			}
			gotF, err := QualityTier(adaptersDir, tc.cli, allFalse)
			if err != nil {
				t.Fatalf("probe-false QualityTier(%s): %v", tc.cli, err)
			}
			if gotF != tc.wantFalse {
				t.Errorf("probe-false %s: tier=%q want %q", tc.cli, gotF, tc.wantFalse)
			}
		})
	}
}

func TestQualityTier_MissingManifestIsUnknown(t *testing.T) {
	tier, err := QualityTier(t.TempDir(), "no-such-cli", nil)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if tier != "unknown" {
		t.Errorf("missing manifest tier=%q, want unknown", tier)
	}
}

func TestQualityTier_MalformedManifestIsUnknown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.capabilities.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	tier, err := QualityTier(dir, "broken", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if tier != "unknown" {
		t.Errorf("malformed manifest tier=%q, want unknown", tier)
	}
}

func TestResolveMode_FalseProbeLeavesDefault(t *testing.T) {
	manifest := []byte(`{
		"capabilities": {
			"fixed_full":  "full",
			"obj_true":    {"modes": ["hybrid", "degraded"], "default": "degraded"},
			"obj_false":   {"default": "none"}
		},
		"probes": [
			{"check": "p_true",  "if_true_mode": "hybrid", "if_false_mode": "full",     "applies_to": ["obj_true"]},
			{"check": "p_false", "if_true_mode": "hybrid", "if_false_mode": "degraded", "applies_to": ["obj_false"]}
		]
	}`)
	m, err := parseFullManifest(manifest)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	probe := Probe(func(check string) bool { return check == "p_true" })

	if got := resolveMode("obj_false", m.capabilities["obj_false"], m.probes, probe); got != "none" {
		t.Errorf("obj_false mode=%q, want none (if_false_mode must be dead — parity quirk)", got)
	}
	if got := resolveMode("obj_true", m.capabilities["obj_true"], m.probes, probe); got != "hybrid" {
		t.Errorf("obj_true mode=%q, want hybrid", got)
	}
	if got := resolveMode("fixed_full", m.capabilities["fixed_full"], m.probes, probe); got != "full" {
		t.Errorf("fixed_full mode=%q, want full", got)
	}
	if got := tierFromManifest(m, probe); got != "none" {
		t.Errorf("tier=%q, want none", got)
	}
}

func TestTierFromManifest_EmptyCapabilitiesIsFull(t *testing.T) {
	m, err := parseFullManifest([]byte(`{"capabilities": {}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tierFromManifest(m, DefaultProbe); got != "full" {
		t.Errorf("empty caps tier=%q, want full", got)
	}
}

func TestProbeApplies_EmptyAppliesToMatchesAll(t *testing.T) {
	manifest := []byte(`{
		"capabilities": {"a": {"default": "none"}, "b": {"default": "none"}},
		"probes": [{"check": "p", "if_true_mode": "full", "if_false_mode": "", "applies_to": []}]
	}`)
	m, err := parseFullManifest(manifest)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	probe := Probe(func(string) bool { return true })
	if got := tierFromManifest(m, probe); got != "full" {
		t.Errorf("empty applies_to + true probe should lift all caps to full, got %q", got)
	}
}

func TestDefaultProbe_ChecksAndUnknown(t *testing.T) {
	if DefaultProbe("totally-unknown-check") {
		t.Error("unknown check must be false")
	}
	wantClaude := func() bool { _, err := exec.LookPath("claude"); return err == nil }()
	if DefaultProbe("claude_on_path") != wantClaude {
		t.Errorf("claude_on_path=%v, want %v (LookPath parity)", DefaultProbe("claude_on_path"), wantClaude)
	}
	if runtime.GOOS != "darwin" && DefaultProbe("sandbox_exec_available") {
		t.Error("sandbox_exec_available must be false off Darwin")
	}
	if runtime.GOOS != "linux" && DefaultProbe("bwrap_available") {
		t.Error("bwrap_available must be false off Linux")
	}
}
