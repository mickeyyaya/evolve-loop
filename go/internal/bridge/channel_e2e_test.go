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

// readCLIFrame loads a committed real capture-pane fixture for the channel e2e.
// The frames live under the panestream package's testdata so the same
// source-of-truth captures drive both the unit extractor tests and this e2e.
func readCLIFrame(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("panestream", "testdata", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(b)
}

// TestChannelE2E_RealFixtures_ClaudeSpan drives the ENTIRE bidirectional channel
// against REAL captured claude frames — no hand-written Producer inputs:
//
//	Supervisor.Ask → inbox → driver delivers (inject_applied → breadcrumbs.live)
//	→ real thinking/answer frames through the driver's PaneDelta → pane.live
//	→ busy→idle via PaneBusy (idle_reached → breadcrumbs.live)
//	→ real Producer normalizes the .live pair → feed
//	→ Supervisor recovers the bracketed answer span.
//
// The fake tmux is SELF-PACED off breadcrumbs.live (wait for inject_applied
// before showing the answer; write the artifact only after idle_reached), so the
// test is robust to goroutine scheduling rather than tuned to exact tick counts.
// The 3:1 driver-tick : producer-poll ratio guarantees inject_applied is
// normalized into the feed in an earlier poll than the answer+idle_reached pair,
// so the answer content's seq falls strictly inside [request, response_complete].
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
	// Self-paced state machine, keyed on what the driver has already written.
	// Driver tick = 3 ms; the Producer polls every 1 ms, so inject_applied lands
	// in the feed before the answer+idle_reached pair (correct span ordering).
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
			_ = os.WriteFile(artifact, []byte("done"), 0o644) // both breadcrumbs in → complete
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

	// The driver seeks the inbox cursor to EOF at boot, so an ask appended
	// BEFORE that seek is skipped forever — and with this test's self-paced
	// tmux, a skipped ask means the answer never arrives. A bare sleep here
	// lost that race on a loaded CI runner (driver goroutine scheduled late →
	// 10m package-timeout panic, macOS 2026-08-03). Sync positively instead:
	// the driver creates build-pane.live strictly AFTER the cursor seek in the
	// same goroutine, so once the file exists the seek has happened and an
	// appended ask is guaranteed to be delivered.
	paneLivePath := filepath.Join(ws, "build-pane.live")
	for bootDeadline := time.Now().Add(30 * time.Second); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(paneLivePath); err == nil {
			break
		}
		if time.Now().After(bootDeadline) {
			t.Fatal("driver never created build-pane.live — REPL boot did not reach the channel stage")
		}
	}

	// The supervisor's clock must ADVANCE: its Ask deadline is computed and
	// checked via the injected Now, so the producer's frozen clock here turned
	// every lost-answer scenario into an infinite poll — the 10s Timeout could
	// structurally never fire, and a missed ask became the package's 10-minute
	// timeout panic (macOS CI 2026-08-03) instead of a 10s ErrResponseTimeout
	// with diagnostics. One fake millisecond per reading keeps the test free of
	// wall-clock timestamps while making the failsafe real.
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
