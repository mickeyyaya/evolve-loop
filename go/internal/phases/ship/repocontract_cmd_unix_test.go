//go:build unix && integration

package ship

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancelKillsTheProcessGroup_CancelReapsTheGrandchildToo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 30 & echo $!; wait")
	cancelKillsTheProcessGroup(cmd)
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	n, err := stdout.Read(buf)
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	grandchild, err := strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil {
		t.Fatalf("parse grandchild pid %q: %v", buf[:n], err)
	}
	pgid := cmd.Process.Pid

	start := time.Now()
	cancel()
	waitErr := cmd.Wait()

	if time.Since(start) > 4*time.Second {
		t.Errorf("Wait took %v after cancel, want the group killed promptly", time.Since(start))
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) || exitErr.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Errorf("Wait = %v, want the shell killed by SIGKILL", waitErr)
	}
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(grandchild, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(grandchild, 0); !errors.Is(err, syscall.ESRCH) {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		t.Errorf("grandchild %d still alive after cancel (kill -0 = %v), want the whole process group gone", grandchild, err)
	}
}

func TestCancelKillsTheProcessGroup_PutsTheChildInItsOwnGroup(t *testing.T) {
	cmd := exec.Command("true")

	cancelKillsTheProcessGroup(cmd)

	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr = %+v, want Setpgid", cmd.SysProcAttr)
	}
	if cmd.Cancel == nil {
		t.Fatal("Cancel = nil, want a group kill")
	}
}
