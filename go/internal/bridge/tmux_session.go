package bridge

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

// resolveSession returns the tmux session name and whether it is a stable
// named (resume-eligible) session. ephemeralPrefix distinguishes drivers
// (evolve-bridge- / evolve-bridge-codex- / evolve-bridge-agy-). Named
// sessions (claude-tmux only) use evolve-bridge-named-<name>.
func resolveSession(cfg *Config, deps Deps, ephemeralPrefix string) (session string, named bool) {
	if cfg.SessionName != "" {
		return NamedSessionName(cfg.SessionName), true
	}
	agent := orDefault(cfg.Agent, "probe")
	runTok := runscope.New("", cfg.RunID, cfg.Cycle).SessionPrefix()
	nonce := strconv.FormatUint(ephemeralSessionNonce.Add(1), 36)
	s := fmt.Sprintf("%s%sc%d-%s-pid%d-n%s-%d", ephemeralPrefix, runTok, cfg.Cycle, agent, os.Getpid(), nonce, deps.Now().Unix())
	return truncate64(s), false
}

var ephemeralSessionNonce atomic.Uint64

func truncate64(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

// NamedSessionName returns the tmux session name for a swarm-controlled named
// session. It is the single source of truth shared by resolveSession (which
// creates the session) and the swarm reaper (which kills it by this name).
// Format: "evolve-bridge-named-<name>", truncated to 64 characters.
func NamedSessionName(name string) string {
	return truncate64("evolve-bridge-named-" + name)
}

// parseExtendSecs parses an "extend:<secs>" auto-respond action.
func parseExtendSecs(action string) int {
	const p = "extend:"
	if !strings.HasPrefix(action, p) {
		return 0
	}
	n := 0
	for _, c := range action[len(p):] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// recoverBlankPane handles a render wedge: an EMPTY capture while the
// session is alive is the renderer wedging, not idleness. Recovery: jiggle
// the window (two SIGWINCHes → full repaint, windowJiggler optional
// capability) and re-read. Returns the freshest pane and whether the wedge
// persisted — a still-blank pane must read BUSY, never pause a live agent
// on a pane that stopped rendering (the maxExtends backstop still bounds it).
func recoverBlankPane(ctx context.Context, deps Deps, session string, scrollback int, pane, pfx string) (string, bool) {
	if strings.TrimSpace(pane) != "" || !deps.Tmux.HasSession(ctx, session) {
		return pane, false
	}
	if j, ok := deps.Tmux.(windowJiggler); ok {
		_ = j.JiggleWindow(ctx, session)
		deps.Sleep(time.Second)
	}
	if re, err := deps.Tmux.CapturePane(ctx, session, scrollback); err == nil && strings.TrimSpace(re) != "" {
		fmt.Fprintf(deps.Stderr, "%s render wedge: blank pane redrawn after jiggle\n", pfx)
		return re, false
	}
	fmt.Fprintf(deps.Stderr, "%s render wedge: pane still blank after jiggle — treating live session as busy\n", pfx)
	return pane, true
}

// tmuxCleanupTimeout bounds the detached cleanup context: a wedged tmux must
// delay teardown by at most this, never leave the provider running.
const tmuxCleanupTimeout = 15 * time.Second

// tmuxCleanup captures final scrollback then kills the session, unless it
// is a named session (preserved for resume). It runs on a context detached
// from the launch context's cancellation: the deferred cleanup fires exactly
// when the launch context is canceled, and a canceled context would make
// every tmux command below refuse to run, leaking the session.
func tmuxCleanup(ctx context.Context, deps Deps, name, session, scrollbackFile string, named bool, scrollback int) {
	pfx := "[" + name + "]"
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tmuxCleanupTimeout)
	defer cancel()
	if !deps.Tmux.HasSession(ctx, session) {
		return
	}
	if raw, err := deps.Tmux.CapturePane(ctx, session, scrollback); err == nil {
		_ = os.WriteFile(scrollbackFile, []byte(raw), 0o644)
	}
	if named {
		fmt.Fprintf(deps.Stderr, "%s session PRESERVED for resume: %s\n", pfx, session)
		return
	}
	_ = deps.Tmux.KillSession(ctx, session)
	fmt.Fprintf(deps.Stderr, "%s session killed: %s\n", pfx, session)
}
