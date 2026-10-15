package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

var modelPickerMarkers = []string{"Select Model", "Switch Model", "Select model"}

const modelPickerPollTicks = 6

func CaptureModelPicker(ctx context.Context, cfg *Config, deps Deps, cli string) (pane string, err error) {
	deps = deps.withDefaults()
	drv, _, derr := newRecipeDriver(cfg, deps, cli)
	if derr != nil {
		return "", derr
	}
	if cfg.SessionName == "" {
		defer func() { _ = killSessionSwept(context.Background(), deps, drv.session) }()
	}
	if serr := drv.EnsureSession(ctx); serr != nil {
		return "", fmt.Errorf("recipe: ensure session: %w", serr)
	}
	if serr := drv.SendCommand(ctx, "/model"); serr != nil {
		return "", serr
	}

	pane, opened := pollForModelPicker(ctx, drv, deps)
	if !opened {
		if kerr := drv.SendKeys(ctx, "Enter"); kerr != nil {
			_ = drv.SendKeys(ctx, "Escape")
			return pane, fmt.Errorf("recipe: /model extra-enter for %s: %w", cli, kerr)
		}
		pane, opened = pollForModelPicker(ctx, drv, deps)
	}
	_ = drv.SendKeys(ctx, "Escape")

	if !opened {
		return pane, fmt.Errorf("recipe: /model picker for %s did not open", cli)
	}
	return pane, nil
}

func pollForModelPicker(ctx context.Context, drv *recipeSessionDriver, deps Deps) (pane string, opened bool) {
	for i := 0; i < modelPickerPollTicks; i++ {
		deps.Sleep(time.Second)
		if drv.ar != nil {
			drv.ar.tick(ctx, drv.session)
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
