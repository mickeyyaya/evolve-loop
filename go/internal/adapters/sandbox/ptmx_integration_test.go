//go:build integration

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func agentTmuxDir(t *testing.T, dir, name string) string {
	t.Helper()
	tmpdir := filepath.Join(dir, name)
	t.Cleanup(func() {
		socket := filepath.Join(tmpdir, fmt.Sprintf("tmux-%d", os.Getuid()), "default")
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
	})
	return tmpdir
}

func TestGenerateSBPL_TheAgentWindowGetsItsOwnTerminalAndNoOtherPane(t *testing.T) {
	probe := requireSandboxedTmux(t)
	dir := shortSocketDir(t)
	run := privateTmuxServer(t, dir, "run")
	victim, err := exec.Command("tmux", "-S", run, "display-message", "-p", "#{pane_tty}").Output()
	if err != nil {
		t.Fatalf("read the host pane tty: %v", err)
	}
	victimTTY := strings.TrimSpace(string(victim))
	tmpdir := agentTmuxDir(t, dir, "agent")
	agent := agentProfile{probe.BinaryPath, GenerateSBPL(Config{RepoRoot: dir, HomeDir: dir, ReadOnlyRepo: true, WritePaths: []string{dir}, AllowNetwork: true})}
	marker := filepath.Join(dir, "ran")
	window := "tty > " + marker + ".tmp; (printf x > " + victimTTY + ") 2>/dev/null && echo wrote >> " + marker + ".tmp || echo denied >> " + marker + ".tmp; mv " + marker + ".tmp " + marker

	out, err := agent.sh(t, "unset TMUX; mkdir -p "+tmpdir+" && TMUX_TMPDIR="+tmpdir+" tmux -L default -f /dev/null new-session -d '"+window+"'"+
		" && for i in $(seq 50); do test -s "+marker+" && exit 0; sleep 0.1; done; exit 1")

	if err != nil {
		t.Fatalf("the agent must start a private tmux server and its window must run its command: %v: %s", err, out)
	}
	got, _ := os.ReadFile(marker)
	lines := strings.Fields(string(got))
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "/dev/ttys") || lines[0] == victimTTY || lines[1] != "denied" {
		t.Fatalf("the window must run on its own pty and must not write to the host pane %s, got %q", victimTTY, got)
	}
}

func TestGenerateSBPL_TheCycle1853ProbeGetsItsOwnServerAndLeavesTheRunServerAlive(t *testing.T) {
	probe := requireSandboxedTmux(t)
	dir := shortSocketDir(t)
	run := privateTmuxServer(t, dir, "run")
	tmpdir := agentTmuxDir(t, dir, "swxprobe")
	agent := agentProfile{probe.BinaryPath, GenerateSBPL(Config{
		RepoRoot: dir, HomeDir: dir, ReadOnlyRepo: true, WritePaths: []string{dir}, AllowNetwork: true,
		DenySockets: []string{run}, DenyLiterals: []string{dir, run},
	})}

	out, _ := agent.sh(t, "unset TMUX; d="+tmpdir+"; mkdir -p $d; TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe; echo \"rc=$?\";"+
		" TMUX_TMPDIR=$d tmux has-session -t probe && echo own-server; TMUX_TMPDIR=$d tmux kill-server 2>/dev/null; rm -rf $d")

	if !strings.Contains(out, "rc=0") || !strings.Contains(out, "own-server") {
		t.Fatalf("the probe must start the agent's own private server, got %q", out)
	}
	hostReaches(t, run)
	sessions, err := exec.Command("tmux", "-S", run, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil || strings.TrimSpace(string(sessions)) != "victim" {
		t.Fatalf("the probe must not touch the run server, sessions=%q err=%v", sessions, err)
	}
}
