package swarm

import (
	"context"
	"fmt"
	"os/exec"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// SessionKiller tears down one worker session.
type SessionKiller interface {
	Kill(ctx context.Context, h SessionHandle) error
}

// ProcessGroupKiller signals a whole process group.
type ProcessGroupKiller func(pgid int) error

// TmuxKiller kills one tmux session by name.
type TmuxKiller func(ctx context.Context, session string) error

// tmuxRun is a test seam, so the unit suite never touches a real tmux server.
var tmuxRun = func(ctx context.Context, args ...string) error {
	return exec.CommandContext(ctx, "tmux", args...).Run()
}

// ExecTmuxKill is the production TmuxKiller on the bridge socket: it refuses an empty name, and a missing session counts as success.
func ExecTmuxKill(ctx context.Context, session string) error {
	if session == "" {
		return fmt.Errorf("refusing to kill tmux session with empty name (tmux would resolve it to the caller's own session)")
	}
	_ = tmuxRun(ctx, bridge.TmuxSocketArgs("kill-session", "-t", session)...)
	return nil
}

// ExecSessionKiller is the production SessionKiller: a process-group kill, then a tmux kill, each best-effort.
type ExecSessionKiller struct {
	KillGroup ProcessGroupKiller
	KillTmux  TmuxKiller
}

// Kill runs both teardown steps even when the first fails, and returns the first error.
func (k ExecSessionKiller) Kill(ctx context.Context, h SessionHandle) error {
	var firstErr error
	// kill(-pgid): pgid 0 is the caller's own group and pgid 1 signals every process the caller may signal.
	if h.PGID > 1 && k.KillGroup != nil {
		if err := k.KillGroup(h.PGID); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("kill pgid %d: %w", h.PGID, err)
		}
	}
	if h.TmuxSession != "" && k.KillTmux != nil {
		if err := k.KillTmux(ctx, h.TmuxSession); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("kill tmux %q: %w", h.TmuxSession, err)
		}
	}
	return firstErr
}

// ReapReport summarizes a reaper sweep.
type ReapReport struct {
	Killed  []string // worker IDs
	Errors  []string
	Skipped int
}

// Reap kills every Live session in the registry and marks each reaped as it goes; one failure never stops the sweep.
func Reap(ctx context.Context, reg *SessionRegistry, killer SessionKiller) ReapReport {
	var rep ReapReport
	live := reg.Live()
	sort.Slice(live, func(i, j int) bool { return live[i].WorkerID < live[j].WorkerID })
	for _, h := range live {
		if err := killer.Kill(ctx, h); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", h.WorkerID, err))
			// Still mark it reaped: the process is likely gone, and a Live corpse would be retried by every sweep.
		}
		if err := reg.MarkReaped(h.WorkerID); err != nil {
			// A stale-Live manifest entry makes the next sweep re-target a corpse, so surface it.
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: mark-reaped: %v", h.WorkerID, err))
		}
		rep.Killed = append(rep.Killed, h.WorkerID)
	}
	return rep
}
