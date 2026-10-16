package inboxbatch

import (
	"path"
	"strings"
)

type Edge struct {
	A, B   int
	Reason string
}

type Rule interface {
	Edges(items []Item) []Edge
}

func DefaultRules() []Rule {
	return []Rule{campaignRule{}, fileAreaRule{}}
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
			edges = append(edges, Edge{A: idx[k-1], B: idx[k], Reason: "campaign " + c})
		}
	}
	return edges
}

type fileAreaRule struct{}

const areaDepth = 3

const hubAreaMaxItems = 5

const minAreaDepth = 2

func (fileAreaRule) Edges(items []Item) []Edge {
	byArea := map[string][]int{}
	for i, it := range items {
		seen := map[string]bool{}
		for _, tok := range it.DeclaredPaths() {
			a := fileArea(tok)
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

func fileArea(f string) string {
	dir := path.Dir(f)
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

type ConnectsRule struct{}

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
		if _, dup := index[it.ID]; !dup {
			index[it.ID] = i
		}
	}
	return index
}

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
