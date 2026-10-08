package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const detachLogTailLines = 20

var loopDetachCommandFn = defaultLoopDetachCommand

var loopDetachPoll = 200 * time.Millisecond

type detachedLaunch struct {
	pid    int
	offset int64
	done   <-chan error
}

func defaultLoopDetachCommand(argv []string) (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return exec.Command(self, append([]string{"loop"}, argv...)...), nil
}

func runLoopDetached(cfg loopConfig, stdout, stderr io.Writer) int {
	attr, err := detachSysProcAttr()
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: --detach: %v\n", err)
		return 1
	}
	runsDir := filepath.Join(cfg.EvolveDir, "runs")
	if live := runlease.LiveRuns(runsDir, time.Now()); len(live) > 0 {
		for _, r := range live {
			fmt.Fprintf(stderr, "evolve loop: --detach: run %s (%s) is already live (owner pid %d); not launching a second loop — stop it with: evolve loop-stop --wait\n", r.Lease.RunID, r.Dir, r.Lease.OwnerPID)
		}
		return 1
	}
	launch, err := startDetachedLoop(cfg, attr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: --detach: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "loop: detached pid %d, log %s\n", launch.pid, cfg.LogPath)
	if err := recordLogWriter(cfg.LogPath, launch.pid); err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: --detach: %v; gc can delete this log while the loop writes it\n", err)
	}
	return awaitDetachedBoot(cfg, launch, detachBootWait(cfg.EvolveDir, stderr), stdout, stderr)
}

func startDetachedLoop(cfg loopConfig, attr *syscall.SysProcAttr) (detachedLaunch, error) {
	logFile, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return detachedLaunch{}, fmt.Errorf("open log %s: %w", cfg.LogPath, err)
	}
	defer func() { _ = logFile.Close() }()
	info, err := logFile.Stat()
	if err != nil {
		return detachedLaunch{}, fmt.Errorf("stat log %s: %w", cfg.LogPath, err)
	}
	cmd, err := loopDetachCommandFn(cfg.DetachArgv)
	if err != nil {
		return detachedLaunch{}, fmt.Errorf("start: %w", err)
	}
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = attr
	if err := cmd.Start(); err != nil {
		return detachedLaunch{}, fmt.Errorf("start: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return detachedLaunch{pid: cmd.Process.Pid, offset: info.Size(), done: done}, nil
}

func recordLogWriter(logPath string, pid int) error {
	path := logPath + gcpolicy.LogWriterPIDSuffix
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("record the log writer pid in %s: %w", path, err)
	}
	return nil
}

func detachBootWait(evolveDir string, stderr io.Writer) time.Duration {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: --detach: %v; waiting the default %s\n", err, policy.DefaultBootDetachWait)
		return policy.DefaultBootDetachWait
	}
	return pol.BootDetachWait()
}

func awaitDetachedBoot(cfg loopConfig, launch detachedLaunch, wait time.Duration, stdout, stderr io.Writer) int {
	runsDir := filepath.Join(cfg.EvolveDir, "runs")
	deadline := time.Now().Add(wait)
	ticker := time.NewTicker(loopDetachPoll)
	defer ticker.Stop()
	for now := range ticker.C {
		select {
		case err := <-launch.done:
			fmt.Fprintf(stderr, "evolve loop: --detach: the loop exited during boot (exit %d); last lines of %s:\n", detachExitCode(err), cfg.LogPath)
			writeDetachTail(stderr, cfg.LogPath, launch.offset)
			return 1
		default:
		}
		if live := runlease.LiveRuns(runsDir, now); len(live) > 0 {
			fmt.Fprintf(stdout, "loop: running — pid %d, run %s (%s), log %s\n", launch.pid, live[0].Lease.RunID, filepath.Base(live[0].Dir), cfg.LogPath)
			return 0
		}
		if !now.Before(deadline) {
			fmt.Fprintf(stderr, "evolve loop: --detach: boot not confirmed within %s — pid %d is still running (not killed); log %s\n", wait, launch.pid, cfg.LogPath)
			writeDetachTail(stderr, cfg.LogPath, launch.offset)
			return 1
		}
	}
	return 1
}

func detachExitCode(err error) int {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	default:
		return -1
	}
}

func writeDetachTail(w io.Writer, path string, offset int64) {
	for _, line := range tailLogSince(path, offset, detachLogTailLines) {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

func tailLogSince(path string, offset int64, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{fmt.Sprintf("(log unreadable: %v)", err)}
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return []string{fmt.Sprintf("(log unreadable: %v)", err)}
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return []string{fmt.Sprintf("(log unreadable: %v)", err)}
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
