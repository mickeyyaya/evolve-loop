package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
)

func readCLIFrame(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("panestream", "testdata", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(b)
}

func TestChannelE2E_RealFixtures_ClaudeSpan(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	thinking := readCLIFrame(t, "claude/thinking.txt")
	answer := readCLIFrame(t, "claude/answer.txt")
	bcPath := filepath.Join(ws, "build-breadcrumbs.live")
	artifact := filepath.Join(ws, "a")

	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	cfg := &Config{
		Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e"),
	}
	deps := covDeps()
	deps.Stderr = io.Discard
	deps.RecoveryStage = "enforce"

	tmux := &paneScriptTmux{pane: thinking}
	deps.Tmux = tmux
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			time.Sleep(200 * time.Microsecond)
			return
		}
		bc, _ := os.ReadFile(bcPath)
		switch {
		case !strings.Contains(string(bc), "inject_applied"):
			tmux.setPane(thinking)
		case !strings.Contains(string(bc), "idle_reached"):
			tmux.setPane(answer)
		default:
			if _, err := os.Stat(artifact); err != nil {
				_ = os.WriteFile(artifact, []byte("done"), 0o644)
			}
		}
		time.Sleep(3 * time.Millisecond)
	}

	now := func() time.Time { return time.Unix(0, 0).UTC() }
	prod := channel.NewProducer(channel.ProducerConfig{
		Workspace: ws, Agent: "build", Phase: "build", Cycle: 1,
		StdoutPath: filepath.Join(ws, "build-pane.live"),
		StderrPath: bcPath,
		PollEvery:  time.Millisecond, Now: now,
	})
	prodCtx, prodCancel := context.WithCancel(context.Background())
	defer prodCancel()
	go func() { _ = prod.Run(prodCtx) }()

	driverDone := make(chan int, 1)
	go func() {
		lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
		code, _ := runTmuxREPL(context.Background(), cfg, deps, lp)
		driverDone <- code
	}()

	paneLivePath := filepath.Join(ws, "build-pane.live")
	for bootDeadline := time.Now().Add(30 * time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(paneLivePath); err == nil {
			break
		}
		if time.Now().After(bootDeadline) {
			t.Fatal("driver never created build-pane.live — REPL boot did not reach the channel stage")
		}
	}

	var supTick int64
	supNow := func() time.Time {
		supTick++
		return time.Unix(0, supTick*int64(time.Millisecond)).UTC()
	}
	sup := channel.NewSupervisor(channel.SupervisorConfig{
		Workspace: ws, Agent: "build", Transport: "claude-tmux",
		Now: supNow, NewID: func() string { return "cX" },
		PollEvery: time.Millisecond, Timeout: 10 * time.Second,
	})
	ans, err := sup.Ask(context.Background(), "summarize what tmux is")
	if err != nil {
		t.Fatalf("Supervisor.Ask: %v", err)
	}
	if !strings.Contains(ans.Text(), "multiplexer") {
		t.Fatalf("recovered answer span missing real fixture content; got:\n%s", ans.Text())
	}

	select {
	case code := <-driverDone:
		if code != ExitOK {
			t.Errorf("driver exit=%d, want ExitOK", code)
		}
	case <-time.After(3 * time.Second):
		t.Error("driver did not complete after idle_reached")
	}
}
