package triagecap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// SelectWaveSeedTopN returns SelectFleetWidthTopN over the inbox backlog.
func SelectWaveSeedTopN(evolveDir string, count int, isProtected func(string) bool) []FleetCandidate {
	return SelectFleetWidthTopN(ReadInboxBacklog(evolveDir, isProtected), count)
}

func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate {
	lifecycle := readOnlyLifecycle(evolveDir)
	entries, _ := filepath.Glob(filepath.Join(lifecycle.InboxDir, "*.json"))
	sort.Strings(entries)
	candidates := make([]FleetCandidate, 0, len(entries))
	for _, p := range entries {
		if doc, ok := laneMaterial(p, isProtected, lifecycle); ok {
			candidates = append(candidates, FleetCandidate{ID: doc.ID, Weight: doc.Weight, Files: doc.Files, Declared: doc.DeclaredSurface()})
		}
	}
	return candidates
}

func laneMaterial(path string, isProtected func(string) bool, lifecycle inboxmover.Options) (inboxbatch.Item, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return inboxbatch.Item{}, false
	}
	var doc inboxbatch.Item
	if json.Unmarshal(raw, &doc) != nil || doc.ID == "" {
		return inboxbatch.Item{}, false
	}
	place, _ := inboxmover.PlaceOnLaneMenu(lifecycle, doc, isProtected)
	return doc, place == inboxmover.MenuReady
}
