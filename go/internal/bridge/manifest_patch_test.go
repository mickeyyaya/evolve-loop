package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func useBridgeManifestDir(t *testing.T, dir string) {
	t.Helper()
	orig := bridgeManifestDirFn
	bridgeManifestDirFn = func() string { return dir }
	t.Cleanup(func() { bridgeManifestDirFn = orig })
}

func TestAddRule_ErrorPaths(t *testing.T) {
	// MkdirAll error: override dir is under a regular file.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	useBridgeManifestDir(t, filepath.Join(blocker, "sub"))
	if _, err := AddRule("claude-p", ManifestPrompt{Name: "a", Regex: "r", Policy: "escalate"}); err == nil {
		t.Fatal("AddRule should fail when the override dir can't be created")
	}

	// WriteFile error: target manifest path is a directory.
	d := t.TempDir()
	useBridgeManifestDir(t, d)
	if err := os.Mkdir(filepath.Join(d, "claude-p.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AddRule("claude-p", ManifestPrompt{Name: "b", Regex: "r", Policy: "escalate"}); err == nil {
		t.Fatal("AddRule should fail when the manifest path is a directory")
	}

	// MarshalIndent error: swap the seam.
	d2 := t.TempDir()
	useBridgeManifestDir(t, d2)
	orig := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal boom") }
	defer func() { marshalIndent = orig }()
	if _, err := AddRule("claude-p", ManifestPrompt{Name: "c", Regex: "r", Policy: "escalate"}); err == nil {
		t.Fatal("AddRule should propagate a marshal error")
	}
}

func TestAppendInteractiveRule(t *testing.T) {
	base := []ManifestPrompt{{Name: "existing", Regex: "x", Policy: "escalate"}}
	bad := []ManifestPrompt{
		{},                                       // missing all
		{Name: "n", Regex: "r", Policy: "bogus"}, // bad policy
		{Name: "n", Regex: "r", Policy: "auto_respond"},    // auto_respond w/o keys
		{Name: "existing", Regex: "r", Policy: "escalate"}, // duplicate name
	}
	for i, rule := range bad {
		if _, err := AppendInteractiveRule(base, rule); err == nil {
			t.Fatalf("bad rule[%d] %+v should error", i, rule)
		}
	}
	if out, err := AppendInteractiveRule(base, ManifestPrompt{Name: "new", Regex: "r", Policy: "escalate"}); err != nil || len(out) != 2 {
		t.Fatalf("escalate add: out=%d err=%v", len(out), err)
	}
	if out, err := AppendInteractiveRule(base, ManifestPrompt{Name: "n2", Regex: "r", Policy: "auto_respond", ResponseKeys: "y,Enter"}); err != nil || len(out) != 2 {
		t.Fatalf("auto_respond add: out=%d err=%v", len(out), err)
	}
}

func TestAddRule_RoundTrip(t *testing.T) {
	useBridgeManifestDir(t, t.TempDir())
	rule := ManifestPrompt{Name: "covrule", Regex: "WIDGET", ResponseKeys: "y,Enter", Policy: "auto_respond", Note: "test"}

	path, err := AddRule("claude-p", rule) // loads embedded, appends, writes override
	if err != nil {
		t.Fatalf("AddRule err: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("override manifest not written: %v", err)
	}
	// LoadManifest now reads the override and sees the new rule.
	m, err := LoadManifest("claude-p")
	if err != nil {
		t.Fatalf("LoadManifest err: %v", err)
	}
	found := false
	for _, p := range m.InteractivePrompts {
		if p.Name == "covrule" {
			found = true
		}
	}
	if !found {
		t.Fatal("override manifest should contain the added rule")
	}
	// Re-adding the same name now collides (it's in the override).
	if _, err := AddRule("claude-p", rule); err == nil {
		t.Fatal("duplicate AddRule should error")
	}
}

func TestAddRule_BadCLI(t *testing.T) {
	useBridgeManifestDir(t, t.TempDir())
	if _, err := AddRule("no-such-cli", ManifestPrompt{Name: "n", Regex: "r", Policy: "escalate"}); err == nil {
		t.Fatal("AddRule for an unknown cli should error")
	}
}
