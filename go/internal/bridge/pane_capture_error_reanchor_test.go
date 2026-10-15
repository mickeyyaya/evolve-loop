package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type flakyPaneTmux struct {
	fakeTmux
	mu   sync.Mutex
	pane string
	fail bool
}

func (p *flakyPaneTmux) set(pane string, fail bool) {
	p.mu.Lock()
	p.pane, p.fail = pane, fail
	p.mu.Unlock()
}

func (p *flakyPaneTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail {
		return "", errCapture
	}
	return p.pane, nil
}

var errCapture = errors.New("capture-pane: no server running")

func TestRunTmuxREPL_CaptureErrorDoesNotReanchorPaneDelta(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	deps := covDeps()
	deps.RecoveryStage = "enforce"
	tmux := &flakyPaneTmux{pane: "❯ explain tmux\n\n❯\n"}
	deps.Tmux = tmux

	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		switch tick {
		case 1:
			tmux.set(paneThinkingFrame, false)
		case 2:
			tmux.set(paneAnswerFrame, false)
		case 3:
			tmux.set(paneAnswerFrame, true)
		case 4:
			tmux.set(paneAnswerFrame, false)
		case 5:
			_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK (a capture error must not abort the wait)", code)
	}

	body, err := os.ReadFile(filepath.Join(ws, "build-pane.live"))
	if err != nil {
		t.Fatalf("read pane.live: %v", err)
	}
	const answer = "tmux is a terminal multiplexer"
	if n := strings.Count(string(body), answer); n != 1 {
		t.Errorf("answer streamed %d times, want exactly 1 — an errored CapturePane re-anchored the pane delta and the next good frame re-emitted the whole pane; got:\n%s", n, body)
	}
}

func TestRunTmuxREPL_CaptureErrorStillCompletes(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	deps := covDeps()
	deps.RecoveryStage = "enforce"
	tmux := &flakyPaneTmux{pane: "❯ explain tmux\n\n❯\n"}
	deps.Tmux = tmux

	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		switch tick {
		case 1:
			tmux.set(paneThinkingFrame, false)
		case 2, 3:
			tmux.set(paneAnswerFrame, true)
		case 4:
			_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}
	body, _ := os.ReadFile(filepath.Join(ws, "build-pane.live"))
	if strings.Contains(string(body), "tmux is a terminal multiplexer") {
		t.Errorf("an errored capture must contribute no content to pane.live; got:\n%s", body)
	}
}
