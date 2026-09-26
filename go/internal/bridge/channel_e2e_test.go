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

// readCLIFrame loads a real capture from panestream's testdata, so one set of captures drives both suites.
func readCLIFrame(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("panestream", "testdata", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(b)
}

// The fake tmux is self-paced off breadcrumbs.live (the answer only after inject_applied, the artifact only
// after idle_reached), so the test does not depend on goroutine scheduling.
func TestChannelE2E_RealFixtures_ClaudeSpan(t *testing.T) {
	ws := t.TempDir()
	thinking := readCLIFrame(t, "claude/thinking.txt") // busy: ✽ Inferring… · esc to interrupt
	answer := readCLIFrame(t, "claude/answer.txt")     // idle: ⏺ bullets, no interrupt affordance
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

	tmux := &paneScriptTmux{pane: thinking} // boot marker ❯ present → REPL boots
	deps.Tmux = tmux
	// Driver tick 3 ms, producer poll 1 ms: inject_applied reaches the feed before the answer and idle_reached pair.
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			time.Sleep(200 * time.Microsecond) // boot/prompt 1 s sleeps
			return
		}
		bc, _ := os.ReadFile(bcPath)
		switch {
		case !strings.Contains(string(bc), "inject_applied"):
			tmux.setPane(thinking) // busy; marker present → the ask delivers, sawBusy set
		case !strings.Contains(string(bc), "idle_reached"):
			tmux.setPane(answer) // PaneBusy=false + ⏺ answer above the box → idle_reached + pane.live
		default:
			// Written once: under the cross-poll stability window a file rewritten every tick keeps bumping its
			// mtime and never settles.
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

	// The driver seeks the inbox cursor to EOF at boot, so an ask appended before the seek is never delivered.
	// It creates build-pane.live after the seek in the same goroutine, so waiting for that file syncs positively.
	paneLivePath := filepath.Join(ws, "build-pane.live")
	for bootDeadline := time.Now().Add(30 * time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(paneLivePath); err == nil {
			break
		}
		if time.Now().After(bootDeadline) {
			t.Fatal("driver never created build-pane.live — REPL boot did not reach the channel stage")
		}
	}

	// The supervisor's clock must advance: its Ask deadline reads the injected Now, and a frozen clock turns a
	// lost answer into an endless poll instead of a 10s ErrResponseTimeout.
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
