package bridge

import (
	"context"
	"strings"
	"testing"
)

// namedCaptureCfg builds a Config bound to an already-existing named session so
// EnsureSession attaches immediately (no boot loop) and the capture flow is the
// unit under test.
func namedCaptureCfg(t *testing.T, name string) *Config {
	t.Helper()
	return &Config{
		CLI: "claude-tmux", Workspace: t.TempDir(), Agent: "models",
		SessionName: name, Realization: RealizeFor("claude-tmux", LaunchIntent{}),
	}
}

func TestCaptureModelPicker_OpensImmediately(t *testing.T) {
	picker := "Select model\n1. Default  Opus 4.8\n2. Sonnet  Sonnet 4.6\n3. Haiku  Haiku 4.5"
	sess := NamedSessionName("mp1")
	tx := &fakeTmux{existing: map[string]bool{sess: true}, paneSeq: []string{picker}}
	cfg := namedCaptureCfg(t, "mp1")

	pane, err := CaptureModelPicker(context.Background(), cfg, recipeDeps(tx), "claude-tmux")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(pane, "Select model") {
		t.Fatalf("pane missing picker: %q", pane)
	}
	// Safety: the picker is always dismissed with Esc (never confirmed).
	if !tx.sentContains("Escape") {
		t.Fatalf("Escape never sent; sends=%v", tx.sentKeys)
	}
}

func TestCaptureModelPicker_NeverOpensErrors(t *testing.T) {
	sess := NamedSessionName("mp2")
	tx := &fakeTmux{existing: map[string]bool{sess: true}, paneSeq: []string{"just a prompt ❯"}}
	cfg := namedCaptureCfg(t, "mp2")

	if _, err := CaptureModelPicker(context.Background(), cfg, recipeDeps(tx), "claude-tmux"); err == nil {
		t.Fatal("expected error when picker never opens")
	}
	// Even on the failure path the picker attempt must be dismissed.
	if !tx.sentContains("Escape") {
		t.Fatalf("Escape not sent on failure path; sends=%v", tx.sentKeys)
	}
}

func TestCaptureModelPicker_UnknownCLI(t *testing.T) {
	cfg := &Config{Workspace: t.TempDir()}
	if _, err := CaptureModelPicker(context.Background(), cfg, recipeDeps(&fakeTmux{}), "no-such-cli"); err == nil {
		t.Fatal("expected manifest error for unknown CLI")
	}
}

func TestContainsAnySubstring(t *testing.T) {
	if !containsAnySubstring("foo Switch Model bar", modelPickerMarkers) {
		t.Fatal("should match agy marker")
	}
	if containsAnySubstring("no markers here", modelPickerMarkers) {
		t.Fatal("should not match")
	}
}
