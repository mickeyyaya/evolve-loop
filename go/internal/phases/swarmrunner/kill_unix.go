// kill helpers use syscall.Kill (unix) — consistent with cmd/evolve/cmd_swarm.go.
// The repo targets macOS/Linux only (bash 3.2 / CLAUDE.md); no Windows build.
package swarmrunner

import (
	"context"
	"fmt"
	"os/exec"
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

// tmuxKiller kills a tmux session by name (best-effort; an already-dead session
// is the desired end state, so a non-zero exit is ignored).
func tmuxKiller(ctx context.Context, session string) error {
	_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", session).Run()
	return nil
}
