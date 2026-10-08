package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestRunBridgeIntrospect_ParsesEveryFlagForAPaneFile(t *testing.T) {
	pane := filepath.Join(t.TempDir(), "pane.txt")
	if err := os.WriteFile(pane, []byte("/help\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runBridgeIntrospect([]string{"--cli=claude-tmux", "--pane-file=" + pane, "--workspace=" + t.TempDir(), "--session-name=s1", "--allow-bypass"}, &stdout, &stderr)

	if rc != 3 || !strings.Contains(stdout.String(), "{") || strings.Contains(stderr.String(), "unknown flag") {
		t.Errorf("rc=%d stdout=%q stderr=%q, want a drift report from the pane file and every flag accepted", rc, stdout.String(), stderr.String())
	}
}

func TestRunBridgeIntrospect_ALiveCaptureThatCannotStartIsAnError(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer

	rc := runBridgeIntrospect([]string{"--cli=claude-tmux", "--workspace=" + t.TempDir()}, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "evolve bridge introspect: recipe: ") || stdout.Len() != 0 {
		t.Errorf("rc=%d stdout=%q stderr=%q, want exit 1 and the refused session start (fleet mode needs a worktree)", rc, stdout.String(), stderr.String())
	}
}

func TestRunBridgeControl_ResolvesTheFamilyBeforeAnySession(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	ws := t.TempDir()
	cases := []struct {
		family string
		want   int
		text   string
	}{
		{"nosuchfamily", 1, "clicontrol: resolve nosuchfamily-tmux"},
		{"claude", 3, `claude does not support event "teleport"`},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		rc := runBridgeControl([]string{tc.family, "teleport", "--workspace=" + ws, "--allow-bypass"}, &stdout, &stderr)
		if rc != tc.want || !strings.Contains(stderr.String(), tc.text) {
			t.Errorf("control %s: rc=%d stderr=%q, want rc %d and %q", tc.family, rc, stderr.String(), tc.want, tc.text)
		}
	}
}
