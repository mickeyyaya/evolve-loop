package scout

import (
	"fmt"
	"sort"
	"strings"
)

// partition.go — scout map-reduce MAP-step slicing (pure). The codebase is
// partitioned into N independent ScanSlices; one scout-scan worker scans each
// slice in parallel. The "core" hub (fan-in=33) is excluded — it is scanned by
// the reduce synthesizer only, so a single edit lands per hub file.

// ScanSlice is one MAP unit: a contiguous group of package import paths a single
// scout-scan worker scans and digests.
type ScanSlice struct {
	ID       string   `json:"id"`
	Packages []string `json:"packages"`
}

// partitionPackages groups packages into at most n contiguous ScanSlices for
// parallel scanning, excluding the core hub. Deterministic: packages are sorted,
// then chunked so related (lexically adjacent) packages stay together — which
// keeps cross-package findings within a single worker. n<1 ⇒ 1; n>len ⇒ len.
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
	per := (len(pkgs) + n - 1) / n // ceil
	slices := make([]ScanSlice, n)
	for i := range slices {
		slices[i].ID = fmt.Sprintf("slice-%d", i+1)
	}
	for i, p := range pkgs {
		idx := i / per
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

// isCoreHub reports whether a package is the high-fan-in core hub, which the
// reduce synthesizer scans (never a MAP worker), so hub edits stay single-writer.
func isCoreHub(pkg string) bool {
	return strings.HasSuffix(pkg, "/internal/core") || strings.Contains(pkg, "/internal/core/")
}
