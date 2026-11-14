// Package tmuxtest runs a test binary on a tmux server that only its own process owns.
package tmuxtest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	setenv       = os.Setenv
	fdDir        = "/dev/fd"
	serverPIDsMu sync.Mutex
	serverPIDs   = map[string]int{}
)

func recordServerPID(socket string, pid int) {
	serverPIDsMu.Lock()
	defer serverPIDsMu.Unlock()
	serverPIDs[socket] = pid
}

func lookupServerPID(socket string) int {
	serverPIDsMu.Lock()
	defer serverPIDsMu.Unlock()
	return serverPIDs[socket]
}

func deleteServerPID(socket string) {
	serverPIDsMu.Lock()
	defer serverPIDsMu.Unlock()
	delete(serverPIDs, socket)
}

func readServerPID(socket string) (int, error) {
	out, err := sysexec.Command(context.Background(), "tmux", "-L", socket, "display-message", "-p", "#{pid}").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

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
	if err := stopFamily(socket); err != nil {
		fmt.Fprintf(os.Stderr, "tmuxtest: %v\n", err)
		return max(code, 1)
	}
	return code
}

func stopFamily(socket string) error {
	errs := []error{StopServer(socket)}
	own, err := filepath.Glob(filepath.Join(bridge.TmuxSocketDir(), socket+"-*"))
	errs = append(errs, err)
	for _, path := range own {
		ctx, cancel := context.WithTimeout(context.Background(), serverStopTimeout)
		errs = append(errs, swarm.ExecKillServer(ctx, filepath.Base(path)))
		cancel()
	}
	return errors.Join(errs...)
}

func StartServer(socket string) error {
	if err := StopServer(socket); err != nil {
		return err
	}
	out, err := sysexec.Command(context.Background(), "tmux", "-L", socket,
		"-f", os.DevNull, "start-server", ";",
		"set-option", "-s", "exit-empty", "off", ";",
		"set-option", "-g", "default-shell", paneShell, ";",
		"set-option", "-g", "default-command", "exec "+paneShell,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start the tmux server on socket %s: %w: %s", socket, err, out)
	}
	if pid, perr := readServerPID(socket); perr == nil {
		recordServerPID(socket, pid)
	}
	return nil
}

func StopServer(socket string) error {
	if socket == "" {
		return fmt.Errorf("refusing kill-server with an empty socket name")
	}
	serverPid := lookupServerPID(socket)
	deleteServerPID(socket)
	ctx, cancel := context.WithTimeout(context.Background(), serverStopTimeout)
	defer cancel()
	err := swarm.ExecKillServer(ctx, socket)
	if serverPid > 1 && syscall.Kill(serverPid, 0) == nil {
		_ = syscall.Kill(serverPid, syscall.SIGTERM)
		for i := 0; i < 50 && syscall.Kill(serverPid, 0) == nil; i++ {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return err
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
	serverPid := lookupServerPID(socket)
	dir, name := shellQuote(bridge.TmuxSocketDir()), shellQuote(socket)
	killCmd := fmt.Sprintf("exec tmux -L %s kill-server", name)
	if serverPid > 1 {
		killCmd = fmt.Sprintf("rm -f %s/%s; kill -TERM %d 2>/dev/null; sleep 1; kill -9 %d 2>/dev/null", dir, name, serverPid, serverPid)
	}
	watch := fmt.Sprintf(`while kill -0 %d 2>/dev/null; do sleep 1; done; for s in %s/%s-*; do [ -S "$s" ] && tmux -S "$s" kill-server; rm -f "$s"; done; %s`,
		owner, dir, name, killCmd)
	out, err := sysexec.Command(context.Background(), "tmux", "-L", socket,
		"new-session", "-d", "-s", guardSession, watch,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start the orphan guard on socket %s: %w: %s", socket, err, out)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
