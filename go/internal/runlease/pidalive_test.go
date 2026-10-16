package runlease

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestPIDAlive_OwnProcessAliveExitedProcessDeadNonPositiveNeverProbed(t *testing.T) {
	if !PIDAlive(os.Getpid()) {
		t.Error("our own pid is alive")
	}
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot spawn a process to retire: %v", err)
	}
	if PIDAlive(cmd.Process.Pid) {
		t.Errorf("pid %d has exited and been reaped — not alive", cmd.Process.Pid)
	}
	for _, pid := range []int{0, -1, -42} {
		if PIDAlive(pid) {
			t.Errorf("pid %d must never be probed (a group, not a process) and reports dead", pid)
		}
	}
	now := time.Now()
	beat := now.UTC().Format(time.RFC3339Nano)
	fresh := Lease{RunID: "r", OwnerPID: cmd.Process.Pid, HeartbeatAt: beat}
	if OwnerLive(fresh, now, 0, PIDAlive) {
		t.Error("a fresh lease with a dead owner is not live")
	}
	if !OwnerLive(Lease{RunID: "r", HeartbeatAt: beat}, now, 0, PIDAlive) {
		t.Error("an ownerless fresh lease stays live (nothing to probe)")
	}
}
