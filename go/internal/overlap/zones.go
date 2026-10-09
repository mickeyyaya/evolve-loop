package overlap

import (
	"path"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

type zone int

const (
	zoneBookkeeping zone = iota
	zoneBuild
	zoneGate
	zoneDerived
	zoneModule
	zoneProse
	zoneUnknown
)

const moduleRoot = "go"

var readRootFiles = []string{"docs/conventions/ste100-writing.md"}

var readRootDirs = []string{"docs/research/", "docs/reference/"}

var proseRoots = []string{"docs/", "solutions/"}

type sideView struct {
	paths  []string
	zones  map[string]zone
	pkgs   map[string]bool
	owners map[string][]string
}

func (s sideView) onlyBookkeeping() bool {
	for _, p := range s.paths {
		if s.zones[p] != zoneBookkeeping {
			return false
		}
	}
	return true
}

func (s sideView) pathsIn(z zone) []string {
	var out []string
	for _, p := range s.paths {
		if s.zones[p] == z {
			out = append(out, p)
		}
	}
	return out
}

func classifySide(cat Catalogs, idx moduleIndex, side Side, paths []string) sideView {
	view := sideView{paths: sortedUnique(paths), zones: map[string]zone{}, pkgs: map[string]bool{}, owners: map[string][]string{}}
	for _, p := range view.paths {
		z, owners := zoneOf(cat, idx, side, p)
		view.zones[p] = z
		view.owners[p] = owners
		for _, o := range owners {
			view.pkgs[o] = true
		}
	}
	return view
}

func zoneOf(cat Catalogs, idx moduleIndex, side Side, p string) (zone, []string) {
	switch {
	case !explanationdocs.IsPlainPath(p):
		return zoneUnknown, nil
	case cat.Bookkeeping(p):
		return zoneBookkeeping, nil
	case cat.BuildZone(p):
		return zoneBuild, nil
	case cat.GateZone(p):
		return zoneGate, nil
	case cat.DerivedOutput(side, p):
		return zoneDerived, nil
	case strings.HasPrefix(p, moduleRoot+"/"):
		return moduleZone(idx.owners(p))
	case isProse(p):
		return zoneProse, nil
	}
	return zoneUnknown, nil
}

func moduleZone(owners []string) (zone, []string) {
	if len(owners) == 0 {
		return zoneUnknown, nil
	}
	return zoneModule, owners
}

func isProse(p string) bool {
	if path.Ext(p) != ".md" || hasAnyPrefix(p, readRootDirs) {
		return false
	}
	for _, f := range readRootFiles {
		if p == f {
			return false
		}
	}
	return hasAnyPrefix(p, proseRoots)
}

func hasAnyPrefix(p string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

func sortedUnique(xs []string) []string {
	set := map[string]bool{}
	for _, x := range xs {
		set[x] = true
	}
	return sortedKeys(set)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
