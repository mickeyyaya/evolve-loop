package triagecap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// SelectWaveSeedTopN returns SelectFleetWidthTopN over the inbox backlog.
func SelectWaveSeedTopN(evolveDir string, count int, isProtected func(string) bool) []FleetCandidate {
	return SelectFleetWidthTopN(ReadInboxBacklog(evolveDir, isProtected), count)
}

// ReadInboxBacklog reads <evolveDir>/inbox/*.json in filename order, skipping bad, id-less and console-routed items.
// Console items are dropped here so the seed backfills with dispatchable work instead of planning zero lanes.
// isProtected is the lane-routing predicate at the roots (cmd/evolve laneForbidden); nil disables only the files-derived routing rule.
func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate {
	entries, _ := filepath.Glob(filepath.Join(evolveDir, "inbox", "*.json"))
	sort.Strings(entries)
	candidates := make([]FleetCandidate, 0, len(entries))
	for _, p := range entries {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var doc inboxbatch.Item
		if json.Unmarshal(raw, &doc) != nil || doc.ID == "" {
			continue
		}
		if routed, _ := inboxbatch.ConsoleRouted(doc, isProtected); routed {
			continue
		}
		candidates = append(candidates, FleetCandidate{ID: doc.ID, Weight: doc.Weight, Files: doc.Files, Declared: doc.DeclaredSurface()})
	}
	return candidates
}
