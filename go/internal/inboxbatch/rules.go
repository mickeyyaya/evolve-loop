package inboxbatch

import (
	"path"
	"strings"
)

// Edge binds items A and B (slice indices); Reason renders into Batch.Reasons.
type Edge struct {
	A, B   int
	Reason string
}

// Rule is one grouping signal; Classify unions the edges of every rule it is given.
type Rule interface {
	// Edges returns every pairing this signal justifies over items.
	Edges(items []Item) []Edge
}

// DefaultRules returns the bounded structural rules: campaign, file area and dep. ConnectsRule is opt-in.
// root_cause is never a rule: its values are unique per-defect prose, so exact matching yields zero edges
// (.evolve/state.json failedApproaches[54]).
func DefaultRules() []Rule {
	return []Rule{campaignRule{}, fileAreaRule{}, depRule{}}
}

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
			// A spanning chain suffices: union-find needs connectivity, not the clique.
			edges = append(edges, Edge{A: idx[k-1], B: idx[k], Reason: "campaign " + c})
		}
	}
	return edges
}

// fileAreaRule binds items whose files share a package area (see fileArea).
type fileAreaRule struct{}

const areaDepth = 3

// hubAreaMaxItems is the ceiling: an area more items share than this is a hub and carries no signal.
const hubAreaMaxItems = 5

// minAreaDepth is the floor: a single top-level directory is a bag of unrelated files, not a unit of work.
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
			continue
		}
		for k := 1; k < len(idx); k++ {
			edges = append(edges, Edge{A: idx[k-1], B: idx[k], Reason: "file-area " + a})
		}
	}
	return edges
}

// fileArea returns f's directory capped at areaDepth segments, or "" when it is shallower than minAreaDepth.
func fileArea(f string) string {
	f = strings.TrimSpace(f)
	dir := path.Dir(f)
	// A trailing "/" names a directory, which path.Dir would climb out of.
	if strings.HasSuffix(f, "/") {
		dir = strings.TrimSuffix(f, "/")
	}
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	seg := strings.Split(dir, "/")
	if len(seg) < minAreaDepth {
		return ""
	}
	if len(seg) > areaDepth {
		seg = seg[:areaDepth]
	}
	return strings.Join(seg, "/")
}

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

// ConnectsRule binds items through connects_to entries that begin with a known id; it is opt-in via Config.Rules.
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

// resolveRef matches an exact id first, then the first token of the prose form "id (why)".
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
