// Package tmuxtest runs a test binary on a tmux server that only its own process owns.
package tmuxtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	paneShell         = "/bin/sh"
	serverStopTimeout = 10 * time.Second
)

func Main(m interface{ Run() int }) int {
	socket := bridge.DeriveTestSocket(os.Getpid())
	if err := os.Setenv(bridge.TmuxSocketEnv, socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: select socket %s: %v\n", socket, err)
		return 1
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return m.Run()
	}
	if err := startOwnServer(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return 1
	}
	code := m.Run()
	if err := stopServer(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return max(code, 1)
	}
	return code
}

func startOwnServer(socket string) error {
	if err := stopServer(socket); err != nil {
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

func stopServer(socket string) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverStopTimeout)
	defer cancel()
	return swarm.ExecKillServer(ctx, socket)
}
