package swarm

import (
	"context"
	"fmt"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type SessionKiller interface {
	Kill(ctx context.Context, h SessionHandle) error
}

type ProcessGroupKiller func(pgid int) error

type TmuxKiller func(ctx context.Context, session string) error

var tmuxRun = func(ctx context.Context, args ...string) error {
	return sysexec.Command(ctx, "tmux", args...).Run()
}

func ExecTmuxKill(ctx context.Context, session string) error {
	if session == "" {
		return fmt.Errorf("refusing to kill tmux session with empty name (tmux would resolve it to the caller's own session)")
	}
	_ = tmuxRun(ctx, bridge.TmuxSocketArgs("kill-session", "-t", bridge.ExactSessionTarget(session))...)
	return nil
}

type ExecSessionKiller struct {
	KillGroup ProcessGroupKiller
	KillTmux  TmuxKiller
}

func (k ExecSessionKiller) Kill(ctx context.Context, h SessionHandle) error {
	var firstErr error
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

type ReapReport struct {
	Killed  []string
	Errors  []string
	Skipped int
}

func Reap(ctx context.Context, reg *SessionRegistry, killer SessionKiller) ReapReport {
	var rep ReapReport
	live := reg.Live()
	sort.Slice(live, func(i, j int) bool { return live[i].WorkerID < live[j].WorkerID })
	for _, h := range live {
		if err := killer.Kill(ctx, h); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", h.WorkerID, err))
		}
		if err := reg.MarkReaped(h.WorkerID); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: mark-reaped: %v", h.WorkerID, err))
		}
		rep.Killed = append(rep.Killed, h.WorkerID)
	}
	return rep
}
