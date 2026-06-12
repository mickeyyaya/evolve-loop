package looppreflight

// versioninventory_amplified_test.go — cycle-308 adversarial amplification
// for captureVersionInventory and checkCLIVersionDrift
// (cli-version-lifecycle-preflight task).
//
// Targets gaps in the TDD contract: empty bins list, simultaneous multi-CLI
// drift, and corrupted cache fail-open.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaptureVersionInventory_EmptyBinsList: no bins provided → empty map with
// no crash. Ensures the function is not guarded by a len>0 precondition that
// would panic on an empty slice.
func TestCaptureVersionInventory_EmptyBinsList(t *testing.T) {
	orig := execVersion
	t.Cleanup(func() { execVersion = orig })
	execVersion = func(bin string) (string, error) {
		return "", fmt.Errorf("should not be called for empty list")
	}

	inv := captureVersionInventory([]string{})
	if len(inv) != 0 {
		t.Errorf("captureVersionInventory([]) = %v, want empty map", inv)
	}
}

// TestVersionDrift_MultipleCLIsDriftSimultaneously: the incident-replay extends
// to multiple CLIs changing in the same batch. The WARN detail must name ALL
// drifting binaries so the operator can see the full scope of the transition.
func TestVersionDrift_MultipleCLIsDriftSimultaneously(t *testing.T) {
	opts := goodPipelineOptions(t)
	// Prior batch: both claude and codex at lower versions.
	writeCLIVersions(t, opts.EvolveDir, map[string]string{
		"claude": "2.1.173",
		"codex":  "0.137.0",
	})
	// Current batch: both moved forward.
	opts.VersionInventory = func() map[string]string {
		return map[string]string{
			"claude": "2.1.175",
			"codex":  "0.139.0",
		}
	}

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level != LevelWarn {
		t.Fatalf("simultaneous multi-CLI drift must WARN; got %s (%q)", c.Level, c.Detail)
	}
	// Both binaries and their version transitions must appear in the detail.
	detail := c.Detail
	for _, want := range []string{"claude", "2.1.173", "2.1.175", "codex", "0.137.0", "0.139.0"} {
		if !strings.Contains(detail, want) {
			t.Errorf("drift detail missing %q; got %q", want, detail)
		}
	}
}

// TestVersionDrift_CorruptedCacheFailsOpen: a malformed cli-versions.json (present
// but not valid JSON) must NOT WARN and must NOT crash. The check fails open
// (treats it as "no prior record") and overwrites the corrupt file with a fresh
// baseline for the next batch.
func TestVersionDrift_CorruptedCacheFailsOpen(t *testing.T) {
	opts := goodPipelineOptions(t)
	// Write a corrupt cache file.
	if err := os.MkdirAll(opts.EvolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(opts.EvolveDir, cliVersionsFile)
	if err := os.WriteFile(cacheFile, []byte(`{ not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run must not error on corrupt cache; got %v", err)
	}
	c := findCheck(t, r, "cli-version-drift")
	if c.Level == LevelWarn {
		t.Errorf("corrupt cache must fail open (no WARN); got WARN with detail %q", c.Detail)
	}

	// The corrupt file must be replaced with a valid baseline for the next batch.
	body, readErr := os.ReadFile(cacheFile)
	if readErr != nil {
		t.Fatalf("cache file missing after corrupt-cache run: %v", readErr)
	}
	var fresh map[string]string
	if err := json.Unmarshal(body, &fresh); err != nil {
		t.Fatalf("cache file after corrupt-cache run is still invalid JSON: %v\nbody: %s", err, body)
	}
	if fresh["claude"] != "2.1.175" {
		t.Errorf("fresh baseline claude = %q, want 2.1.175", fresh["claude"])
	}
}

// TestVersionDrift_UpdatedCacheReflectsCurrentInventory: after a successful run
// (drift detected or not), the persisted cli-versions.json must reflect the
// CURRENT batch's inventory, not the prior one. This ensures successive batches
// converge: if versions stay stable, subsequent batches won't re-WARN.
func TestVersionDrift_UpdatedCacheReflectsCurrentInventory(t *testing.T) {
	opts := goodPipelineOptions(t)
	// Prior batch at old version.
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.173"})
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// After the run, the cache must store the NEW version (2.1.175).
	cacheFile := filepath.Join(opts.EvolveDir, cliVersionsFile)
	body, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("cache file missing after run: %v", err)
	}
	var persisted map[string]string
	if err := json.Unmarshal(body, &persisted); err != nil {
		t.Fatalf("cache file invalid JSON: %v", err)
	}
	if persisted["claude"] != "2.1.175" {
		t.Errorf("persisted cache has claude=%q, want 2.1.175 (must update to current version)", persisted["claude"])
	}
}
