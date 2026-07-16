package inboxbatch

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultMaxItems caps a batch when Config.MaxItems is unset. Four related
// items is the most a single cycle's tdd→build→audit pipeline carries without
// tripping the triage capacity clamp's overpacking territory; the policy
// layer may override per project.
const DefaultMaxItems = 4

// Config parameterizes Classify. The zero value is safe (compiled defaults).
type Config struct {
	// MaxItems caps a batch's size; clusters above it chunk in topological
	// order. <=0 means DefaultMaxItems.
	MaxItems int
	// Rules overrides the grouping signals (tests, future policy wiring).
	// nil means defaultRules().
	Rules []Rule
}

// Batch is one coherent unit of work for a single cycle: items ordered so
// dependencies come first, the binding signals that justify the grouping, and
// the max member weight (a batch is as urgent as its most urgent member —
// summing would reward padding).
type Batch struct {
	Items   []Item
	Reasons []string
	Weight  float64
	// DependsOnPrev marks a chunk split off an oversized cluster whose dep
	// chain crosses the split: run the previous batch first.
	DependsOnPrev bool
}

// Classify groups items into batches: rules emit edges → union-find clusters
// → dep-topological order inside each cluster → cap-split into chunks →
// batches ranked by weight (desc), ties by first item id. Pure and
// deterministic: same items, same config, same batches.
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
	reasons := map[int][]string{} // root → binding signals (dedup'd on emit)
	for _, r := range rules {
		for _, e := range r.Edges(items) {
			uf.union(e.A, e.B)
			reasons[uf.find(e.A)] = append(reasons[uf.find(e.A)], e.Reason)
		}
	}

	// Collect clusters; re-root the reason lists (unions after an emit can
	// move a root, so fold every recorded list into the FINAL root).
	clusters := map[int][]int{}
	for i := range items {
		clusters[uf.find(i)] = append(clusters[uf.find(i)], i)
	}
	finalReasons := map[int][]string{}
	for root, rs := range reasons {
		finalReasons[uf.find(root)] = append(finalReasons[uf.find(root)], rs...)
	}

	// Rank clusters as UNITS (go-reviewer HIGH, 2026-07-16): a flat per-chunk
	// weight sort let a continuation outrank its own predecessor whenever a
	// low-weight dep chain gated a high-weight item past the chunk boundary —
	// the render's "run the previous batch first" note then pointed nowhere.
	// Sorting whole clusters by their max member weight keeps chunks adjacent
	// and predecessor-first BY CONSTRUCTION, and ranks the cluster where its
	// most urgent item deserves.
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

// topoOrder returns member indices with every dep before its dependent
// (Kahn), ready-set ordered by weight desc then id for determinism. A dep
// cycle (bad data) falls back to the same weight-then-id order over the
// remainder — items are never dropped, never hang.
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
	if len(out) < len(member) { // dep cycle: append the remainder deterministically
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

// maxRenderedReasons caps the binding signals shown per batch line — a prompt
// section, not a forensic dump (real clusters can carry dozens of signals).
const maxRenderedReasons = 3

// RenderMarkdown formats batches for the triage prompt and the operator CLI:
// one line per batch with rank, weight, binding signals (compacted), and
// member ids — enough for the LLM to select a WHOLE batch as top_n. Empty
// input renders empty (the byte-identical prompt pin).
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

// compactReasons renders at most maxRenderedReasons signals plus a "+N more"
// summary for the rest.
func compactReasons(rs []string) string {
	if len(rs) == 0 {
		return "no shared signal"
	}
	if len(rs) <= maxRenderedReasons {
		return strings.Join(rs, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(rs[:maxRenderedReasons], ", "), len(rs)-maxRenderedReasons)
}

// weightDescThenID is the tie-break rule shared by topoOrder's ready-set and
// Classify's final batch ranking: higher weight first, lower id second — one
// invariant, defined once, so the two orderings can never drift apart.
func weightDescThenID(weightA float64, idA string, weightB float64, idB string) bool {
	if weightA != weightB {
		return weightA > weightB
	}
	return idA < idB
}

// dedupSorted returns the unique reasons, sorted — stable operator-facing
// output regardless of rule emission order.
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

// unionFind is the minimal disjoint-set with path compression — connectivity
// over rule edges is all Classify needs.
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
		u.parent[x] = u.parent[u.parent[x]] // path halving
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
