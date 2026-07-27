package inboxbatch

// classify_campaign_partition_test.go — campaign is a PARTITION, not a signal.
// The operator's campaign field is the explicit "this is one initiative"
// declaration; an inferred file-area or dep edge must never merge two DISTINCT
// non-empty campaigns into one cluster. Measured on the live 84-item backlog
// (2026-07-28): a chain of small shared areas fused 17 campaigns into one
// ~37-item cluster rendered as batches 1-9, each marked "run the previous
// batch first" — a 9-cycle serialized chain over unrelated initiatives.

import (
	"strings"
	"testing"
)

// TestClassify_TwoCampaignsSharingAreaNeverMerge pins the partition core: two
// items in DIFFERENT non-empty campaigns sharing a deep (>= minAreaDepth) file
// area stay in separate batches — the operator's declaration dominates the
// inferred signal.
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

// TestClassify_SameCampaignStillClustersAcrossAreas guards the other half: the
// partition must not weaken campaign binding itself — same-campaign items with
// no shared files still cluster.
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

// TestClassify_CampaignlessItemsKeepAreaClustering pins the no-regression
// acceptance: items with NO campaign keep clustering on file-area/dep exactly
// as today — the partition constrains only distinct non-empty campaigns.
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

// TestClassify_CampaignlessBridgeCannotFuseTwoCampaigns closes the transitive
// hole a pairwise edge filter would leave open: a campaign-less item M sharing
// an area with BOTH camp-x's A and camp-y's B must not become the bridge that
// unions the two campaigns. M joins exactly one of them; every batch holds
// items from at most ONE non-empty campaign.
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

// TestClassify_CampaignPartitionIsDeterministic pins determinism under the
// partition: the guard makes union outcomes ORDER-dependent (which cluster a
// campaign-less bridge joins depends on which edge unions first), and two of
// the three rules emit edges from map iteration — so Classify must impose a
// deterministic edge order itself. Same items, same config, same batches,
// every run.
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
