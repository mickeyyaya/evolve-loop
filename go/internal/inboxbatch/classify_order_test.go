package inboxbatch

import (
	"slices"
	"strings"
	"testing"
)

func orderBy(ids ...string) func([]Item) []Item {
	return func(items []Item) []Item {
		ordered := slices.Clone(items)
		slices.SortStableFunc(ordered, func(a, b Item) int { return slices.Index(ids, a.ID) - slices.Index(ids, b.ID) })
		return ordered
	}
}

func batchIDs(batches []Batch) string {
	lines := make([]string, len(batches))
	for i, b := range batches {
		lines[i] = ids(b)
	}
	return strings.Join(lines, " | ")
}

func TestClassify_ClustersRankByTheirBestRankedMember(t *testing.T) {
	items := []Item{
		item("heavy", 0.9),
		item("camp-low", 0.1, withCampaign("c")),
		item("camp-mid", 0.5, withCampaign("c")),
		item("solo", 0.7),
	}
	got := batchIDs(Classify(items, Config{MaxItems: 4, Order: orderBy("camp-mid", "solo", "heavy", "camp-low")}))
	if got != "camp-mid,camp-low | solo | heavy" {
		t.Errorf("batches = %s, want the cluster led by the top-ranked item first, members in rank order", got)
	}
}

func TestClassify_RunsTheOrderSeamOnceOverEveryItem(t *testing.T) {
	calls, seen := 0, 0
	order := func(items []Item) []Item {
		calls++
		seen = len(items)
		return items
	}
	items := []Item{item("a", 0.5, withCampaign("c")), item("b", 0.5, withCampaign("c")), item("d", 0.5)}
	Classify(items, Config{Order: order})
	if calls != 1 || seen != 3 {
		t.Errorf("Order ran %d time(s) over %d item(s), want once over all 3", calls, seen)
	}
}

func TestClassify_WithoutAnOrderKeepsTheCallersOrder(t *testing.T) {
	items := []Item{item("light", 0.1), item("heavy", 0.9, withCampaign("c")), item("mid", 0.5, withCampaign("c"))}
	if got := batchIDs(Classify(items, Config{})); got != "light | heavy,mid" {
		t.Errorf("batches = %s, want the caller's order: Classify never ranks by weight itself", got)
	}
}

func TestClassify_AnItemTheOrderDropsFollowsEveryRankedItem(t *testing.T) {
	items := []Item{item("dropped", 0.9), item("ranked", 0.1)}
	drop := func([]Item) []Item { return []Item{item("ranked", 0.1)} }
	if got := batchIDs(Classify(items, Config{Order: drop})); got != "ranked | dropped" {
		t.Errorf("batches = %s, want an unranked item after the ranked ones, never lost", got)
	}
}

func TestClassify_SameIDItemsKeepDistinctPlacesThroughTheOrder(t *testing.T) {
	items := []Item{{ID: "dup", Path: "a.json"}, {ID: "dup", Path: "b.json"}, {ID: "x", Path: "c.json"}}
	reverse := func(in []Item) []Item {
		out := slices.Clone(in)
		slices.Reverse(out)
		return out
	}
	batches := Classify(items, Config{Order: reverse})
	var paths []string
	for _, b := range batches {
		for _, it := range b.Items {
			paths = append(paths, it.Path)
		}
	}
	if strings.Join(paths, ",") != "c.json,b.json,a.json" {
		t.Errorf("paths = %v, want the injected order item by item", paths)
	}
}

func TestRenderMarkdown_LabelsEachItemOnItsOwnLineBelowTheBatch(t *testing.T) {
	batches := Classify([]Item{item("a", 0.9, withCampaign("c")), item("b", 0.5, withCampaign("c"))}, Config{})
	out := RenderMarkdown(batches, func(it Item) string { return "score for " + it.ID })
	want := "- batch 1 (campaign c): a, b\n  - a: score for a\n  - b: score for b\n"
	if out != want {
		t.Errorf("RenderMarkdown =\n%q\nwant\n%q", out, want)
	}
	if bare := RenderMarkdown(batches, nil); bare != "- batch 1 (campaign c): a, b\n" {
		t.Errorf("a nil label renders the batch line alone: %q", bare)
	}
}
