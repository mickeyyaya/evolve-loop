//go:build integration

package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealTmux_AMissingSessionNeverResolvesToALongerName(t *testing.T) {
	requireTmux(t)
	ctx := context.Background()
	gone := itSession("exact")
	longer := gone + "9"
	if err := itTmuxCtl.NewSession(ctx, longer, 80, 24); err != nil {
		t.Fatalf("new-session %s: %v", longer, err)
	}
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(context.Background(), longer) })
	buffer := filepath.Join(t.TempDir(), "buffer.txt")
	mustWrite(t, buffer, "PASTED-INTO-ANOTHER-SESSION")

	if itTmuxCtl.HasSession(ctx, gone) {
		t.Errorf("HasSession(%q) = true while only %q exists", gone, longer)
	}
	for name, read := range map[string]func(context.Context, string) (string, error){
		"pane-command": itTmuxCtl.PaneCommand,
		"pane-tty":     itTmuxCtl.paneTTY,
	} {
		if got, _ := read(ctx, gone); got != "" {
			t.Errorf("%s on missing session %q read %q from %q", name, gone, got, longer)
		}
	}
	for name, op := range map[string]func() error{
		"send-keys":       func() error { return itTmuxCtl.SendKeys(ctx, gone, "", true) },
		"capture-pane":    func() error { _, err := itTmuxCtl.CapturePane(ctx, gone, 0); return err },
		"capture-history": func() error { _, err := itTmuxCtl.CapturePane(ctx, gone, 100); return err },
		"resize-window":   func() error { return itTmuxCtl.JiggleWindow(ctx, gone) },
		"paste-buffer": func() error {
			if err := itTmuxCtl.LoadBuffer(ctx, gone, buffer); err != nil {
				t.Fatalf("load-buffer: %v", err)
			}
			return itTmuxCtl.PasteBuffer(ctx, gone)
		},
	} {
		if err := op(); err == nil {
			t.Errorf("%s on missing session %q succeeded; it reached %q", name, gone, longer)
		}
	}
	if w, _ := itTmuxCtl.run(ctx, "display-message", "-p", "-t", ExactSessionTarget(longer), "#{window_width}"); strings.TrimSpace(w) != "80" {
		t.Errorf("an op on missing %q resized %q to %q", gone, longer, w)
	}
	_ = itTmuxCtl.KillSession(ctx, gone)
	if !itTmuxCtl.HasSession(ctx, longer) {
		t.Fatalf("killing the missing session %q killed %q", gone, longer)
	}
}

func TestExactSessionTarget_NamesOnlyThatSession(t *testing.T) {
	if got := ExactSessionTarget("evolve-bridge-it-x-12"); got != "=evolve-bridge-it-x-12:" {
		t.Fatalf("ExactSessionTarget = %q, want =evolve-bridge-it-x-12:", got)
	}
}
