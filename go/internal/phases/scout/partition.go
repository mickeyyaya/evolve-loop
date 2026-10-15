package scout

import (
	"fmt"
	"sort"
	"strings"
)

type ScanSlice struct {
	ID       string   `json:"id"`
	Packages []string `json:"packages"`
}

var sliceIndex = func(i, per int) int { return i / per }

func partitionPackages(packages []string, n int) []ScanSlice {
	if n < 1 {
		n = 1
	}
	pkgs := make([]string, 0, len(packages))
	for _, p := range packages {
		if isCoreHub(p) {
			continue
		}
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	if len(pkgs) == 0 {
		return nil
	}
	if n > len(pkgs) {
		n = len(pkgs)
	}
	per := (len(pkgs) + n - 1) / n
	slices := make([]ScanSlice, n)
	for i := range slices {
		slices[i].ID = fmt.Sprintf("slice-%d", i+1)
	}
	for i, p := range pkgs {
		idx := sliceIndex(i, per)
		if idx >= n {
			idx = n - 1
		}
		slices[idx].Packages = append(slices[idx].Packages, p)
	}
	out := make([]ScanSlice, 0, n)
	for _, s := range slices {
		if len(s.Packages) > 0 {
			out = append(out, s)
		}
	}
	return out
}

func isCoreHub(pkg string) bool {
	return strings.HasSuffix(pkg, "/internal/core") || strings.Contains(pkg, "/internal/core/")
}
