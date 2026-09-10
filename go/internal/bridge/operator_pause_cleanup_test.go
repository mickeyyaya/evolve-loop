package bridge

import (
	"context"
	"io"
	"path/filepath"
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
			tmuxCleanup(ctx, Deps{Tmux: tm, Stderr: io.Discard}, "fixture", "owned-session", filepath.Join(t.TempDir(), "capture.txt"), false, 1)
			if !tm.killed {
				t.Fatal("owned provider session survives cleanup after cancellation")
			}
		})
	}
}
