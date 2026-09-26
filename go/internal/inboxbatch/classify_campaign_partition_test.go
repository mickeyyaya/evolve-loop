package inboxbatch

import (
	"strings"
	"testing"
)

func TestClassify_TwoCampaignsSharingAreaNeverMerge(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x"), withFiles("go/internal/router/a.go")),
		item("b", 0.8, withCampaign("camp-y"), withFiles("go/internal/router/b.go")),
	}
	batches := Classify(items, Config{})
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 — a file-area edge merged two distinct campaigns (camp-x + camp-y); campaign is a partition, not a signal", len(batches))
	}
}

func TestClassify_SameCampaignStillClustersAcrossAreas(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x"), withFiles("go/internal/router/a.go")),
		item("b", 0.8, withCampaign("camp-x"), withFiles("docs/operations/b.md")),
	}
	batches := Classify(items, Config{})
	if len(batches) != 1 || ids(batches[0]) != "a,b" {
		t.Fatalf("same-campaign pair must stay one batch; got %d batches", len(batches))
	}
}

func TestClassify_CampaignlessItemsKeepAreaClustering(t *testing.T) {
	items := []Item{
		item("a", 0.9, withFiles("go/internal/subagent/a.go")),
		item("b", 0.8, withFiles("go/internal/subagent/b.go")),
	}
	batches := Classify(items, Config{})
	if len(batches) != 1 || ids(batches[0]) != "a,b" {
		t.Fatalf("campaignless same-area pair must still cluster; got %d batches", len(batches))
	}
}

func TestClassify_CampaignlessBridgeCannotFuseTwoCampaigns(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x"), withFiles("go/internal/router/a.go")),
		item("m", 0.7, withFiles("go/internal/router/m.go", "go/internal/bridge/m.go")),
		item("b", 0.8, withCampaign("camp-y"), withFiles("go/internal/bridge/b.go")),
	}
	batches := Classify(items, Config{})
	for _, b := range batches {
		camps := map[string]bool{}
		for _, it := range b.Items {
			if c := strings.TrimSpace(it.Campaign); c != "" {
				camps[c] = true
			}
		}
		if len(camps) > 1 {
			t.Fatalf("batch %s spans %d campaigns — the campaign-less bridge m fused camp-x and camp-y through union-find transitivity", ids(b), len(camps))
		}
	}
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (m attaches to exactly one campaign's cluster)", len(batches))
	}
}

func TestClassify_CampaignPartitionIsDeterministic(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x"), withFiles("go/internal/router/a.go")),
		item("m", 0.7, withFiles("go/internal/router/m.go", "go/internal/bridge/m.go")),
		item("b", 0.8, withCampaign("camp-y"), withFiles("go/internal/bridge/b.go")),
		item("n", 0.6, withFiles("go/internal/bridge/n.go")),
	}
	first := renderAll(Classify(items, Config{}))
	for i := 0; i < 20; i++ {
		if got := renderAll(Classify(items, Config{})); got != first {
			t.Fatalf("run %d diverged:\n%s\nvs first:\n%s", i, got, first)
		}
	}
}

func renderAll(batches []Batch) string {
	var sb strings.Builder
	for _, b := range batches {
		sb.WriteString(ids(b))
		sb.WriteString("\n")
	}
	return sb.String()
}
