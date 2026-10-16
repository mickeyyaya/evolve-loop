package swarm

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

type ReapRunReport struct {
	Killed  int
	Skipped int
	Errors  int
}

func ReapRunSessions(ctx context.Context, recordsPath string, kill TmuxKiller) ReapRunReport {
	var rep ReapRunReport
	recs, err := sessionrecord.ReadAll(recordsPath)
	if err != nil {
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
