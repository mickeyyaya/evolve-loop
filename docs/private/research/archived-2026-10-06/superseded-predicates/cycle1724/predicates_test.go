//go:build acs

package cycle1724

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"testing"
)

func TestC1724_002_ClassifyIgnoresDepsForOrdering(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "parent", Weight: 0.2, Campaign: "camp-x"},
		{ID: "child", Weight: 0.95, Campaign: "camp-x", Deps: []string{"parent"}},
	}
	batches := inboxbatch.Classify(items, inboxbatch.Config{MaxItems: 4})
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1 (campaign still unions them); got %+v", len(batches), batches)
	}
	if got := batchIDs(batches[0]); got != "child,parent" {
		t.Errorf("order = %s, want child,parent (weight-desc; topological dep ordering must be gone) — "+
			"topoOrder's deps-specific consumption (classify.go:142-233) is dead under W3 and must be removed",
			got)
	}
}
