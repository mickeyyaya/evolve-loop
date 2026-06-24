package main

// cmd_loop_gc_test.go — RED white-box tests for the L3.4 GC loop hook
// (task gc-shadow-wiring, cycle 298). These call the unexported runGCHook
// directly so they exercise the real Discover→Plan→(Apply) wiring against a
// synthetic .evolve tree and assert on observable side effects (manifest file
// written / not written, run dirs mutated / preserved, live runs excluded).
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - Add `func runGCHook(cfg loopConfig, workspace string, stderr io.Writer)`
//     to cmd_loop.go. It reads os.Getenv("EVOLVE_GC") (default "off"):
//       off            → return immediately, no manifest.
//       <invalid>      → log a warning to stderr, return; no manifest, no crash.
//       shadow         → policy.Load + gc.Discover + gc.Plan; write
//                        <workspace>/gc-shadow-manifest.json (valid JSON,
//                        decodes to gc.Manifest); NO filesystem mutations.
//       enforce        → shadow + gc.Apply (run dirs actually archived/deleted).
//   - The hook is fail-open: a missing runs/ dir yields an empty manifest, no
//     error, no crash.
//
// These tests are currently RED: runGCHook does not exist, so package main
// fails to compile. They turn GREEN when Builder adds the hook.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

const gcManifestName = "gc-shadow-manifest.json"

// gcSetMode overwrites policy.json in evolveDir to set gc.mode, preserving the
// aggressive runs retention policy written by gcEnv. Called after gcEnv to set
// a non-default mode (the default, empty Mode, is treated as "off").
func gcSetMode(t *testing.T, evolveDir, mode string) {
	t.Helper()
	pol := `{"gc":{"mode":"` + mode + `","runs":{"keep_full":1,"delete_after_days":1}}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gcMkRun creates a discovered run dir (run.json marker) with a set mtime.
func gcMkRun(t *testing.T, runsDir, name string, mod time.Time) string {
	t.Helper()
	p := filepath.Join(runsDir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "run.json"), []byte(`{"cycle_id":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
	return p
}

// gcEnv builds a synthetic .evolve dir with two dead run dirs and an aggressive
// policy (keep_full=1, delete_after_days=1) so the OLDER run is a delete target
// and the NEWER run is protected by keep_full. Returns (evolveDir, workspace,
// keptPath, targetPath). Times are anchored to real wall clock because the hook
// uses time.Now() internally (no injected clock).
func gcEnv(t *testing.T) (evolveDir, workspace, keptPath, targetPath string) {
	t.Helper()
	evolveDir = t.TempDir()
	workspace = t.TempDir()
	runs := filepath.Join(evolveDir, "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	keptPath = gcMkRun(t, runs, "cycle-keep", now.Add(-99*24*time.Hour))   // newest → kept (i<keep_full)
	targetPath = gcMkRun(t, runs, "cycle-old", now.Add(-100*24*time.Hour)) // oldest → delete target
	pol := `{"gc":{"runs":{"keep_full":1,"delete_after_days":1}}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func gcReadManifest(t *testing.T, workspace string) (gc.Manifest, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspace, gcManifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return gc.Manifest{}, false
		}
		t.Fatal(err)
	}
	var m gc.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("gc-shadow-manifest.json is not valid JSON / not a Manifest: %v", err)
	}
	return m, true
}

func gcManifestHasPath(m gc.Manifest, path string) bool {
	for _, it := range m.Items {
		if it.Path == path {
			return true
		}
	}
	return false
}

// TestGCShadow: shadow mode writes a valid manifest naming the delete target
// but performs NO filesystem mutation (the target dir survives).
func TestGCShadow(t *testing.T) {
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "shadow")

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

	m, ok := gcReadManifest(t, workspace)
	if !ok {
		t.Fatal("shadow mode must write gc-shadow-manifest.json")
	}
	if !gcManifestHasPath(m, targetPath) {
		t.Errorf("manifest must plan the old run %q for deletion; items=%v", targetPath, m.Items)
	}
	// No mutation: both run dirs must still exist on disk.
	for _, p := range []string{keptPath, targetPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("shadow mode must not mutate the tree, but %q is gone: %v", p, err)
		}
	}
}

// TestGCOff: off mode (and unset) writes no manifest and touches nothing.
func TestGCOff(t *testing.T) {
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	// policy.json has no gc.mode → defaults to "off"

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

	if _, ok := gcReadManifest(t, workspace); ok {
		t.Error("off mode must NOT write gc-shadow-manifest.json")
	}
	for _, p := range []string{keptPath, targetPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("off mode must not mutate the tree, but %q is gone: %v", p, err)
		}
	}
}

// TestGCEnforce: enforce mode writes the manifest AND applies it — the delete
// target is actually removed while the kept run survives.
func TestGCEnforce(t *testing.T) {
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "enforce")

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

	if _, ok := gcReadManifest(t, workspace); !ok {
		t.Error("enforce mode must also write gc-shadow-manifest.json")
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Errorf("enforce mode must DELETE the old run %q (err=%v)", targetPath, err)
	}
	if _, err := os.Stat(keptPath); err != nil {
		t.Errorf("enforce mode must preserve the kept run %q: %v", keptPath, err)
	}
}

// TestGCInvalidMode: an unrecognized value warns and returns — no manifest, no
// mutation, no panic.
func TestGCInvalidMode(t *testing.T) {
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "bogus")

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf) // must not panic

	if _, ok := gcReadManifest(t, workspace); ok {
		t.Error("invalid mode must NOT write a manifest")
	}
	for _, p := range []string{keptPath, targetPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("invalid mode must not mutate the tree, but %q is gone: %v", p, err)
		}
	}
	if !strings.Contains(strings.ToLower(buf.String()), "gc") {
		t.Errorf("invalid mode should log a warning mentioning gc; stderr=%q", buf.String())
	}
}

// TestGCShadowMissingRunsDir: shadow mode against an .evolve with no runs/ dir
// fails open — empty manifest, no error, no crash.
func TestGCShadowMissingRunsDir(t *testing.T) {
	evolveDir := t.TempDir() // deliberately no runs/ subdir
	workspace := t.TempDir()
	pol := `{"gc":{"mode":"shadow"}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf) // must not panic

	m, ok := gcReadManifest(t, workspace)
	if !ok {
		t.Fatal("shadow mode must still write a (possibly empty) manifest with no runs/ dir")
	}
	if len(m.Items) != 0 {
		t.Errorf("missing runs/ dir must yield an empty manifest, got %v", m.Items)
	}
}

// TestGCShadowLiveRunExcluded: a run that the host-global cycle-state.json names
// as the in-flight workspace is LIVE and must never appear in the manifest, even
// when it is old enough to be a delete target.
func TestGCShadowLiveRunExcluded(t *testing.T) {
	evolveDir, workspace, _, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "shadow")
	// Mark the would-be delete target LIVE via cycle-state.json.
	state := `{"cycle_id":298,"workspace_path":"` + targetPath + `"}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

	m, ok := gcReadManifest(t, workspace)
	if !ok {
		t.Fatal("shadow mode must write a manifest")
	}
	if gcManifestHasPath(m, targetPath) {
		t.Errorf("a LIVE run %q must never be planned for deletion; items=%v", targetPath, m.Items)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("the live run %q must remain on disk: %v", targetPath, err)
	}
}
