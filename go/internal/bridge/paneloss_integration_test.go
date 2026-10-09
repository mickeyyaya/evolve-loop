//go:build integration

package bridge_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestRealTmux_AKilledServerEndsTheWatchingDispatchAsPaneLostWithinOneLivenessInterval(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed; the replay needs a real server to kill")
	}
	socket := bridge.DeriveTestSocket(os.Getpid()) + "-panelost"
	t.Setenv(bridge.TmuxSocketEnv, socket)
	if err := tmuxtest.StartServer(socket); err != nil {
		t.Fatalf("start the replay's own tmux server: %v", err)
	}
	t.Cleanup(func() {
		if err := tmuxtest.StopServer(socket); err != nil {
			t.Errorf("stop the replay's tmux server: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var killErr error

	run := bridge.DispatchFakeREPLAndLoseServer(t, ctx, 50*time.Millisecond, func() { killErr = tmuxtest.StopServer(socket) })

	if killErr != nil {
		t.Fatalf("kill the replay's tmux server: %v", killErr)
	}
	if got := launchoutcome.CauseCode(run.Code, run.Stderr); got != string(launchoutcome.TimeoutPaneLost) {
		t.Fatalf("exit=%d cause_code=%q, want %d with %q: a dispatch that watches a dead server must end as a lost pane; stderr:\n%s",
			run.Code, got, bridge.ExitArtifactTimeout, launchoutcome.TimeoutPaneLost, run.Stderr)
	}
	if limit := run.LossTick + 2; run.WaitTicks > limit {
		t.Errorf("wait ticks = %d, want <= %d: the loss ends the dispatch within one liveness interval", run.WaitTicks, limit)
	}
	if strings.Contains(run.Stderr, "stop-review") {
		t.Errorf("a stall review ran or was reported on a dead pane; stderr:\n%s", run.Stderr)
	}
	if !strings.Contains(run.Stderr, "the pane was lost)") {
		t.Errorf("the closeout line does not name the lost pane; stderr:\n%s", run.Stderr)
	}
}
