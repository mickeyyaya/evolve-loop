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

type hostLag struct {
	beforeIdle time.Duration
	afterIdle  time.Duration
}

func TestChannelE2E_RealFixtures_ClaudeSpan_UnderHostLag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		lag  hostLag
	}{
		{name: "slow_to_idle_reached", lag: hostLag{beforeIdle: 8 * time.Second}},
		{name: "slow_after_idle_reached", lag: hostLag{afterIdle: 4 * time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runClaudeSpanE2E(t, tc.lag)
		})
	}
}

func runClaudeSpanE2E(t *testing.T, lag hostLag) {
	t.Helper()
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
	lagBeforeIdle, lagAfterIdle := lag.beforeIdle, lag.afterIdle
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
			time.Sleep(lagBeforeIdle)
			lagBeforeIdle = 0
		default:
			time.Sleep(lagAfterIdle)
			lagAfterIdle = 0
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
	runCtx, runCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	prodDone := make(chan struct{})
	driverExited := make(chan struct{})
	driverDone := make(chan int, 1)
	t.Cleanup(func() {
		runCancel()
		<-prodDone
		<-driverExited
	})
	go func() {
		defer close(prodDone)
		_ = prod.Run(runCtx)
	}()
	go func() {
		defer close(driverExited)
		lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
		code, _ := runTmuxREPL(runCtx, cfg, deps, lp)
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

	sup := channel.NewSupervisor(channel.SupervisorConfig{
		Workspace: ws, Agent: "build", Transport: "claude-tmux",
		Now: time.Now, NewID: func() string { return "cX" },
		PollEvery: time.Millisecond, Timeout: 5 * time.Minute,
	})
	ans, err := sup.Ask(runCtx, "summarize what tmux is")
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
	case <-runCtx.Done():
		t.Error("driver did not complete after idle_reached")
	}
}
