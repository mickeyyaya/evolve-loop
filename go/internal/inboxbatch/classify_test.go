package inboxbatch

// classify_test.go — the deterministic grouping core. Rules (Strategy) emit
// edges; union-find clusters; dep-topological order within a batch; oversized
// clusters chunk sequentially in topo order (a dep is always in an earlier or
// the same chunk); batches rank by their max item weight. Everything is pure
// and deterministic — the LLM's job stays choosing WHICH batch, never HOW to
// group.

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

// TestClassify_CampaignGroups: a shared campaign field binds items into one
// batch regardless of files.
func TestClassify_CampaignGroups(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x")),
		item("b", 0.5, withCampaign("camp-x")),
		item("c", 0.7), // no signals — singleton
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (campaign pair + singleton)", len(batches))
	}
	// Rank by max member weight: camp-x batch (0.9) before singleton c (0.7).
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

// TestClassify_HubFileAreaDoesNotBlob: an area referenced by MORE than
// hubAreaMaxItems items is a hub (go/internal/core appears in half the real
// backlog) and carries no discriminative signal — it must not fuse otherwise
// unrelated groups into a mega-cluster. Real-backlog validation drove this:
// without the hub cap, 55 of 61 items fused into one cluster.
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
	// hub area spans 7 > hubAreaMaxItems → ignored; router pair + guards pair
	// + three singletons.
	if len(batches) != 5 {
		t.Fatalf("batches = %d, want 5 (hub area must not bind); got %+v", len(batches), batches)
	}
	// Equal max weights (0.6) tie-break by first id: guards pair before router
	// pair; both REAL shared areas still bind.
	if got := ids(batches[0]); got != "g1,g2" {
		t.Errorf("batch[0] = %s, want g1,g2 (weight tie → id order)", got)
	}
	if got := ids(batches[1]); got != "r1,r2" {
		t.Errorf("batch[1] = %s, want r1,r2 (real shared area still binds)", got)
	}
}

// TestClassify_FileAreaGroups: items touching the same package area group,
// distinct areas stay apart.
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

// TestClassify_ConnectsToDoesNotClusterByDefault: connects_to is PROSE
// navigation ("link liberally" is the inbox house style) — real-backlog
// validation showed its transitive closure chains distinct campaigns into one
// mega-cluster. It is deliberately NOT a default grouping signal; the
// ConnectsRule remains available for Config.Rules opt-in.
func TestClassify_ConnectsToDoesNotClusterByDefault(t *testing.T) {
	items := []Item{
		item("alpha", 0.8, withConnects("beta (shares the digest surface)")),
		item("beta", 0.3),
	}
	if got := len(Classify(items, Config{MaxItems: 4})); got != 2 {
		t.Fatalf("batches = %d, want 2 (connects_to alone must not bind)", got)
	}
	// Opt-in: adding ConnectsRule to the defaults binds them, prose resolved
	// by id prefix and unresolvable prose ignored.
	withLink := Classify(items, Config{MaxItems: 4, Rules: append(DefaultRules(), ConnectsRule{})})
	if len(withLink) != 1 || ids(withLink[0]) != "alpha,beta" {
		t.Fatalf("opt-in ConnectsRule: batches = %+v, want one alpha,beta batch", withLink)
	}
}

// TestClassify_DepsOrderTopologically: within a batch, a dep precedes its
// dependent even when the dependent outweighs it.
func TestClassify_DepsOrderTopologically(t *testing.T) {
	items := []Item{
		item("child", 0.95, withDeps("parent")),
		item("parent", 0.2),
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1 (dep edge unions them)", len(batches))
	}
	if got := ids(batches[0]); got != "parent,child" {
		t.Errorf("order = %s, want parent,child (topological, not weight)", got)
	}
	if batches[0].Weight != 0.95 {
		t.Errorf("batch weight = %v, want the max member weight 0.95", batches[0].Weight)
	}
}

// TestClassify_DepOnMissingItemIsSatisfied: deps naming consumed/absent items
// don't block or edge — treated as already satisfied.
func TestClassify_DepOnMissingItemIsSatisfied(t *testing.T) {
	items := []Item{item("solo", 0.5, withDeps("already-shipped-thing"))}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 1 || ids(batches[0]) != "solo" {
		t.Fatalf("batches = %+v, want the solo singleton", batches)
	}
}

