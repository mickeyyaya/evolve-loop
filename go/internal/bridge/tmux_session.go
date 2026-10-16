package bridge

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

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

func NamedSessionName(name string) string {
	return truncate64("evolve-bridge-named-" + name)
}

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

const tmuxCleanupTimeout = 15 * time.Second

type paneCapture struct {
	shared, dispatch string
	lines            int
}

func tmuxCleanup(ctx context.Context, deps Deps, name, session string, capture paneCapture, named bool) {
	pfx := "[" + name + "]"
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tmuxCleanupTimeout)
	defer cancel()
	if !deps.Tmux.HasSession(ctx, session) {
		return
	}
	if raw, err := deps.Tmux.CapturePane(ctx, session, capture.lines); err == nil {
		_ = os.WriteFile(capture.shared, []byte(raw), 0o644)
		captureDispatchPane(deps, pfx, capture.dispatch, lastBytes(raw, defaultIfZero(deps.PaneCaptureMaxBytes, attemptpostmortem.DefaultConfig().MaxPaneCaptureBytes)))
	}
	if named {
		deps.sweep.preserve()
		fmt.Fprintf(deps.Stderr, "%s session PRESERVED for resume: %s\n", pfx, session)
		return
	}
	_ = killSessionSwept(ctx, deps, session)
	fmt.Fprintf(deps.Stderr, "%s session killed: %s\n", pfx, session)
}

func captureDispatchPane(deps Deps, pfx, path, raw string) {
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		fmt.Fprintf(deps.Stderr, "%s WARN the pane capture of the dispatch was not written: %s: %v\n", pfx, path, err)
		return
	}
	observePaneCapture(deps, path)
}

func lastBytes(raw string, limit int) string {
	return raw[max(0, len(raw)-limit):]
}

func observePaneCapture(deps Deps, path string) {
	if deps.onPaneCapture != nil {
		deps.onPaneCapture(path)
	}
}

func observeDispatchID(deps Deps, id string) {
	if deps.onDispatchID != nil {
		deps.onDispatchID(id)
	}
}
