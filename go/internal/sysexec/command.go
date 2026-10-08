package sysexec

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

const WaitDelay = 5 * time.Second

func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = WaitDelay
	return cmd
}
