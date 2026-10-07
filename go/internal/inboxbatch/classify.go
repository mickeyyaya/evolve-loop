package inboxbatch

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// DefaultMaxItems caps a batch when Config.MaxItems is unset.
const DefaultMaxItems = 4

// Config parameterizes Classify; the zero value uses the compiled defaults.
type Config struct {
	// MaxItems <= 0 means DefaultMaxItems.
	MaxItems int
	// Rules nil means DefaultRules().
	Rules []Rule
	Order func([]Item) []Item
}

type Batch struct {
	Items   []Item
	Reasons []string
	// DependsOnPrev marks a later chunk of a split cluster: run the previous batch first.
	DependsOnPrev bool
}

func Classify(items []Item, cfg Config) []Batch {
	if len(items) == 0 {
		return nil
	}
	maxItems := cfg.MaxItems
	if maxItems <= 0 {
		maxItems = DefaultMaxItems
	}
	rules := cfg.Rules
	if rules == nil {
		rules = DefaultRules()
	}
	clusters, reasons := clusterByRules(items, rules)
	position := rankPositions(items, cfg.Order)
	var out []rankedCluster
	for root, member := range clusters {
		ordered := rankOrder(member, position)
		out = append(out, rankedCluster{best: position[ordered[0]], chunks: chunk(itemsAt(items, ordered), dedupSorted(reasons[root]), maxItems)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].best < out[j].best })
	var batches []Batch
	for _, co := range out {
		batches = append(batches, co.chunks...)
	}
	return batches
}

func clusterByRules(items []Item, rules []Rule) (clusters map[int][]int, reasons map[int][]string) {
	uf := newUnionFind(len(items))
	campaignOfRoot := make([]string, len(items))
	for i, it := range items {
		campaignOfRoot[i] = strings.TrimSpace(it.Campaign)
	}
	signalsByRoot := map[int][]string{}
	for _, e := range sortedRuleEdges(rules, items) {
		ra, rb := uf.find(e.A), uf.find(e.B)
		if ra != rb {
			ca, cb := campaignOfRoot[ra], campaignOfRoot[rb]
			if ca != "" && cb != "" && ca != cb {
				continue
			}
			uf.union(ra, rb)
			if merged := uf.find(ra); campaignOfRoot[merged] == "" {
				campaignOfRoot[merged] = ca + cb
			}
		}
		signalsByRoot[uf.find(e.A)] = append(signalsByRoot[uf.find(e.A)], e.Reason)
	}
	clusters = map[int][]int{}
	for i := range items {
		clusters[uf.find(i)] = append(clusters[uf.find(i)], i)
	}
	reasons = map[int][]string{}
	for root, rs := range signalsByRoot {
		reasons[uf.find(root)] = append(reasons[uf.find(root)], rs...)
	}
	return clusters, reasons
}

type rankedCluster struct {
	chunks []Batch
	best   int
}

func itemsAt(items []Item, indices []int) []Item {
	picked := make([]Item, len(indices))
	for i, idx := range indices {
		picked[i] = items[idx]
	}
	return picked
}

func chunk(members []Item, reasons []string, maxItems int) []Batch {
	var batches []Batch
	for start := 0; start < len(members); start += maxItems {
		end := min(start+maxItems, len(members))
		batches = append(batches, Batch{Items: members[start:end:end], Reasons: reasons, DependsOnPrev: start > 0})
	}
	return batches
}

func rankPositions(items []Item, order func([]Item) []Item) []int {
	position := make([]int, len(items))
	if order == nil {
		for i := range position {
			position[i] = i
		}
		return position
	}
	ranked := order(slices.Clone(items))
	slots := map[ItemKey][]int{}
	for at, it := range ranked {
		slots[it.Key()] = append(slots[it.Key()], at)
	}
	for i, it := range items {
		key := it.Key()
		if len(slots[key]) == 0 {
			position[i] = len(ranked) + i
			continue
		}
		position[i], slots[key] = slots[key][0], slots[key][1:]
	}
	return position
}

func rankOrder(member, position []int) []int {
	ordered := slices.Clone(member)
	slices.SortFunc(ordered, func(a, b int) int { return position[a] - position[b] })
	return ordered
}

// sortedRuleEdges collects every rule's edges in one canonical (A, B, Reason) order.
func sortedRuleEdges(rules []Rule, items []Item) []Edge {
	var edges []Edge
	for _, r := range rules {
		edges = append(edges, r.Edges(items)...)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].A != edges[j].A {
			return edges[i].A < edges[j].A
		}
		if edges[i].B != edges[j].B {
			return edges[i].B < edges[j].B
		}
		return edges[i].Reason < edges[j].Reason
	})
	return edges
}

// maxRenderedReasons caps the binding signals shown per batch line.
const maxRenderedReasons = 3

func RenderMarkdown(batches []Batch, label func(Item) string) string {
	var b strings.Builder
	for n, batch := range batches {
		ids := make([]string, len(batch.Items))
		for i, it := range batch.Items {
			ids[i] = it.ID
		}
		cont := ""
		if batch.DependsOnPrev {
			cont = " (continuation — run the previous batch first)"
		}
		fmt.Fprintf(&b, "- batch %d (%s)%s: %s\n", n+1, compactReasons(batch.Reasons), cont, strings.Join(ids, ", "))
		for _, it := range batch.Items {
			if label != nil {
				fmt.Fprintf(&b, "  - %s: %s\n", it.ID, label(it))
			}
		}
	}
	return b.String()
}

func compactReasons(rs []string) string {
	if len(rs) == 0 {
		return "no shared signal"
	}
	if len(rs) <= maxRenderedReasons {
		return strings.Join(rs, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(rs[:maxRenderedReasons], ", "), len(rs)-maxRenderedReasons)
}

func dedupSorted(rs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rs {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

type unionFind struct{ parent []int }

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{parent: p}
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[rb] = ra
	}
}
