// Package tmuxtest runs a test binary on a tmux server that only its own process owns.
package tmuxtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	paneShell         = "/bin/sh"
	serverStopTimeout = 10 * time.Second
	guardSession      = "tmuxtest-orphan-guard"
)

var (
	setenv = os.Setenv
	fdDir  = "/dev/fd"
)

func Main(m interface{ Run() int }) int {
	socket := bridge.DeriveTestSocket(os.Getpid())
	if err := setenv(bridge.TmuxSocketEnv, socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: select socket %s: %v\n", socket, err)
		return 1
	}
	if err := closeInheritedOnExec(); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return 1
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return m.Run()
	}
	if err := StartServer(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return 1
	}
	if err := startOrphanGuard(socket, os.Getpid()); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		_ = StopServer(socket)
		return 1
	}
	code := m.Run()
	if err := StopServer(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return max(code, 1)
	}
	return code
}

func StartServer(socket string) error {
	if err := StopServer(socket); err != nil {
		return err
	}
	out, err := sysexec.Command(context.Background(), "tmux", bridge.TmuxSocketArgs(
		"-f", os.DevNull, "start-server", ";",
		"set-option", "-s", "exit-empty", "off", ";",
		"set-option", "-g", "default-shell", paneShell, ";",
		"set-option", "-g", "default-command", "exec "+paneShell,
	)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start the tmux server on socket %s: %w: %s", socket, err, out)
	}
	return nil
}

func StopServer(socket string) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverStopTimeout)
	defer cancel()
	return swarm.ExecKillServer(ctx, socket)
}

func closeInheritedOnExec() error {
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return fmt.Errorf("list the inherited descriptors in %s: %w", fdDir, err)
	}
	for _, e := range entries {
		fd, err := strconv.Atoi(e.Name())
		if err != nil {
			return fmt.Errorf("read the descriptor %q in %s: %w", e.Name(), fdDir, err)
		}
		if fd > 2 {
			syscall.CloseOnExec(fd)
		}
	}
	return nil
}

func startOrphanGuard(socket string, owner int) error {
	dir, name := shellQuote(bridge.TmuxSocketDir()), shellQuote(socket)
	watch := fmt.Sprintf(`while kill -0 %d 2>/dev/null; do sleep 1; done; for s in %s/%s-*; do [ -S "$s" ] && tmux -S "$s" kill-server; done; exec tmux -L %s kill-server`,
		owner, dir, name, name)
	out, err := sysexec.Command(context.Background(), "tmux", bridge.TmuxSocketArgs(
		"new-session", "-d", "-s", guardSession, watch,
	)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start the orphan guard on socket %s: %w: %s", socket, err, out)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
