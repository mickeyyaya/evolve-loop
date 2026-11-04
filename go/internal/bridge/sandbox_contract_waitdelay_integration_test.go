//go:build integration && darwin

package bridge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const agentReturnBound = 15 * time.Second

func hostWithoutSandbox(t *testing.T) contractHost {
	t.Helper()
	return contractHost{prefix: []string{"/usr/bin/env"}, scratch: contractTempDir(t, "/tmp", "sbc-agent")}
}

func TestAgentWaitDelay_ReturnsWhileADaemonChildHoldsStdout(t *testing.T) {
	t.Parallel()
	h := hostWithoutSandbox(t)
	row := contractRow{script: func(contractHost) string { return "sleep 25 & echo started" }}
	start := time.Now()
	out, _ := h.agent(t, row)
	if elapsed := time.Since(start); elapsed > agentReturnBound {
		t.Errorf("the agent call waited %s for a daemon child that holds stdout; it must return within %s", elapsed, agentReturnBound)
	}
	if out != "started" {
		t.Errorf("the output written before the wait ended must be kept, got %q", out)
	}
}

func TestAgentWaitDelay_NoPrivateTmuxServerOutlivesTheRow(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	h := hostWithoutSandbox(t)
	socketName := "evolvewd" + strconv.Itoa(os.Getpid())
	tmuxDir := filepath.Join(h.scratch, "own")
	tmuxCmd := func(args ...string) *exec.Cmd {
		cmd := exec.Command("tmux", append([]string{"-L", socketName}, args...)...)
		cmd.Env = append(paneEnv(os.Environ()), "TMUX_TMPDIR="+tmuxDir)
		return cmd
	}
	t.Cleanup(func() { _ = tmuxCmd("kill-server").Run() })
	row := contractRow{script: func(h contractHost) string {
		return "mkdir -p " + tmuxDir + " && export TMUX_TMPDIR=" + tmuxDir + " && tmux -L " + socketName + " -f /dev/null new-session -d 'sleep 120'; echo started"
	}}
	t.Run("row", func(t *testing.T) {
		start := time.Now()
		out, _ := h.agent(t, row)
		if elapsed := time.Since(start); elapsed > agentReturnBound {
			t.Errorf("the agent call waited %s for the daemon tmux server; it must return within %s", elapsed, agentReturnBound)
		}
		if out != "started" {
			t.Errorf("the output written before the wait ended must be kept, got %q", out)
		}
	})
	if err := tmuxCmd("has-session").Run(); err == nil {
		t.Errorf("the private tmux server -L %s of the row is still alive after the row ended", socketName)
	}
}
