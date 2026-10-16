package main

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

func gcSetMode(t *testing.T, evolveDir, mode string) {
	t.Helper()
	pol := `{"gc":{"mode":"` + mode + `","runs":{"keep_full":1,"delete_after_days":1}}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

func gcEnv(t *testing.T) (evolveDir, workspace, keptPath, targetPath string) {
	t.Helper()
	evolveDir = t.TempDir()
	workspace = t.TempDir()
	runs := filepath.Join(evolveDir, "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	keptPath = gcMkRun(t, runs, "cycle-keep", now.Add(-99*24*time.Hour))
	targetPath = gcMkRun(t, runs, "cycle-old", now.Add(-100*24*time.Hour))
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

func TestGCShadow(t *testing.T) {
	t.Parallel()
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
	for _, p := range []string{keptPath, targetPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("shadow mode must not mutate the tree, but %q is gone: %v", p, err)
		}
	}
}

func TestGCOff(t *testing.T) {
	t.Parallel()
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "off")

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

func TestGCEnforce(t *testing.T) {
	t.Parallel()
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

func TestGCInvalidMode(t *testing.T) {
	t.Parallel()
	evolveDir, workspace, keptPath, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "bogus")

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

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

func TestGCShadowMissingRunsDir(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	workspace := t.TempDir()
	pol := `{"gc":{"mode":"shadow"}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loopConfig{EvolveDir: evolveDir, ProjectRoot: filepath.Dir(evolveDir)}
	var buf bytes.Buffer
	runGCHook(cfg, workspace, &buf)

	m, ok := gcReadManifest(t, workspace)
	if !ok {
		t.Fatal("shadow mode must still write a (possibly empty) manifest with no runs/ dir")
	}
	if len(m.Items) != 0 {
		t.Errorf("missing runs/ dir must yield an empty manifest, got %v", m.Items)
	}
}

func TestGCShadowLiveRunExcluded(t *testing.T) {
	t.Parallel()
	evolveDir, workspace, _, targetPath := gcEnv(t)
	gcSetMode(t, evolveDir, "shadow")
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
