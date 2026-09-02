package dashboard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// lifecycleDirs are the inbox sub-directories the mover promotes items into.
// Only their counts are shown; the pending pool is the actionable list.
var lifecycleDirs = []string{"consumed", "processing", "retry", "processed"}

// inboxDir is <root>/.evolve/inbox, the pending pool inboxbatch.LoadDir reads.
func inboxDir(root string) string { return filepath.Join(paths.EvolveDirOf(root), "inbox") }

// readQueue projects the inbox through inboxbatch.LoadDir (the same loader
// triage consumes, so the dashboard cannot disagree with it about what an item
// is) and counts the lifecycle sub-directories.
func readQueue(root string) (QueueSummary, []string) {
	inbox := inboxDir(root)
	items, warnings, err := inboxbatch.LoadDir(inbox)
	var q QueueSummary
	if err != nil {
		return q, []string{"inbox: " + err.Error()}
	}
	for i := range warnings {
		warnings[i] = "inbox: " + warnings[i]
	}
	q.Pending = make([]QueueItem, 0, len(items))
	for _, it := range items {
		q.Pending = append(q.Pending, QueueItem{ID: it.ID, Title: it.Title, Kind: it.Kind, Class: it.Class,
			Route: it.Route, Priority: it.Priority, Weight: it.Weight})
	}
	sort.SliceStable(q.Pending, func(i, j int) bool {
		if q.Pending[i].Weight != q.Pending[j].Weight {
			return q.Pending[i].Weight > q.Pending[j].Weight
		}
		return q.Pending[i].ID < q.Pending[j].ID
	})
	q.Consumed = countJSON(filepath.Join(inbox, "consumed"))
	q.Processing = countJSON(filepath.Join(inbox, "processing"))
	q.Retry = countJSON(filepath.Join(inbox, "retry"))
	q.Processed = countJSON(filepath.Join(inbox, "processed"))
	return q, warnings
}

// countJSON counts *.json files directly under dir; a missing dir counts 0.
func countJSON(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			n++
		}
	}
	return n
}
