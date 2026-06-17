package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// modelPickerMarkers are substrings that indicate a CLI's /model picker is
// open: codex "Select Model", agy "Switch Model", claude "Select model".
var modelPickerMarkers = []string{"Select Model", "Switch Model", "Select model"}

// modelPickerPollTicks bounds each poll (seconds) for the picker to render.
const modelPickerPollTicks = 6

// CaptureModelPicker launches-or-attaches the CLI's REPL, opens its interactive
// /model picker, captures the pane, and dismisses the picker with Esc WITHOUT
// confirming — so the live model is never changed. The returned pane is parsed
// by modelquery's per-CLI picker parsers. An ephemeral session is killed after
// capture so a daily refresh does not leak live REPL processes.
//
// Safety: the picker is opened by `/model` + Enter (the slash-command idiom).
// codex intercepts that first Enter with a slash autocomplete and needs ONE
// more Enter to open the picker; that extra Enter is sent ONLY while the picker
// is not yet open, so a CLI whose picker opened immediately is never
// accidentally confirmed. The final keystroke is always Esc.
func CaptureModelPicker(ctx context.Context, cfg *Config, deps Deps, cli string) (pane string, err error) {
	deps = deps.withDefaults()
	// I1 NOTE: the model-query probe is intentionally NOT scratch-cwd'd here. Its
	// Workspace can resolve to the live checkout (liveRefresh falls back to
	// os.Getwd() when unset, cmd_models_live.go:88), so a scratch dir would land
	// IN main. The correct fix is to give that path a temp Workspace first; until
	// then it keeps the recipe os.Getwd() fallback (pre-existing, no regression).
	drv, _, derr := newRecipeDriver(cfg, deps, cli)
	if derr != nil {
		return "", derr
	}
	if cfg.SessionName == "" { // ephemeral session — reap the live REPL afterwards
		// Detached context: the session must be reaped even if the caller's ctx
		// was already cancelled (deadline/parent cancel) by the time we return.
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
		// codex's slash-autocomplete swallowed the first Enter; one more opens it.
		// A SendKeys error here means the session died — fail fast (Esc-dismissing
		// first) rather than burning another full poll on a dead session.
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

// pollForModelPicker polls the pane up to modelPickerPollTicks times, returning
// the last captured pane and whether a picker marker appeared.
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
