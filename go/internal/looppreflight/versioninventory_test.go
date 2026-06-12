package looppreflight

// versioninventory_test.go — RED tests for cycle-308 task
// `cli-version-lifecycle-preflight` (inbox item 2026-06-12T16-08-42Z).
//
// Root cause: claude moved 2.1.173 → 2.1.175 BETWEEN batches despite
// autoUpdates:false (a staged-update-on-boot path the freeze setting doesn't
// gate). loop-preflight.json recorded no version strings, so the silent version
// change was invisible. This task adds:
//
//	(1) version inventory — capture `<bin> --version` for each tmux CLI into
//	    loop-preflight.json (Result.CLIVersions, persisted as "cli_versions");
//	(2) drift detection — persist last-seen versions to .evolve/cli-versions.json
//	    and WARN when an inventoried CLI's version changed vs the previous batch.
//
// New API this file pins (Builder implements):
//
//	var execVersion = func(bin string) (string, error)            (versioninventory.go)
//	captureVersionInventory(bins []string) map[string]string      (versioninventory.go)
//	checkCLIVersionDrift(o resolved) CheckResult                  (checks.go) — name "cli-version-drift"
//	Options.VersionInventory func() map[string]string             (looppreflight.go) — seam, default captures tmux bins
//	Result.CLIVersions map[string]string                          (looppreflight.go) — JSON "cli_versions"
//
// Helpers goodPipelineOptions/findCheck/fixedNow live in looppreflight_test.go.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliVersionsFile = "cli-versions.json"

// writeCLIVersions persists a last-seen version map to .evolve/cli-versions.json
// under evolveDir (the prior-batch record the drift check compares against).
func writeCLIVersions(t *testing.T, evolveDir string, versions map[string]string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(versions)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, cliVersionsFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCLIVersionInventory pins captureVersionInventory's parse behavior via the
// execVersion seam: a version-looking token is extracted per binary, and a
// binary whose probe errors is omitted (best-effort, never a crash). Behavioral
// — exercises the real parser; a no-op returning an empty map fails it.
func TestCLIVersionInventory(t *testing.T) {
	orig := execVersion
	t.Cleanup(func() { execVersion = orig })
	execVersion = func(bin string) (string, error) {
		switch bin {
		case "claude":
			return "claude 2.1.175 (release build)", nil
		case "codex":
			return "codex-cli 0.139.0", nil
		}
		return "", fmt.Errorf("%s: not found", bin)
	}

	inv := captureVersionInventory([]string{"claude", "codex", "missing"})
	if inv["claude"] != "2.1.175" {
		t.Errorf("claude version = %q, want 2.1.175", inv["claude"])
	}
	if inv["codex"] != "0.139.0" {
		t.Errorf("codex version = %q, want 0.139.0", inv["codex"])
	}
	if v, ok := inv["missing"]; ok {
		t.Errorf("a binary whose probe errors must be omitted; got %q", v)
	}
}

// TestCLIVersionInventory_LandsInPreflight: the captured inventory is exposed on
// Result.CLIVersions AND serialized into the persisted loop-preflight.json under
// "cli_versions" (the observability gap the incident exposed).
func TestCLIVersionInventory_LandsInPreflight(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.CLIVersions["claude"] != "2.1.175" {
		t.Errorf("Result.CLIVersions[claude] = %q, want 2.1.175", r.CLIVersions["claude"])
	}
	// The persisted loop-preflight.json payload must carry the inventory.
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal Result: %v", err)
	}
	if !strings.Contains(string(b), `"cli_versions"`) {
		t.Errorf("loop-preflight.json payload missing cli_versions field; got %s", b)
	}
	if !strings.Contains(string(b), "2.1.175") {
		t.Errorf("loop-preflight.json payload missing the captured version; got %s", b)
	}
}

// TestVersionDrift_Fires_On_Synthetic_Transition: the incident replay. The prior
// batch recorded claude 2.1.173; this batch sees 2.1.175 → the drift check WARNs
// and names both versions so the operator sees exactly what moved.
func TestVersionDrift_Fires_On_Synthetic_Transition(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.173"})
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn {
		t.Fatalf("a version change vs the prior batch must WARN (drift); got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "2.1.173") || !strings.Contains(c.Detail, "2.1.175") {
		t.Errorf("drift detail must show old→new (2.1.173 → 2.1.175); got %q", c.Detail)
	}
}

// TestVersionDrift_NoWarnWhenVersionUnchanged: a steady version across batches
// must not WARN (the common case — no false alarms).
func TestVersionDrift_NoWarnWhenVersionUnchanged(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.175"})
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level == LevelWarn {
		t.Errorf("unchanged version must NOT WARN; got WARN with detail %q", c.Detail)
	}
}

// TestVersionDrift_NoWarnWhenNoPriorRecord: the first batch (no
// .evolve/cli-versions.json yet) must not WARN — it records the baseline for
// next time. Behavioral: assert the baseline file is persisted afterward.
func TestVersionDrift_NoWarnWhenNoPriorRecord(t *testing.T) {
	opts := goodPipelineOptions(t)
	// EvolveDir from goodPipelineOptions is a fresh empty temp dir — no prior record.
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level == LevelWarn {
		t.Errorf("first batch (no prior record) must NOT WARN; got WARN with detail %q", c.Detail)
	}
	// The baseline must be persisted so the NEXT batch can compare.
	persisted := filepath.Join(opts.EvolveDir, cliVersionsFile)
	body, readErr := os.ReadFile(persisted)
	if readErr != nil {
		t.Fatalf("baseline cli-versions.json not persisted on first run: %v", readErr)
	}
	if !strings.Contains(string(body), "2.1.175") {
		t.Errorf("persisted baseline missing the captured version; got %s", body)
	}
}
