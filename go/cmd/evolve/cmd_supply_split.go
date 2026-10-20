package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxsplit"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
)

func splitProposer(inboxDir string, isProtected func(string) bool, stderr io.Writer) orchestrator.Proposer {
	return func(candidates []inboxbatch.Item, admissible, window int) []orchestrator.Proposal {
		items := make([]inboxsplit.Item, 0, len(candidates))
		for _, candidate := range candidates {
			item, err := readInboxItemObject(filepath.Join(inboxDir, candidate.Path))
			if err != nil {
				fmt.Fprintf(stderr, "[census] WARN: split proposal: %s: %v\n", candidate.ID, err)
				continue
			}
			items = append(items, item)
		}
		var out []orchestrator.Proposal
		for _, p := range inboxsplit.Proposals(items, admissible, window, isProtected) {
			out = append(out, orchestrator.Proposal{Item: p.OriginalID, Console: p.ConsoleID, Lane: p.LaneID})
		}
		return out
	}
}
