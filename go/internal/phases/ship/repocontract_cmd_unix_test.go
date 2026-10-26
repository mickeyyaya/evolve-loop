//go:build unix && integration

package ship

import (
	"bufio"
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

func TestCancelKillsTheProcessGroup_CancelReapsTheGrandchildToo(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready, readyWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Close()
	defer readyWriter.Close()
	cmd := exec.CommandContext(ctx, "sh", "-c", `sh -c 'echo $$; exec sleep 30' & exec sleep 30`)
	cancelKillsTheProcessGroup(cmd)
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdout = readyWriter
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	readyWriter.Close()
	grandchild := readReadyPid(t, ready, 5*time.Second)
	pgid := cmd.Process.Pid

	start := time.Now()
	cancel()
	waitErr := cmd.Wait()
	elapsed := time.Since(start)

	if elapsed > 4*time.Second {
		t.Errorf("Wait took %v after cancel, want the group killed promptly", elapsed)
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) || exitErr.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Errorf("Wait = %v (state %v, %v after cancel), want the child killed by SIGKILL", waitErr, cmd.ProcessState, elapsed)
	}
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(grandchild, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(grandchild, 0); !errors.Is(err, syscall.ESRCH) {
		if gpid, gerr := syscall.Getpgid(grandchild); gerr == nil && gpid == pgid {
			_ = syscall.Kill(grandchild, syscall.SIGKILL)
		}
		t.Errorf("grandchild %d still alive after cancel (kill -0 = %v), want the whole process group gone", grandchild, err)
	}
}

func readReadyPid(t *testing.T, ready *os.File, timeout time.Duration) int {
	t.Helper()
	if err := ready.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(ready).ReadString('\n')
	if err != nil {
		t.Fatalf("grandchild never reported ready within %v: %v", timeout, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parse grandchild pid %q: %v", line, err)
	}
	return pid
}

func TestCancelKillsTheProcessGroup_PutsTheChildInItsOwnGroup(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("true")

	cancelKillsTheProcessGroup(cmd)

	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("SysProcAttr = %+v, want Setpgid", cmd.SysProcAttr)
	}
	if cmd.Cancel == nil {
		t.Fatal("Cancel = nil, want a group kill")
	}
}
