//go:build unix

package ship

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func cancelKillsTheProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

type waitState interface {
	ExitCode() int
	Sys() any
	String() string
}

func describeWaitResult(err error, state waitState, elapsed time.Duration) string {
	if ps, typed := state.(*os.ProcessState); state == nil || (typed && ps == nil) {
		return fmt.Sprintf("no process state elapsed=%s err=%v", elapsed, err)
	}
	signal := "unknown"
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		signal = "none"
		if status.Signaled() {
			signal = status.Signal().String()
		}
	}
	return fmt.Sprintf("exit=%d signal=%s elapsed=%s err=%v", state.ExitCode(), signal, elapsed, err)
}
