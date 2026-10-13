package versionbump

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBumpCodexPlugin_MalformedManifestFailsAndAnAlreadyCurrentOneIsASilentNoOp(t *testing.T) {
	dir := t.TempDir()
	malformed := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(malformed, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if mod, err := bumpCodexPlugin(malformed, "2.0.0", false); err == nil || mod != "" {
		t.Fatalf("a manifest that cannot be read must fail loudly: mod=%q err=%v", mod, err)
	}

	current := filepath.Join(dir, "current.json")
	if err := os.WriteFile(current, []byte(`{"name":"evo","version": "2.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if mod, err := bumpCodexPlugin(current, "2.0.0", false); err != nil || mod != "" {
		t.Fatalf("already at the target: mod=%q err=%v", mod, err)
	}

	stale := filepath.Join(dir, "stale.json")
	if err := os.WriteFile(stale, []byte(`{"name":"evo","version": "1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mod, err := bumpCodexPlugin(stale, "2.0.0", false)
	if err != nil || mod != ".codex-plugin/plugin.json" {
		t.Fatalf("stale: mod=%q err=%v", mod, err)
	}
	if body, _ := os.ReadFile(stale); !strings.Contains(string(body), `"version": "2.0.0"`) {
		t.Errorf("manifest = %s", body)
	}
	if mod, err := bumpCodexPlugin("", "2.0.0", false); err != nil || mod != "" {
		t.Errorf("no path means no mirror: mod=%q err=%v", mod, err)
	}
}
