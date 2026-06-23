package swarm

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/sessionrecord"
)

// reap_runsessions.go — CB.5 run-teardown reaper (concurrency campaign W4).
//
// A run's tmux sessions are reaped FROM ITS OWN REGISTRY FILE
// (<workspace>/tmux-sessions.jsonl, written by the bridge at session
// creation) — never from a server-wide listing. Per-run file ⇒ a run's
// teardown is structurally incapable of touching another run's sessions
// (the 2026-06-11 killer-B class: fuzzy teardown on the shared server
// destroyed every concurrent soak). Killing an already-dead session is a
// no-op by ExecTmuxKill's best-effort contract, so reaping after a clean
// cycle (where per-launch cleanup already killed everything) is harmless.

// ReapRunReport summarizes one registry reap.
type ReapRunReport struct {
	Killed  int // sessions passed to the killer
	Skipped int // unsafe names refused (empty / outside the evolve-bridge namespace)
	Errors  int // killer errors (best-effort; reap continues)
}

// ReapRunSessions kills every session in the run registry at recordsPath via
// kill (production: ExecTmuxKill). A missing registry is a run that launched
// no sessions — zero-activity success. Names that are empty (the killer-B
// suicide class: tmux resolves an empty target to the CALLER'S session) or
// outside the evolve-bridge- namespace are skipped and counted, never killed.
func ReapRunSessions(ctx context.Context, recordsPath string, kill TmuxKiller) ReapRunReport {
	var rep ReapRunReport
	recs, err := sessionrecord.ReadAll(recordsPath)
	if err != nil {
		// Unreadable registry degrades to leak-on-crash (the pre-CB.5 state),
		// surfaced via the error count rather than a partial fuzzy reap.
		rep.Errors++
		return rep
	}
	for _, r := range recs {
		if r.Session == "" || !strings.HasPrefix(r.Session, "evolve-bridge-") {
			rep.Skipped++
			continue
		}
		if err := kill(ctx, r.Session); err != nil {
			rep.Errors++
			continue
		}
		rep.Killed++
	}
	return rep
}
