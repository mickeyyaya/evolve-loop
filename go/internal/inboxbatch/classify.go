package inboxbatch

import (
	"fmt"
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
}

// Batch is one cycle's unit of work: deps-first items, binding reasons, and the max member weight.
type Batch struct {
	Items   []Item
	Reasons []string
	Weight  float64
	// DependsOnPrev marks a later chunk of a split cluster: run the previous batch first.
	DependsOnPrev bool
}

// Classify groups items into dep-ordered, cap-split batches ranked by weight; it is pure and deterministic.
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

	uf := newUnionFind(len(items))
	// Campaign is a partition: an inferred edge never merges two distinct
	// non-empty campaigns, and each root's claim keeps the guard transitive.
	clusterCampaign := make([]string, len(items))
	for i, it := range items {
		clusterCampaign[i] = strings.TrimSpace(it.Campaign)
	}
	reasons := map[int][]string{} // root → binding signals
	// The partition guard makes union outcomes order-dependent, so determinism rests on the sorted edges.
	for _, e := range sortedRuleEdges(rules, items) {
		ra, rb := uf.find(e.A), uf.find(e.B)
		if ra != rb {
			ca, cb := clusterCampaign[ra], clusterCampaign[rb]
			if ca != "" && cb != "" && ca != cb {
				continue
			}
			uf.union(ra, rb)
			// An empty merged claim means one side was empty, so ca+cb is the other side's claim.
			if merged := uf.find(ra); clusterCampaign[merged] == "" {
				clusterCampaign[merged] = ca + cb
			}
		}
		reasons[uf.find(e.A)] = append(reasons[uf.find(e.A)], e.Reason)
	}

	// A later union can move a root, so fold every reason list into its final root.
	clusters := map[int][]int{}
	for i := range items {
		clusters[uf.find(i)] = append(clusters[uf.find(i)], i)
	}
	finalReasons := map[int][]string{}
	for root, rs := range reasons {
		finalReasons[uf.find(root)] = append(finalReasons[uf.find(root)], rs...)
	}

	// Rank whole clusters, not chunks, so a continuation always follows its predecessor.
	type clusterOut struct {
		chunks  []Batch
		weight  float64
		firstID string
	}
	var out []clusterOut
	for root, member := range clusters {
		ordered := topoOrder(items, member)
		rs := dedupSorted(finalReasons[root])
		co := clusterOut{firstID: items[ordered[0]].ID}
		for start := 0; start < len(ordered); start += maxItems {
			end := start + maxItems
			if end > len(ordered) {
				end = len(ordered)
			}
			chunk := make([]Item, 0, end-start)
			w := 0.0
			for _, idx := range ordered[start:end] {
				chunk = append(chunk, items[idx])
				if items[idx].Weight > w {
					w = items[idx].Weight
				}
			}
			if w > co.weight {
				co.weight = w
			}
			co.chunks = append(co.chunks, Batch{
				Items:         chunk,
				Reasons:       rs,
				Weight:        w,
				DependsOnPrev: start > 0,
			})
		}
		out = append(out, co)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return weightDescThenID(out[i].weight, out[i].firstID, out[j].weight, out[j].firstID)
	})
	var batches []Batch
	for _, co := range out {
		batches = append(batches, co.chunks...)
	}
	return batches
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

// topoOrder returns member indices deps-first (Kahn); a dep cycle appends the remainder in weight-then-id order.
func topoOrder(items []Item, member []int) []int {
	inCluster := map[string]int{}
	for _, i := range member {
		inCluster[items[i].ID] = i
	}
	indeg := map[int]int{}
	dependents := map[int][]int{}
	for _, i := range member {
		for _, d := range items[i].Deps {
			if j, ok := resolveRef(d, inCluster); ok && j != i {
				indeg[i]++
				dependents[j] = append(dependents[j], i)
			}
		}
	}
	less := func(a, b int) bool {
		return weightDescThenID(items[a].Weight, items[a].ID, items[b].Weight, items[b].ID)
	}
	var ready []int
	for _, i := range member {
		if indeg[i] == 0 {
			ready = append(ready, i)
		}
	}
	sort.Slice(ready, func(x, y int) bool { return less(ready[x], ready[y]) })

	out := make([]int, 0, len(member))
	for len(ready) > 0 {
		i := ready[0]
		ready = ready[1:]
		out = append(out, i)
		for _, dep := range dependents[i] {
			indeg[dep]--
			if indeg[dep] == 0 {
				ready = append(ready, dep)
			}
		}
		sort.Slice(ready, func(x, y int) bool { return less(ready[x], ready[y]) })
	}
	if len(out) < len(member) {
		var rest []int
		seen := map[int]bool{}
		for _, i := range out {
			seen[i] = true
		}
		for _, i := range member {
			if !seen[i] {
				rest = append(rest, i)
			}
		}
		sort.Slice(rest, func(x, y int) bool { return less(rest[x], rest[y]) })
		out = append(out, rest...)
	}
	return out
}

// maxRenderedReasons caps the binding signals shown per batch line.
const maxRenderedReasons = 3

// RenderMarkdown formats one line per batch for the triage prompt and the CLI; no batches render "".
func RenderMarkdown(batches []Batch) string {
	if len(batches) == 0 {
		return ""
	}
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
		fmt.Fprintf(&b, "- batch %d (weight %.2f; %s)%s: %s\n",
			n+1, batch.Weight, compactReasons(batch.Reasons), cont, strings.Join(ids, ", "))
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

// weightDescThenID is the one ordering topoOrder's ready set and Classify's ranking share.
func weightDescThenID(weightA float64, idA string, weightB float64, idB string) bool {
	if weightA != weightB {
		return weightA > weightB
	}
	return idA < idB
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
