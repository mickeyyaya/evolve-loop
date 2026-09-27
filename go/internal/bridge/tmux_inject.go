package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/keyspec"
)

// emitChannelBreadcrumb writes one structured channel marker to w; an empty
// corrID is a no-op so non-correlated injects add no noise.
func emitChannelBreadcrumb(w io.Writer, channel, corrID string) {
	if corrID == "" {
		return
	}
	fmt.Fprintf(w, "{\"evolve_channel\":%q,\"corr_id\":%q}\n", channel, corrID)
}

// channelEnabled reports whether the live bidirectional channel (ADR-0037) is
// on: the channel is implied by the recovery stage (enforce → on; off/shadow
// → off, byte-identical).
func channelEnabled(deps Deps) bool {
	return channel.Enabled(recoveryStageFromEnv(deps))
}

// injectEnvelope delivers one live-injection envelope into the running REPL.
// command/nudge/system_rule are idle-gated and re-queued (bounded by
// maxInjectDefer) when the agent isn't at the prompt; interrupt sends ESC
// first then injects regardless of state; keystroke sends raw tmux key
// tokens with no gating, no ESC prefix, and no Enter suffix.
func injectEnvelope(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch, env inbox.Envelope) string {
	pfx := "[" + lp.name + "]"
	if env.Kind == inbox.KindKeystroke {
		if suspect := keyspec.Validate(env.Body); len(suspect) > 0 {
			fmt.Fprintf(deps.Stderr, "%s keystroke WARN: unrecognized key token(s) %v in %q — sending verbatim\n", pfx, suspect, env.Body)
		}
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
	return env.CorrID
}

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
	_, err := settlePasteThenEnter(ctx, deps, "["+session+"]", session, pasteSettleFor(len(body)), len(body))
	return err
}
