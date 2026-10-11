package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type pauseCleanupController struct {
	FakeTmuxController
	killed bool
}

func (c *pauseCleanupController) HasSession(ctx context.Context, _ string) bool {
	return ctx.Err() == nil
}
func (c *pauseCleanupController) CapturePane(ctx context.Context, _ string, _ int) (string, error) {
	return "", ctx.Err()
}
func (c *pauseCleanupController) KillSession(ctx context.Context, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.killed = true
	return nil
}
func TestOperatorPauseCleanupCancelsProvider(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "live_context_control"
		if canceled {
			name = "canceled_context_must_still_stop_provider"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			tm := &pauseCleanupController{}
			tmuxCleanup(ctx, Deps{Tmux: tm, Stderr: io.Discard}, "fixture", "owned-session", paneCapture{shared: filepath.Join(t.TempDir(), "capture.txt"), lines: 1}, false)
			if !tm.killed {
				t.Fatal("owned provider session survives cleanup after cancellation")
			}
		})
	}
}

func TestTmuxCleanup_CapturesThePaneOfTheDispatchBeforeTheKill(t *testing.T) {
	dir := t.TempDir()
	capture := paneCapture{shared: filepath.Join(dir, "tmux-final-scrollback.txt"), dispatch: filepath.Join(dir, "build-s1-final-pane.txt"), lines: 5}
	tm := &pauseCleanupController{}
	var observed []string
	deps := Deps{Tmux: tm, Stderr: io.Discard, onPaneCapture: func(path string) { observed = append(observed, path) }}

	tmuxCleanup(context.Background(), deps, "fixture", "s1", capture, false)

	body, err := os.ReadFile(capture.dispatch)
	if err != nil || len(observed) != 1 || observed[0] != capture.dispatch || !tm.killed {
		t.Fatalf("capture = %q, %v observed = %v killed = %v; want the dispatch capture, observed once, then the kill", body, err, observed, tm.killed)
	}
}

func TestTmuxCleanup_AFailedDispatchCaptureIsAWarnAndIsNotObserved(t *testing.T) {
	dir := t.TempDir()
	capture := paneCapture{shared: filepath.Join(dir, "tmux-final-scrollback.txt"), dispatch: filepath.Join(dir, "absent", "build-s1-final-pane.txt"), lines: 5}
	var stderr bytes.Buffer
	var observed []string
	deps := Deps{Tmux: &pauseCleanupController{}, Stderr: &stderr, onPaneCapture: func(path string) { observed = append(observed, path) }}

	tmuxCleanup(context.Background(), deps, "fixture", "s1", capture, false)

	if len(observed) != 0 || !strings.Contains(stderr.String(), "WARN the pane capture of the dispatch") || !strings.Contains(stderr.String(), capture.dispatch) {
		t.Fatalf("observed = %v stderr = %q, want no observation and a WARN that names the path", observed, stderr.String())
	}
}

type paneTextController struct {
	pauseCleanupController
	text string
}

func (c *paneTextController) CapturePane(context.Context, string, int) (string, error) {
	return c.text, nil
}

func TestTmuxCleanup_TheDispatchCaptureIsPrivateAndKeepsOnlyTheLastBytes(t *testing.T) {
	cases := []struct {
		name string
		max  int
		text string
		want string
	}{
		{"the policy cap", 6, "first line\nLAST-1", "LAST-1"},
		{"the compiled default keeps a short pane", 0, "short pane", "short pane"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			capture := paneCapture{shared: filepath.Join(dir, "shared.txt"), dispatch: filepath.Join(dir, "build-s1-final-pane.txt"), lines: 5}
			deps := Deps{Tmux: &paneTextController{text: tc.text}, Stderr: io.Discard, PaneCaptureMaxBytes: tc.max}

			tmuxCleanup(context.Background(), deps, "fixture", "s1", capture, false)

			body, err := os.ReadFile(capture.dispatch)
			info, statErr := os.Stat(capture.dispatch)
			if err != nil || statErr != nil || string(body) != tc.want || info.Mode().Perm() != 0o600 {
				t.Fatalf("capture = %q (%v, %v), want %q with mode 0600", body, err, statErr, tc.want)
			}
		})
	}
}
