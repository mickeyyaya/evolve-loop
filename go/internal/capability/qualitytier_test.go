package capability

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestQualityTier_GoldenParityWithBashManifests pins the Go port to the live
// _capability-check.sh output captured (2026-06-18) over the real, kept
// adapters/*.capabilities.json manifests. probe-true vs probe-false matches the
// two deterministic golden runs (binary present / absent). This is the
// before-deletion parity oracle for Wave A.
func TestQualityTier_GoldenParityWithBashManifests(t *testing.T) {
	adaptersDir := filepath.Join("..", "..", "..", "adapters")
	if _, err := os.Stat(adaptersDir); err != nil {
		t.Skipf("adapters dir not reachable from package cwd: %v", err)
	}
	allTrue := Probe(func(string) bool { return true })
	allFalse := Probe(func(string) bool { return false })

	cases := []struct {
		cli       string
		wantTrue  string // probe true  (claude/agy on PATH)
		wantFalse string // probe false (off PATH)
	}{
		{"claude", "full", "full"},        // no probes, fixed/default full
		{"claude-tmux", "none", "none"},   // no probes, lowest fixed mode is budget_cap=none
		{"codex", "hybrid", "none"},       // claude_on_path: true→hybrid, false→default(none) via parity quirk
		{"gemini", "hybrid", "none"},      // same shape as codex
		{"agy", "hybrid", "none"},         // agy_on_path
		{"antigravity", "hybrid", "none"}, // agy_on_path
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

// TestQualityTier_MissingManifestIsUnknown matches the bash caller: a missing
// manifest makes _capability-check.sh exit non-zero, and consensus-dispatch
// records the voter's tier as "unknown" (excluded by require_min_tier≥hybrid).
func TestQualityTier_MissingManifestIsUnknown(t *testing.T) {
	tier, err := QualityTier(t.TempDir(), "no-such-cli", nil)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if tier != "unknown" {
		t.Errorf("missing manifest tier=%q, want unknown", tier)
	}
}

// TestQualityTier_MalformedManifestIsUnknown mirrors bash `jq empty` failing on
// a non-JSON manifest → exit 1 → tier "unknown".
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

// TestResolveMode_FalseProbeLeavesDefault pins the PARITY QUIRK: a false probe
// must NOT apply if_false_mode (the live bash collapses false→unknown via jq
// `//`, so the default survives). If this test ever reports "degraded", the Go
// port has diverged from the shell by "fixing" the quirk — which is a behavior
// change, not a migration.
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

	// obj_false: default none, p_false→false → stays none (NOT degraded).
	if got := resolveMode("obj_false", m.capabilities["obj_false"], m.probes, probe); got != "none" {
		t.Errorf("obj_false mode=%q, want none (if_false_mode must be dead — parity quirk)", got)
	}
	// obj_true: default degraded, p_true→true → upgraded to hybrid.
	if got := resolveMode("obj_true", m.capabilities["obj_true"], m.probes, probe); got != "hybrid" {
		t.Errorf("obj_true mode=%q, want hybrid", got)
	}
	// fixed string capability returns its literal regardless of probes.
	if got := resolveMode("fixed_full", m.capabilities["fixed_full"], m.probes, probe); got != "full" {
		t.Errorf("fixed_full mode=%q, want full", got)
	}
	// Aggregate = lowest = none (obj_false).
	if got := tierFromManifest(m, probe); got != "none" {
		t.Errorf("tier=%q, want none", got)
	}
}

// TestTierFromManifest_EmptyCapabilitiesIsFull pins the degenerate case: no
// capabilities → low stays at rankFull → "full" (bash LOW_RANK=3 initial).
func TestTierFromManifest_EmptyCapabilitiesIsFull(t *testing.T) {
	m, err := parseFullManifest([]byte(`{"capabilities": {}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tierFromManifest(m, DefaultProbe); got != "full" {
		t.Errorf("empty caps tier=%q, want full", got)
	}
}

// TestProbeApplies_EmptyAppliesToMatchesAll covers the bash select where an
// absent/empty applies_to matches every capability.
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

// TestDefaultProbe_ChecksAndUnknown names DefaultProbe and pins: unknown checks
// are false, OS-gated sandbox probes respect GOOS, and binary probes agree with
// exec.LookPath on this host.
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