// TestClassify_OversizedClusterChunksInTopoOrder: a cluster above MaxItems
// splits sequentially — deps land in an earlier or the same chunk, and later
// chunks are flagged as depending on earlier ones.
func TestClassify_OversizedClusterChunksInTopoOrder(t *testing.T) {
	items := []Item{
		item("d1", 0.9, withCampaign("big")),
		item("d2", 0.8, withCampaign("big"), withDeps("d1")),
		item("d3", 0.7, withCampaign("big"), withDeps("d2")),
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
		t.Error("chunk 2 must be flagged DependsOnPrev (d3's dep chain crosses the split)")
	}
}

// TestClassify_ContinuationNeverOutranksPredecessor — go-reviewer HIGH
// regression (2026-07-16, their exact counter-example): a long LOW-weight dep
// chain gating one HIGH-weight item pushes it into a continuation chunk; the
// old flat weight sort ranked that continuation ABOVE its own predecessor.
// Batches now rank as CLUSTER UNITS (cluster max weight), chunks staying
// adjacent and predecessor-first — so the render's "run the previous batch
// first" note always points at the line above.
func TestClassify_ContinuationNeverOutranksPredecessor(t *testing.T) {
	items := []Item{
		item("E", 0.5, withCampaign("chain")), // independent, mid-weight
		item("A1", 0.05, withCampaign("chain")),
		item("A2", 0.04, withCampaign("chain"), withDeps("A1")),
		item("A3", 0.03, withCampaign("chain"), withDeps("A2")),
		item("A4", 0.02, withCampaign("chain"), withDeps("A3")),
		item("C", 0.99, withCampaign("chain"), withDeps("A4")),
		item("other", 0.7), // unrelated singleton — must rank AFTER the whole 0.99 cluster
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3 (two chunks + singleton)", len(batches))
	}
	if batches[0].DependsOnPrev {
		t.Fatalf("batch[0] is a continuation — the predecessor must come first; got %s", ids(batches[0]))
	}
	if !batches[1].DependsOnPrev {
		t.Fatalf("batch[1] must be the continuation adjacent to its predecessor; got %s", ids(batches[1]))
	}
	if got := ids(batches[2]); got != "other" {
		t.Errorf("the unrelated singleton must rank after the 0.99 cluster's BOTH chunks; batch[2] = %s", got)
	}
}

// TestClassify_DepCycleFallsBackDeterministically: a dep cycle (bad data) must
// not hang or drop items — falls back to weight-then-id order, all items kept.
func TestClassify_DepCycleFallsBackDeterministically(t *testing.T) {
	items := []Item{
		item("x", 0.5, withDeps("y")),
		item("y", 0.5, withDeps("x")),
	}
	batches := Classify(items, Config{MaxItems: 4})
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(batches))
	}
	if got := ids(batches[0]); got != "x,y" {
		t.Errorf("cycle fallback order = %s, want x,y (weight tie → id)", got)
	}
}

// TestClassify_ZeroMaxUsesDefault: Config{} means the compiled default cap —
// callers never construct an accidental unbounded batch.
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

// TestRenderMarkdown_EmptyIsEmpty: no items → empty string (the byte-identical
// prompt pin for the triage injection).
func TestRenderMarkdown_EmptyIsEmpty(t *testing.T) {
	if got := RenderMarkdown(nil); got != "" {
		t.Errorf("RenderMarkdown(nil) = %q, want empty", got)
	}
}

// TestRenderMarkdown_ListsBatchesWithIDsAndReasons: the rendered section names
// each batch's ids, weight, and binding reason — what triage needs to pick a
// WHOLE batch as top_n.
func TestRenderMarkdown_ListsBatchesWithIDsAndReasons(t *testing.T) {
	items := []Item{
		item("a", 0.9, withCampaign("camp-x")),
		item("b", 0.5, withCampaign("camp-x")),
	}
	out := RenderMarkdown(Classify(items, Config{MaxItems: 4}))
	for _, want := range []string{"batch 1", "a", "b", "campaign", "0.90"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered section missing %q:\n%s", want, out)
		}
	}
}

// TestRenderMarkdown_CompactsLongReasonLists: real clusters carry many binding
// signals; the render caps at a few and summarizes the rest — a prompt
// section, not a forensic dump.
func TestRenderMarkdown_CompactsLongReasonLists(t *testing.T) {
	b := Batch{
		Items:  []Item{{ID: "x", Weight: 0.5}},
		Weight: 0.5,
		Reasons: []string{
			"campaign a", "campaign b", "dep p→q", "dep q→r", "file-area go/internal/x", "file-area go/internal/y",
		},
	}
	out := RenderMarkdown([]Batch{b})
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
