package triagecap

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
)

func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate {
	return readRankedBacklog(evolveDir, isProtected, time.Now(), os.Stderr)
}

func readRankedBacklog(evolveDir string, isProtected func(string) bool, now time.Time, loopLog io.Writer) []FleetCandidate {
	rank, warnings := rankinputs.Load(evolveDir, now)
	lifecycle := readOnlyLifecycle(evolveDir)
	scan, err := inboxbatch.ScanDir(lifecycle.InboxDir)
	if err != nil {
		fmt.Fprintf(loopLog, "[triagecap] WARN inbox backlog unreadable: %v\n", err)
		return nil
	}
	for _, w := range scan.Warnings {
		fmt.Fprintf(loopLog, "[triagecap] %s\n", w)
	}
	queue := scan.Items
	for _, w := range append(warnings, inboxrank.ClassWarnings(queue, rank.Config)...) {
		fmt.Fprintf(loopLog, "[triagecap] WARN inbox rank: %s\n", w)
	}
	ranked := inboxmover.RankLaneMenu(lifecycle, queue, isProtected, rank).Ranked
	candidates := make([]FleetCandidate, len(ranked))
	for i, r := range ranked {
		candidates[i] = FleetCandidate{ID: r.Item.ID, Weight: r.Item.Weight, Files: r.Item.Files}
	}
	return candidates
}
