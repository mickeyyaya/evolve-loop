package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

type paneScriptTmux struct {
	fakeTmux
	mu   sync.Mutex
	pane string
}

func (p *paneScriptTmux) setPane(s string) {
	p.mu.Lock()
	p.pane = s
	p.mu.Unlock()
}

func (p *paneScriptTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pane, nil
}

const (
	paneThinkingFrame = "❯ explain tmux\n" +
		"\n" +
		"✻ Thinking…\n" +
		"\n" +
		"❯\n" +
		"  ⏵⏵ bypass permissions on\n"
	paneAnswerFrame = "❯ explain tmux\n" +
		"\n" +
		"⏺ tmux is a terminal multiplexer\n" +
		"\n" +
		"❯\n" +
		"  ⏵⏵ bypass permissions on\n"
)

func paneLiveCfg(t *testing.T, ws string) *Config {
	t.Helper()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	return &Config{
		Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact:  filepath.Join(ws, "a"),
		StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e"),
	}
}

func TestRunTmuxREPL_ChannelOff_NoLiveFiles(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	deps := covDeps()
	tmux := &paneScriptTmux{pane: "❯"}
	deps.Tmux = tmux

	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		if tick == 1 {
			_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}

	for _, name := range []string{"build-pane.live", "build-breadcrumbs.live"} {
		if _, err := os.Stat(filepath.Join(ws, name)); !os.IsNotExist(err) {
			t.Errorf("channel off must not create %s (err=%v)", name, err)
		}
	}
}

func TestRunTmuxREPL_ChannelOn_StreamsPaneDeltaToFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	deps := covDeps()
	deps.RecoveryStage = "enforce"
	tmux := &paneScriptTmux{pane: "❯ explain tmux\n\n❯\n"}
	deps.Tmux = tmux

	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		switch tick {
		case 1:
			tmux.setPane(paneThinkingFrame)
		case 2:
			tmux.setPane(paneAnswerFrame)
		case 3:
			_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK", code)
	}

	body, err := os.ReadFile(filepath.Join(ws, "build-pane.live"))
	if err != nil {
		t.Fatalf("read pane.live: %v", err)
	}
	if !strings.Contains(string(body), "tmux is a terminal multiplexer") {
		t.Errorf("pane.live missing streamed answer; got:\n%s", body)
	}
	if strings.Contains(string(body), "bypass permissions") {
		t.Errorf("pane.live leaked volatile footer; got:\n%s", body)
	}
}

func TestRunTmuxREPL_ChannelOn_BreadcrumbsToFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	stderr := &bytes.Buffer{}
	deps := covDeps()
	deps.Stderr = stderr
	deps.RecoveryStage = "enforce"
	bcPath := filepath.Join(ws, "build-breadcrumbs.live")

	tmux := &paneScriptTmux{pane: "❯"}
	deps.Tmux = tmux

	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		switch tick {
		case 1:
			_ = inbox.Append(ws, "build", inbox.Envelope{
				Kind: inbox.KindCommand, Body: "answer me", CorrID: "cX", Source: "supervisor",
			}, fixedTime)
		case 2:
			tmux.setPane("❯ q\n\n✽ Inferring…\n\n❯\n  ⏵⏵ bypass permissions on · esc to interrupt")
		case 3:
			tmux.setPane("❯ q\n\n⏺ done\n\n❯\n  ⏵⏵ bypass permissions on · ← for agents")
		default:
			if _, err := os.Stat(cfg.Artifact); err != nil {
				if b, _ := os.ReadFile(bcPath); strings.Contains(string(b), "idle_reached") {
					_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
				}
			}
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK; stderr:\n%s", code, stderr.String())
	}

	b, err := os.ReadFile(bcPath)
	if err != nil {
		t.Fatalf("read breadcrumbs.live: %v", err)
	}
	out := string(b)
	wantInject := `{"evolve_channel":"inject_applied","corr_id":"cX"}`
	wantIdle := `{"evolve_channel":"idle_reached","corr_id":"cX"}`
	iInject := strings.Index(out, wantInject)
	iIdle := strings.Index(out, wantIdle)
	if iInject < 0 || iIdle < 0 {
		t.Fatalf("breadcrumbs file missing markers; got:\n%s", out)
	}
	if iInject >= iIdle {
		t.Errorf("inject_applied must precede idle_reached; inject@%d idle@%d", iInject, iIdle)
	}
	if n := strings.Count(out, wantIdle); n != 1 {
		t.Errorf("idle_reached emitted %d times, want exactly 1", n)
	}
	if strings.Contains(stderr.String(), "evolve_channel") {
		t.Errorf("breadcrumb leaked to stderr:\n%s", stderr.String())
	}
}

func TestRunTmuxREPL_ChannelOn_LiveFileOpenError_WARN(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	cfg.Agent = "missing/sub"
	stderr := &bytes.Buffer{}
	deps := covDeps()
	deps.Stderr = stderr
	deps.RecoveryStage = "enforce"
	tmux := &paneScriptTmux{pane: "❯"}
	deps.Tmux = tmux

	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return
		}
		tick++
		if tick == 1 {
			_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK (channel open error must not abort)", code)
	}
	if !strings.Contains(stderr.String(), "WARN channel") {
		t.Errorf("expected a WARN for the unopenable .live file; stderr:\n%s", stderr.String())
	}
}

func TestPaneProfileFor(t *testing.T) {
	t.Parallel()
	if p := paneProfileFor(tmuxLaunch{name: "claude-tmux", promptMarker: "❯"}); p.Name != panestream.FormatClaudeREPL || p.BoundaryMarker != "❯" {
		t.Errorf("claude-tmux → %+v, want claude profile", p)
	}
	if p := paneProfileFor(tmuxLaunch{name: "agy-tmux", promptMarker: ">"}); p.Name != panestream.FormatAgyREPL || !p.BoundaryExact {
		t.Errorf("agy-tmux → %+v, want agy profile (BoundaryExact)", p)
	}
	if p := paneProfileFor(tmuxLaunch{name: "itest-tmux", promptMarker: "§"}); p.Name != "itest" || p.BoundaryMarker != "§" {
		t.Errorf("itest-tmux → %+v, want fallback{itest, §}", p)
	}
}
