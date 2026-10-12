package testkey

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
)

func closure(in Input) ([]overlap.Package, error) {
	byPath := make(map[string]overlap.Package, len(in.Module.Packages))
	for _, p := range in.Module.Packages {
		byPath[p.ImportPath] = p
	}
	wanted := map[string]bool{}
	for _, root := range append([]string{in.Package}, in.TestImports...) {
		p, ok := byPath[root]
		if !ok {
			return nil, fmt.Errorf("testkey: package %s is not in the module map", root)
		}
		wanted[root] = true
		for _, d := range p.Deps {
			wanted[d] = true
		}
	}
	out := make([]overlap.Package, 0, len(wanted))
	for path := range wanted {
		p, ok := byPath[path]
		if !ok {
			return nil, fmt.Errorf("testkey: dependency %s is not in the module map", path)
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b overlap.Package) int { return cmp.Compare(a.ImportPath, b.ImportPath) })
	return out, nil
}
