package codeindex

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
)

func SampleSymbols(pool []BenchSymbol, n int, probes []string) []BenchSymbol {
	var probed, rest []BenchSymbol
	for _, sym := range pool {
		if isProbe(sym.Name, probes) {
			probed = append(probed, sym)
		} else {
			rest = append(rest, sym)
		}
	}
	slices.SortFunc(rest, compareByPositionHash)
	picked := rest[:min(max(n, 0), len(rest))]
	return append(slices.Clone(picked), probed...)
}

func isProbe(name string, probes []string) bool {
	return slices.ContainsFunc(probes, func(p string) bool { return name == p || strings.HasSuffix(name, "."+p) })
}

func compareByPositionHash(a, b BenchSymbol) int {
	return cmp.Or(
		cmp.Compare(positionHash(a), positionHash(b)),
		cmp.Compare(a.DefFile, b.DefFile),
		cmp.Compare(a.DefLine, b.DefLine),
		cmp.Compare(a.Name, b.Name),
	)
}

func positionHash(s BenchSymbol) uint64 {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", s.DefFile, s.DefLine)))
	return binary.BigEndian.Uint64(sum[:8])
}
