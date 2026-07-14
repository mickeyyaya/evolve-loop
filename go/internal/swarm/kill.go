package swarm

import (
	"context"
	"fmt"
	"os/exec"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// SessionKiller tears down one worker session. Injected so the reaper/dispatcher
// are testable without real tmux or signals. The production impl
// (ExecSessionKiller) does process-group kill → tmux kill-session → confirm.
type SessionKiller interface {
	Kill(ctx context.Context, h SessionHandle) error
}

// ProcessGroupKiller signals a whole process group (negative pgid). Injected so
// tests don't actually send signals. The production impl wraps syscall.Kill.
type ProcessGroupKiller func(pgid int) error

// TmuxKiller kills a tmux session by name (the bridge's TmuxController.KillSession
// shape, narrowed to what teardown needs). Injected for testability.
type TmuxKiller func(ctx context.Context, session string) error

// tmuxRun is the exec seam for ExecTmuxKill, overridden in tests so the unit
// suite never touches a real tmux server.
var tmuxRun = func(ctx context.Context, args ...string) error {
	return exec.CommandContext(ctx, "tmux", args...).Run()
}

// ExecTmuxKill is the production TmuxKiller (shared by swarmrunner teardown and
// `evolve swarm reap`). SAFETY: it refuses an empty session name — tmux resolves
// an empty `-t` target to the CLIENT'S CURRENT session, so a blank name fired
// from inside any tmux pane kills the caller's own session (the 2026-06-11
// killer-B forensics: a test live-firing the empty case destroyed every soak
// session on the shared default socket). Otherwise best-effort: a missing
// session is the desired end state, so the tmux exit code is ignored.
func ExecTmuxKill(ctx context.Context, session string) error {
	if session == "" {
		return fmt.Errorf("refusing to kill tmux session with empty name (tmux would resolve it to the caller's own session)")
	}
	// -L bridge.TmuxSocket: agent panes live on the bridge's isolated socket, so
	// teardown must target it — a default-socket kill would leave them orphaned.
	_ = tmuxRun(ctx, bridge.TmuxSocketArgs("kill-session", "-t", session)...)
	return nil
}

// ExecSessionKiller is the production SessionKiller: it kills the worker's
// process group (reaping the inner CLI + any grandchildren that a bare
// process-kill would orphan), then kills its tmux session. Both steps are
// best-effort and independent — a missing pgid or session is not an error, so a
// partially-torn-down worker still gets fully cleaned.
type ExecSessionKiller struct {
	KillGroup ProcessGroupKiller
	KillTmux  TmuxKiller
}

// Kill implements SessionKiller.
func (k ExecSessionKiller) Kill(ctx context.Context, h SessionHandle) error {
	var firstErr error
	// SAFETY: group-kill uses a NEGATIVE pgid (syscall.Kill(-pgid, …)). pgid 0 =
	// the caller's own group; pgid 1 = init/launchd, and kill(-1, …) signals
	// EVERY process the caller may signal — either is catastrophic. Require >1.
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
	Killed  []string // worker IDs whose sessions were killed
	Errors  []string // human-readable per-session errors (best-effort sweep continues)
	Skipped int      // sessions already reaped
}

// Reap tears down every still-Live session in the registry, marking each reaped
// as it goes, and persisting after each so a crash mid-sweep leaves an accurate
// manifest. Best-effort: a kill error on one session is recorded and the sweep
// continues to the next (an orphan must never block reaping its siblings).
//
// This is the in-process teardown path (dispatch scope exit) AND the body of
// `evolve swarm reap` after it loads a manifest into a registry.
func Reap(ctx context.Context, reg *SessionRegistry, killer SessionKiller) ReapReport {
	var rep ReapReport
	live := reg.Live()
	sort.Slice(live, func(i, j int) bool { return live[i].WorkerID < live[j].WorkerID })
	for _, h := range live {
		if err := killer.Kill(ctx, h); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", h.WorkerID, err))
			// Still mark reaped: the process is likely gone (kill of a dead pgid
			// errors), and leaving it Live would make every future sweep retry a
			// corpse. The error is surfaced for the operator, not retried forever.
		}
		if err := reg.MarkReaped(h.WorkerID); err != nil {
			// Whether or not the kill above succeeded, the worker still lands
			// in rep.Killed (the process is presumed dead) — but the manifest
			// still shows it Live. A stale-Live entry makes the next sweep
			// re-target a corpse, so surface the persistence failure instead
			// of swallowing it. Reaping continues.
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: mark-reaped: %v", h.WorkerID, err))
		}
		rep.Killed = append(rep.Killed, h.WorkerID)
	}
	return rep
}
