package looppreflight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliVersionsFile = "cli-versions.json"

// writeCLIVersions writes the prior batch's record that the drift check compares against.
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

func TestVersionDrift_NoWarnWhenNoPriorRecord(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level == LevelWarn {
		t.Errorf("first batch (no prior record) must NOT WARN; got WARN with detail %q", c.Detail)
	}
	persisted := filepath.Join(opts.EvolveDir, cliVersionsFile)
	body, readErr := os.ReadFile(persisted)
	if readErr != nil {
		t.Fatalf("baseline cli-versions.json not persisted on first run: %v", readErr)
	}
	if !strings.Contains(string(body), "2.1.175") {
		t.Errorf("persisted baseline missing the captured version; got %s", body)
	}
}
