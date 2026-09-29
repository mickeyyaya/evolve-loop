package acssuite

import (
	"sort"
	"strings"
)

func AncestorCyclePackages(changed []string, cycle int) []string {
	seen := map[string]bool{}
	var out []string
	for _, path := range changed {
		rest, ok := strings.CutPrefix(path, "go/acs/")
		if !ok {
			continue
		}
		name, _, nested := strings.Cut(rest, "/")
		n, canonical := canonicalCyclePackageNumber(name)
		dir := "go/acs/" + name
		if !nested || !canonical || n == cycle || seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}
