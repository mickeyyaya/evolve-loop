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
)

// paneScriptTmux is a scriptable fake whose CapturePane returns whatever the
// test's Sleep hook last set via setPane. Unlike fakeTmux.paneSeq (consumed in
// order), every capture within a tick returns the SAME current frame, so a test
// controls exactly what the per-tick channel delta extractor sees.
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

// Minimal claude-shaped rendered frames (BoundaryMarker "❯"). The thinking
// frame has only the echoed prompt as stable content (the spinner trims as a
// volatile tail row); the answer frame adds the ⏺ bullet above the input box.
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

// TestRunTmuxREPL_ChannelOff_NoLiveFiles asserts that with EVOLVE_CHANNEL unset
// the driver creates neither .live file (byte-identical off path).
func TestRunTmuxREPL_ChannelOff_NoLiveFiles(t *testing.T) {
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	deps := covDeps() // LookupEnv = mapLookup(nil) → EVOLVE_CHANNEL unset
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

// TestRunTmuxREPL_ChannelOn_StreamsPaneDeltaToFile asserts that with the channel
// on, the answer content rendered in the pane accrues to <agent>-pane.live via
// the per-tick PaneDelta extractor.
func TestRunTmuxREPL_ChannelOn_StreamsPaneDeltaToFile(t *testing.T) {
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	deps := covDeps()
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "enforce"})
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
			tmux.setPane(paneThinkingFrame) // primes baseline
		case 2:
			tmux.setPane(paneAnswerFrame) // answer becomes stable → emitted
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
	// The volatile input box / footer must never leak as content.
	if strings.Contains(string(body), "bypass permissions") {
		t.Errorf("pane.live leaked volatile footer; got:\n%s", body)
	}
}

// TestRunTmuxREPL_ChannelOn_BreadcrumbsToFile delivers a correlated command and
// asserts both breadcrumbs land in <agent>-breadcrumbs.live (in order, exactly
// once each) and NOT on stderr.
func TestRunTmuxREPL_ChannelOn_BreadcrumbsToFile(t *testing.T) {
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	stderr := &bytes.Buffer{}
	deps := covDeps()
	deps.Stderr = stderr
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "enforce"})
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
			// Busy: a real claude thinking footer (PaneBusy keys on the
			// interrupt affordance, not the ❯ marker which persists throughout).
			tmux.setPane("❯ q\n\n✽ Inferring…\n\n❯\n  ⏵⏵ bypass permissions on · esc to interrupt")
		case 3:
			// Idle: answer present, no interrupt affordance → busy→idle transition.
			tmux.setPane("❯ q\n\n⏺ done\n\n❯\n  ⏵⏵ bypass permissions on · ← for agents")
		default:
			// Drop the artifact only after both breadcrumbs are in the file so
			// the loop cannot exit before idle_reached fires.
			if b, _ := os.ReadFile(bcPath); strings.Contains(string(b), "idle_reached") {
				_ = os.WriteFile(cfg.Artifact, []byte("done"), 0o644)
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
	// Breadcrumbs must go to the file, not the driver's stderr log.
	if strings.Contains(stderr.String(), "evolve_channel") {
		t.Errorf("breadcrumb leaked to stderr:\n%s", stderr.String())
	}
}

// TestRunTmuxREPL_ChannelOn_LiveFileOpenError_WARN forces an unopenable .live
// path (agent name with a missing parent dir) and asserts the driver WARNs and
// still completes — channel streaming degrades, it does not abort the phase.
func TestRunTmuxREPL_ChannelOn_LiveFileOpenError_WARN(t *testing.T) {
	ws := t.TempDir()
	cfg := paneLiveCfg(t, ws)
	cfg.Agent = "missing/sub" // parent dir ws/missing does not exist → OpenFile fails
	stderr := &bytes.Buffer{}
	deps := covDeps()
	deps.Stderr = stderr
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "enforce"})
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
	if p := paneProfileFor(tmuxLaunch{name: "claude-tmux", promptMarker: "❯"}); p.Name != "claude" || p.BoundaryMarker != "❯" {
		t.Errorf("claude-tmux → %+v, want claude profile", p)
	}
	if p := paneProfileFor(tmuxLaunch{name: "agy-tmux", promptMarker: ">"}); p.Name != "agy" || !p.BoundaryExact {
		t.Errorf("agy-tmux → %+v, want agy profile (BoundaryExact)", p)
	}
	// Unknown driver → fallback profile built from the launch's prompt marker.
	if p := paneProfileFor(tmuxLaunch{name: "itest-tmux", promptMarker: "§"}); p.Name != "itest" || p.BoundaryMarker != "§" {
		t.Errorf("itest-tmux → %+v, want fallback{itest, §}", p)
	}
}
