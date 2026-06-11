// kill helpers use syscall.Kill (unix) — consistent with cmd/evolve/cmd_swarm.go.
// The repo targets macOS/Linux only (bash 3.2 / CLAUDE.md); no Windows build.
package swarmrunner

import (
	"fmt"
	"syscall"
)

// groupKiller SIGKILLs a whole process group (negative pgid). It refuses pgid<=1
// (0 = the caller's own group, 1 = init/everything) — defense in depth atop
// swarm.ExecSessionKiller's own >1 gate.
func groupKiller(pgid int) error {
	if pgid <= 1 {
		return fmt.Errorf("refusing to kill process group %d", pgid)
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// tmux session teardown is swarm.ExecTmuxKill (shared with `evolve swarm reap`;
// empty-name refusal + best-effort semantics live there, seam-tested in swarm).
