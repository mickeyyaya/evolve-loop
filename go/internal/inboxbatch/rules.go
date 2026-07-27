package inboxbatch

import (
	"path"
	"strings"
)

// Edge relates two items (by slice index) with the human-readable signal that
// bound them — the Reason surfaces into Batch.Reasons so operators and the
// triage agent can see WHY a batch holds together.
type Edge struct {
	A, B   int
	Reason string
}

// Rule is one grouping signal (Strategy). Rules are independent and
// composable: Classify unions the edges of every rule it is given. Adding a
// future signal (e.g. title-token similarity) is a new Rule, not a rewrite.
// Single-method by design (go-reviewer 2026-07-16): each Edge carries its own
// human-readable Reason, so a separate rule-name accessor was vestigial.
type Rule interface {
	// Edges returns every pairing this signal justifies over items.
	Edges(items []Item) []Edge
}

// DefaultRules is the compiled rule set: the STRONG structural signals —
// campaign, package area, and hard dependency edges. connects_to is
// deliberately excluded: it is prose navigation ("link liberally" is the inbox
// house style) and real-backlog validation showed its transitive closure
// chains half the backlog into one mega-cluster; opt in via ConnectsRule when
// a tighter backlog warrants it. Order is presentation-only (edges union).
func DefaultRules() []Rule {
	return []Rule{campaignRule{}, fileAreaRule{}, depRule{}}
}

// campaignRule binds items sharing a non-empty campaign field — the explicit
// "this is one initiative" signal (e.g. "merge-efficiency-2026-07").
type campaignRule struct{}

func (campaignRule) Edges(items []Item) []Edge {
	byCampaign := map[string][]int{}
	for i, it := range items {
		if c := strings.TrimSpace(it.Campaign); c != "" {
			byCampaign[c] = append(byCampaign[c], i)
		}
	}
	var edges []Edge
	for c, idx := range byCampaign {
		for k := 1; k < len(idx); k++ {
			// A spanning chain (0-1, 1-2, …) suffices: union-find only needs
			// connectivity, not the full clique.
			edges = append(edges, Edge{A: idx[k-1], B: idx[k], Reason: "campaign " + c})
		}
	}
	return edges
}

// fileAreaRule binds items whose files touch the same package area — one
// worktree, one build, one audit can carry them together. The area is the
// directory of each file capped at areaDepth path segments (go/internal/router
// and go/internal/router/sub both map to go/internal/router), so sub-package
// splits don't defeat the grouping while top-level dirs (docs/, skills/) stay
// meaningfully separate.
type fileAreaRule struct{}

const areaDepth = 3

// hubAreaMaxItems is the discriminative-signal CEILING: an area referenced by
// more items than this is a HUB (go/internal/core appears in half the real
// backlog) and binds nothing — the inverse-document-frequency idea. Without
// it, real-backlog validation fused 55 of 61 items into one cluster through
// the core hub.
const hubAreaMaxItems = 5

// minAreaDepth is the discriminative FLOOR — the same argument as
// hubAreaMaxItems applied from below. A path whose directory is a single
// top-level segment ("agents", "skills", "go") names a BAG of unrelated files,
// not a unit of work: one worktree, one build and one audit cannot meaningfully
// carry "everything that touches agents/". Such an area binds nothing.
//
// Without this floor, every persona file in the repo collapsed to the area
// "agents" and every top-level skill file to "skills". Measured on the 84-item
// backlog (2026-07-27), two "agents" edges chained three unrelated campaigns
// (chronicle-2026-07 <-> pipeline-integrity <-> convergence-2026-07) into a
// single 43-item cluster — 51% of the backlog, chunked into 11 batches each
// marked "run the previous batch first". These shallow areas slip UNDER
// hubAreaMaxItems, so the ceiling alone never caught them.
const minAreaDepth = 2

func (fileAreaRule) Edges(items []Item) []Edge {
	byArea := map[string][]int{}
	for i, it := range items {
		seen := map[string]bool{}
		for _, f := range it.Files {
			a := fileArea(f)
			if a == "" || seen[a] {
				continue
			}
			seen[a] = true
			byArea[a] = append(byArea[a], i)
		}
	}
	var edges []Edge
	for a, idx := range byArea {
		if len(idx) > hubAreaMaxItems {
			continue // hub area: no discriminative signal
		}
		for k := 1; k < len(idx); k++ {
			edges = append(edges, Edge{A: idx[k-1], B: idx[k], Reason: "file-area " + a})
		}
	}
	return edges
}

// fileArea reduces a file path to its grouping area: the containing directory,
// capped at areaDepth segments and required to be at least minAreaDepth deep.
// A bare filename (no directory) and a bare top-level directory both have no
// area — see minAreaDepth for why shallow areas must not bind.
func fileArea(f string) string {
	f = strings.TrimSpace(f)
	dir := path.Dir(f)
	// An item may name a directory ("go/internal/acssuite/"); path.Dir would
	// climb out of it, so keep the directory itself as the area.
	if strings.HasSuffix(f, "/") {
		dir = strings.TrimSuffix(f, "/")
	}
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	seg := strings.Split(dir, "/")
	if len(seg) < minAreaDepth {
		return "" // top-level bag, not a unit of work
	}
	if len(seg) > areaDepth {
		seg = seg[:areaDepth]
	}
	return strings.Join(seg, "/")
}

// depRule binds items through hard dependency edges (deps[]) — the same edges
// the topological sort consumes for in-batch ordering, so a dep and its
// dependent land in one batch whenever the cap allows.
type depRule struct{}

func (depRule) Edges(items []Item) []Edge {
	index := indexByID(items)
	var edges []Edge
	for i, it := range items {
		for _, d := range it.Deps {
			if j, ok := resolveRef(d, index); ok && j != i {
				edges = append(edges, Edge{A: j, B: i, Reason: "dep " + items[j].ID + "→" + it.ID})
			}
		}
	}
	return edges
}

// ConnectsRule binds items through connects_to references — prose by
// convention ("other-id (why it relates)"), resolved when an entry begins
// with an existing item's id; unresolvable prose is ignored rather than
// guessed at. NOT in DefaultRules (see there); exported for Config.Rules
// opt-in on backlogs whose links are sparse enough not to blob.
type ConnectsRule struct{}

// Edges returns one edge per resolvable connects_to reference.
func (ConnectsRule) Edges(items []Item) []Edge {
	index := indexByID(items)
	var edges []Edge
	for i, it := range items {
		for _, c := range it.ConnectsTo {
			if j, ok := resolveRef(c, index); ok && j != i {
				edges = append(edges, Edge{A: i, B: j, Reason: "connects " + it.ID + "↔" + items[j].ID})
			}
		}
	}
	return edges
}

func indexByID(items []Item) map[string]int {
	index := make(map[string]int, len(items))
	for i, it := range items {
		index[it.ID] = i
	}
	return index
}

// resolveRef matches a deps/connects_to entry against known item ids: exact
// match first, then the prose convention "id (explanation…)" — the id is the
// first whitespace-delimited token.
func resolveRef(ref string, index map[string]int) (int, bool) {
	ref = strings.TrimSpace(ref)
	if j, ok := index[ref]; ok {
		return j, true
	}
	if tok := strings.Fields(ref); len(tok) > 0 {
		if j, ok := index[tok[0]]; ok {
			return j, true
		}
	}
	return 0, false
}
