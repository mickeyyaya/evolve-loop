package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// modelPickerMarkers show an open /model picker: codex "Select Model", agy "Switch Model", claude "Select model".
var modelPickerMarkers = []string{"Select Model", "Switch Model", "Select model"}

// modelPickerPollTicks is how many one-second polls wait for the picker to render.
const modelPickerPollTicks = 6

// CaptureModelPicker captures the CLI's /model picker pane and dismisses it with Esc, so the live model never changes.
func CaptureModelPicker(ctx context.Context, cfg *Config, deps Deps, cli string) (pane string, err error) {
	deps = deps.withDefaults()
	// cfg.Workspace only anchors diagnostic writes. The session cwd is cfg.Worktree, left unset on purpose:
	// a picker capture needs no cwd guarantee.
	drv, _, derr := newRecipeDriver(cfg, deps, cli)
	if derr != nil {
		return "", derr
	}
	if cfg.SessionName == "" { // ephemeral session — reap the live REPL afterwards
		// Detached context: the session is reaped even when the caller's ctx is already cancelled.
		defer func() { _ = deps.Tmux.KillSession(context.Background(), drv.session) }()
	}
	if serr := drv.EnsureSession(ctx); serr != nil {
		return "", fmt.Errorf("recipe: ensure session: %w", serr)
	}
	if serr := drv.SendCommand(ctx, "/model"); serr != nil {
		return "", serr
	}

	pane, opened := pollForModelPicker(ctx, drv, deps)
	if !opened {
		// codex's slash autocomplete swallows the first Enter, so one more opens the picker; it is sent only while
		// the picker is closed, never to confirm. A SendKeys error means the session died: dismiss and fail fast.
		if kerr := drv.SendKeys(ctx, "Enter"); kerr != nil {
			_ = drv.SendKeys(ctx, "Escape")
			return pane, fmt.Errorf("recipe: /model extra-enter for %s: %w", cli, kerr)
		}
		pane, opened = pollForModelPicker(ctx, drv, deps)
	}
	_ = drv.SendKeys(ctx, "Escape") // dismiss without confirming → model unchanged

	if !opened {
		return pane, fmt.Errorf("recipe: /model picker for %s did not open", cli)
	}
	return pane, nil
}

func pollForModelPicker(ctx context.Context, drv *recipeSessionDriver, deps Deps) (pane string, opened bool) {
	for i := 0; i < modelPickerPollTicks; i++ {
		deps.Sleep(time.Second)
		if drv.ar != nil {
			drv.ar.tick(ctx, drv.session) // dismiss a boot modal; harmless otherwise
		}
		pane, _ = drv.Capture(ctx)
		if containsAnySubstring(pane, modelPickerMarkers) {
			return pane, true
		}
	}
	return pane, false
}

func containsAnySubstring(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
