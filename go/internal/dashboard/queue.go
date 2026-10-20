package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

var lifecycleDirs = []string{"consumed", "processing", "retry", "processed"}

func inboxDir(root string) string { return filepath.Join(paths.EvolveDirOf(root), "inbox") }

func readQueue(root string, now time.Time) (QueueSummary, []string) {
	inbox := inboxDir(root)
	scan, err := inboxbatch.ScanDir(inbox)
	var q QueueSummary
	if err != nil {
		return q, []string{"inbox: " + err.Error()}
	}
	items := scan.Items
	var warnings []string
	for _, w := range scan.Warnings {
		warnings = append(warnings, "inbox: "+w.String())
	}
	in, rankWarnings := rankinputs.Load(paths.EvolveDirOf(root), now)
	for _, w := range rankWarnings {
		warnings = append(warnings, "inbox rank: "+w)
	}
	q.Pending = make([]QueueItem, 0, len(items))
	for _, r := range in.Order(items, items) {
		it := r.Item
		q.Pending = append(q.Pending, QueueItem{ID: it.ID, Title: it.Title, Kind: it.Kind, Class: it.Class,
			Route: it.Route, Priority: it.Priority, Weight: it.Weight, Score: r.Breakdown.Score})
	}
	q.Consumed = countJSON(filepath.Join(inbox, "consumed"))
	q.Processing = countJSON(filepath.Join(inbox, "processing"))
	q.Retry = countJSON(filepath.Join(inbox, "retry"))
	q.Processed = countJSON(filepath.Join(inbox, "processed"))
	return q, warnings
}

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
