package sysexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommand_SetsWaitDelayAndACancelThatSendsSIGTERM(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := Command(ctx, "sleep", "30")

	if cmd.WaitDelay != WaitDelay || cmd.Cancel == nil {
		t.Fatalf("WaitDelay = %v, Cancel set = %v, want %v and a Cancel", cmd.WaitDelay, cmd.Cancel != nil, WaitDelay)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	cancel()
	err := cmd.Wait()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Wait = %v, want the exit of a signaled process", err)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Errorf("exit = %v, want death by SIGTERM: the child gets a chance to stop its own children", exitErr)
	}
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }

func TestDefaultRunner_AGrandchildThatKeepsThePipeOpenIsASuccessAfterWaitDelay(t *testing.T) {
	t.Parallel()
	var stdout, stderr strings.Builder
	start := time.Now()

	code, err := DefaultRunner(context.Background(), "sh", "", []string{"-c", "sleep 30 & echo $!; exit 0"}, nil, nil, &stdout, &stderr)

	if pid, perr := strconv.Atoi(strings.TrimSpace(stdout.String())); perr == nil {
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	}
	if code != 0 || err != nil {
		t.Errorf("DefaultRunner = (%d, %v), want (0, nil): the command itself exited 0", code, err)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("DefaultRunner took %v, want about WaitDelay (%v)", elapsed, WaitDelay)
	}
}
