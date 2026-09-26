package swarm

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// ReapRunReport summarizes one registry reap.
type ReapRunReport struct {
	Killed  int
	Skipped int // empty, or outside the evolve-bridge- namespace
	Errors  int // an unreadable registry or a killer error
}

// ReapRunSessions kills the sessions in the run's own registry at recordsPath, never a server-wide listing; a missing registry is success.
func ReapRunSessions(ctx context.Context, recordsPath string, kill TmuxKiller) ReapRunReport {
	var rep ReapRunReport
	recs, err := sessionrecord.ReadAll(recordsPath)
	if err != nil {
		// Degrade to a counted leak rather than fall back to a fuzzy server-wide reap.
		rep.Errors++
		return rep
	}
	for _, r := range recs {
		// tmux resolves an empty target to the caller's own session.
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
