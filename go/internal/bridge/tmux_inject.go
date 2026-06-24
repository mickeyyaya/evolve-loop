package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/keyspec"
)

// emitChannelBreadcrumb writes one structured channel marker to w. The producer's
// correlator parses these to bracket an injected ask's answer span (ADR-0037).
// Empty corrID is a no-op so non-correlated injects add no noise. The caller
// chooses w: the <agent>-breadcrumbs.live file when the channel is on (enforce),
// else io.Discard (the producer tails the FILE — RT2 moved these off the in-memory
// stderr stream a discarded producer never read).
func emitChannelBreadcrumb(w io.Writer, channel, corrID string) {
	if corrID == "" {
		return
	}
	fmt.Fprintf(w, "{\"evolve_channel\":%q,\"corr_id\":%q}\n", channel, corrID)
}

// channelEnabled reports whether the live bidirectional channel (ADR-0037) is
// on. ADR-0045 I6 folded the rollout into EVOLVE_PHASE_RECOVERY: the channel is
// implied by the stage (enforce → on; off/shadow → off, byte-identical).
// channel.Enabled is the single source for both this driver and the observer adapter.
func channelEnabled(deps Deps) bool {
	return channel.Enabled(recoveryStageFromEnv(deps))
}

// injectEnvelope delivers one live-injection envelope into the running REPL.
// command/nudge/system_rule are idle-gated (injected only when the prompt
// marker is visible); a mid-turn arrival is re-queued, bounded by
// maxInjectDefer. interrupt sends ESC first, then injects regardless of state.
// keystroke sends body as raw tmux key tokens — no ESC prefix, no idle-gate,
// no Enter suffix; the operator owns exactly what reaches the REPL.
//
// It returns the CorrID of a successfully-delivered idle-gated ask (empty
// otherwise: non-correlated body, keystroke/interrupt path, re-queue, or drop).
// Only this function knows the idle-gate passed (vs re-queued/dropped), so the
// non-empty return is the caller's signal in runTmuxREPL to emit inject_applied
// and open the busy→idle span for idle_reached.
func injectEnvelope(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch, env inbox.Envelope) string {
	pfx := "[" + lp.name + "]"
	// Cycle-124 F4 / ADR-0023 addendum: the "full tmux control" hatch the
	// operator asked for. Body is one tmux key-spec (literal text and/or
	// space-separated named keys like "Enter" / "Escape" / "C-c" / "Up" /
	// "y Enter") sent verbatim via SendKeys with enter=false. NO idle-gate
	// (operator may need to send keys precisely BECAUSE the agent isn't
	// idle — e.g. dismissing a modal that hung mid-turn), NO ESC prefix
	// (unlike interrupt), NO automatic Enter append. Empty body is a no-op
	// to match the existing SendKeys contract (line 59 of tmux.go skips
	// empty key strings). The operator is fully responsible for what they
	// inject; the bridge does not interpret the body.
	if env.Kind == inbox.KindKeystroke {
		// Warn-not-block: flag tokens that look like a mistyped key name
		// (e.g. "Excape") so an operator notices the keystroke will be typed
		// verbatim rather than acted on — but NEVER refuse the send (this is
		// the full-control hatch; literal text is a legitimate body).
		if suspect := keyspec.Validate(env.Body); len(suspect) > 0 {
			fmt.Fprintf(deps.Stderr, "%s keystroke WARN: unrecognized key token(s) %v in %q — sending verbatim\n", pfx, suspect, env.Body)
		}
		// Surface a failed send instead of logging success unconditionally
		// (cycle-124 review MEDIUM): a vanished session / killed pane would
		// otherwise show as `injected keystroke "Enter"` on stderr while
		// nothing actually reached the REPL.
		if err := deps.Tmux.SendKeys(ctx, lp.session, env.Body, false); err != nil {
			fmt.Fprintf(deps.Stderr, "%s keystroke send failed: %v (source=%s)\n", pfx, err, env.Source)
			return ""
		}
		fmt.Fprintf(deps.Stderr, "%s injected keystroke %q (source=%s)\n", pfx, env.Body, env.Source)
		return ""
	}
	if env.Kind == inbox.KindInterrupt {
		_ = deps.Tmux.SendKeys(ctx, lp.session, "Escape", false)
		deps.Sleep(injectInterruptSettle)
		_ = injectText(ctx, cfg, deps, lp.session, env.Body) // fire-and-forget live injection
		fmt.Fprintf(deps.Stderr, "%s injected interrupt (source=%s)\n", pfx, env.Source)
		return ""
	}

	// Idle-gated kinds: only inject when the agent is waiting at the prompt.
	pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
	if !strings.Contains(pane, lp.promptMarker) {
		if env.DeferCount >= maxInjectDefer {
			fmt.Fprintf(deps.Stderr, "%s DROP injected %s after %d defers (agent never idled)\n", pfx, env.Kind, env.DeferCount)
			return ""
		}
		env.DeferCount++
		if err := inbox.Append(cfg.Workspace, cfg.Agent, env, deps.Now); err != nil {
			fmt.Fprintf(deps.Stderr, "%s WARN re-queue of %s failed: %v\n", pfx, env.Kind, err)
		}
		return ""
	}

	body := env.Body
	if env.Kind == inbox.KindSystemRule {
		body = "## Rules\n" + body
	}
	_ = injectText(ctx, cfg, deps, lp.session, body) // fire-and-forget live injection
	fmt.Fprintf(deps.Stderr, "%s injected %s (source=%s)\n", pfx, env.Kind, env.Source)
	// Return the CorrID of the just-delivered idle-gated ask so runTmuxREPL can
	// emit the inject_applied breadcrumb (to the channel sink it owns) and open
	// the busy→idle span. Empty CorrID = uncorrelated inject → caller no-ops.
	return env.CorrID
}

// injectText delivers body into the session via the paste buffer (so
// multi-line/special characters survive — SendKeys would mangle them), then
// Enter. It uses a dedicated scratch file so it never collides with the task
// prompt's resolved-prompt.txt. Returns the first transport error so callers
// that gate on delivery (the recipe engine) can surface a dead session
// instead of waiting out a full timeout; the fire-and-forget live-injection
// callers ignore it (preserving prior behavior).
func injectText(ctx context.Context, cfg *Config, deps Deps, session, body string) error {
	scratch := filepath.Join(cfg.Workspace, ".bridge-inbox", orDefault(cfg.Agent, "agent")+"-inject.txt")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		fmt.Fprintf(deps.Stderr, "[%s] WARN inject scratch mkdir: %v\n", session, err)
		return fmt.Errorf("inject scratch mkdir: %w", err)
	}
	if err := os.WriteFile(scratch, []byte(body), 0o644); err != nil {
		fmt.Fprintf(deps.Stderr, "[%s] WARN inject scratch write: %v\n", session, err)
		return fmt.Errorf("inject scratch write: %w", err)
	}
	if err := deps.Tmux.LoadBuffer(ctx, session, scratch); err != nil {
		return fmt.Errorf("inject load-buffer: %w", err)
	}
	if err := deps.Tmux.PasteBuffer(ctx, session); err != nil {
		return fmt.Errorf("inject paste-buffer: %w", err)
	}
	deps.Sleep(time.Second)
	return deps.Tmux.SendKeys(ctx, session, "", true) // Enter
}
