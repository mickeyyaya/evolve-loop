package looppreflight

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestVersionDrift_MultipleCLIsDriftSimultaneously(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{
		"claude": "2.1.173",
		"codex":  "0.137.0",
	})
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
	detail := c.Detail
	for _, want := range []string{"claude", "2.1.173", "2.1.175", "codex", "0.137.0", "0.139.0"} {
		if !strings.Contains(detail, want) {
			t.Errorf("drift detail missing %q; got %q", want, detail)
		}
	}
}

func TestVersionDrift_CorruptedCacheFailsOpen(t *testing.T) {
	opts := goodPipelineOptions(t)
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

func TestVersionDrift_UpdatedCacheReflectsCurrentInventory(t *testing.T) {
	opts := goodPipelineOptions(t)
	writeCLIVersions(t, opts.EvolveDir, map[string]string{"claude": "2.1.173"})
	opts.VersionInventory = func() map[string]string { return map[string]string{"claude": "2.1.175"} }

	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

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
