package acssuite

import (
	"regexp"
	"sort"
	"strings"
)

var cyclePackageFileRe = regexp.MustCompile(`^go/(acs/cycle\d+)/`)

func AncestorCyclePackages(added []string, cycle int) []string {
	own := strings.TrimPrefix(CyclePackage(cycle), "./")
	seen := map[string]bool{}
	var out []string
	for _, path := range added {
		m := cyclePackageFileRe.FindStringSubmatch(path)
		if m == nil || m[1] == own || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, "go/"+m[1])
	}
	sort.Strings(out)
	return out
}
