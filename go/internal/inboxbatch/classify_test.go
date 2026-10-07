package inboxbatch

import (
	"strings"
	"testing"
)

func item(id string, weight float64, mut ...func(*Item)) Item {
	it := Item{ID: id, Title: "t-" + id, Weight: weight}
	for _, m := range mut {
		m(&it)
	}
	return it
}

func withCampaign(c string) func(*Item) { return func(i *Item) { i.Campaign = c } }
func withFiles(f ...string) func(*Item) { return func(i *Item) { i.Files = f } }
func withConnects(c ...string) func(*Item) {
	return func(i *Item) { i.ConnectsTo = c }
}
func withDeps(d ...string) func(*Item) { return func(i *Item) { i.Deps = d } }

func TestClassify_CampaignGroups(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x")),
		item("b", 0.5, withCampaign("camp-x")),
		item("c", 0.7),
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (campaign pair + singleton)", len(batches))
	}
	if got := ids(batches[0]); got != "a,b" {
		t.Errorf("batch[0] = %s, want a,b", got)
	}
	if got := ids(batches[1]); got != "c" {
		t.Errorf("batch[1] = %s, want c", got)
	}
	if !strings.Contains(strings.Join(batches[0].Reasons, " "), "campaign") {
		t.Errorf("batch reasons must explain the binding signal; got %v", batches[0].Reasons)
	}
}

func TestClassify_HubFileAreaDoesNotBlob(t *testing.T) {
	hub := "go/internal/core/x.go"
	items := []Item{
		item("r1", 0.6, withFiles("go/internal/router/a.go", hub)),
		item("r2", 0.5, withFiles("go/internal/router/b.go", hub)),
		item("g1", 0.6, withFiles("go/internal/guards/a.go", hub)),
		item("g2", 0.5, withFiles("go/internal/guards/b.go", hub)),
		item("s1", 0.4, withFiles(hub)),
		item("s2", 0.4, withFiles(hub)),
		item("s3", 0.4, withFiles(hub)),
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 5 {
		t.Fatalf("batches = %d, want 5 (hub area must not bind); got %+v", len(batches), batches)
	}
	if got := ids(batches[0]); got != "r1,r2" {
		t.Errorf("batch[0] = %s, want r1,r2 (real shared area still binds, in the caller's order)", got)
	}
	if got := ids(batches[1]); got != "g1,g2" {
		t.Errorf("batch[1] = %s, want g1,g2 (real shared area still binds)", got)
	}
}

func TestClassify_FileAreaGroups(t *testing.T) {
	items := []Item{
		item("r1", 0.6, withFiles("go/internal/router/digest.go")),
		item("r2", 0.4, withFiles("go/internal/router/floor.go", "docs/x.md")),
		item("g1", 0.5, withFiles("go/internal/guards/role.go")),
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (router pair + guards singleton)", len(batches))
	}
	if got := ids(batches[0]); got != "r1,r2" {
		t.Errorf("batch[0] = %s, want r1,r2 (same package area)", got)
	}
}

func TestClassify_ConnectsToDoesNotClusterByDefault(t *testing.T) {
	items := []Item{
		item("alpha", 0.8, withConnects("beta (shares the digest surface)")),
		item("beta", 0.3),
	}
	if got := len(Classify(items, Config{MaxItems: 4})); got != 2 {
		t.Fatalf("batches = %d, want 2 (connects_to alone must not bind)", got)
	}
	withLink := Classify(items, Config{MaxItems: 4, Rules: append(DefaultRules(), ConnectsRule{})})
	if len(withLink) != 1 || ids(withLink[0]) != "alpha,beta" {
		t.Fatalf("opt-in ConnectsRule: batches = %+v, want one alpha,beta batch", withLink)
	}
}

func TestClassify_DepsNeitherBindNorReorderTheInjectedOrder(t *testing.T) {
	unbound := []Item{
		item("child", 0.95, withDeps("parent")),
		item("parent", 0.2),
	}
	if got := len(Classify(unbound, Config{MaxItems: 4})); got != 2 {
		t.Fatalf("batches = %d, want 2 (a Deps-only reference must not bind an edge)", got)
	}

	bound := []Item{
		item("parent", 0.2, withCampaign("camp-x")),
		item("child", 0.95, withCampaign("camp-x"), withDeps("parent")),
	}
	batches := Classify(bound, Config{MaxItems: 4, Order: orderBy("child", "parent")})
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1 (campaign still unions them)", len(batches))
	}
	if got := ids(batches[0]); got != "child,parent" {
		t.Errorf("order = %s, want child,parent (the injected order; Deps must not reorder)", got)
	}
}

func TestClassify_OversizedClusterChunksInRankOrder(t *testing.T) {
	items := []Item{
		item("d1", 0.9, withCampaign("big")),
		item("d2", 0.8, withCampaign("big")),
		item("d3", 0.7, withCampaign("big")),
	}
	batches := Classify(items, Config{MaxItems: 2})
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (3 items, cap 2)", len(batches))
	}
	if got := ids(batches[0]); got != "d1,d2" {
		t.Errorf("chunk 1 = %s, want d1,d2", got)
	}
	if got := ids(batches[1]); got != "d3" {
		t.Errorf("chunk 2 = %s, want d3", got)
	}
	if !batches[1].DependsOnPrev {
		t.Error("chunk 2 must be flagged DependsOnPrev (it is a later chunk of the same cluster)")
	}
}

func TestClassify_ZeroMaxUsesDefault(t *testing.T) {
	var items []Item
	for _, id := range []string{"a", "b", "c", "d", "e", "f"} {
		items = append(items, item(id, 0.5, withCampaign("one")))
	}
	batches := Classify(items, Config{})
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (6 items at the default cap %d)", len(batches), DefaultMaxItems)
	}
	if len(batches[0].Items) != DefaultMaxItems {
		t.Errorf("first chunk = %d items, want DefaultMaxItems=%d", len(batches[0].Items), DefaultMaxItems)
	}
}

func TestRenderMarkdown_EmptyIsEmpty(t *testing.T) {
	if got := RenderMarkdown(nil, nil); got != "" {
		t.Errorf("RenderMarkdown(nil) = %q, want empty", got)
	}
}

func TestRenderMarkdown_ListsBatchesWithIDsAndReasons(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x")),
		item("b", 0.5, withCampaign("camp-x")),
	}
	out := RenderMarkdown(Classify(items, Config{MaxItems: 4}), nil)
	for _, want := range []string{"batch 1", "a", "b", "campaign"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered section missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMarkdown_CompactsLongReasonLists(t *testing.T) {
	b := Batch{
		Items: []Item{{ID: "x", Weight: 0.5}},
		Reasons: []string{
			"campaign a", "campaign b", "dep p→q", "dep q→r", "file-area go/internal/x", "file-area go/internal/y",
		},
	}
	out := RenderMarkdown([]Batch{b}, nil)
	if !strings.Contains(out, "+3 more") {
		t.Errorf("6 reasons must compact to %d + a '+3 more' summary:\n%s", maxRenderedReasons, out)
	}
	if strings.Contains(out, "file-area go/internal/y") {
		t.Errorf("reasons beyond the cap must not render individually:\n%s", out)
	}
}

func ids(b Batch) string {
	var s []string
	for _, it := range b.Items {
		s = append(s, it.ID)
	}
	return strings.Join(s, ",")
}
