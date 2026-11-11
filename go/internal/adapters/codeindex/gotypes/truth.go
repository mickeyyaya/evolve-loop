package gotypes

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

func Truth(indexPath string) ([]codeindex.BenchSymbol, error) {
	data, err := readIndex(indexPath)
	if err != nil {
		return nil, err
	}
	return data.truth(), nil
}

func (d indexData) truth() []codeindex.BenchSymbol {
	sites := make(map[int][]codeindex.TruthSite, len(d.Funcs))
	nonStatic := map[int]bool{}
	for _, c := range d.Calls {
		if c.Callee < 0 {
			continue
		}
		sites[c.Callee] = append(sites[c.Callee], codeindex.TruthSite{File: d.Files[c.File], Line: c.Line, Func: d.Funcs[c.Caller].qualified()})
		nonStatic[c.Callee] = nonStatic[c.Callee] || c.Kind != kindStatic
	}
	names := map[string]int{}
	for _, f := range d.Funcs {
		names[f.Name]++
	}
	moduleFiles := d.moduleFiles()
	var out []codeindex.BenchSymbol
	for id, f := range d.Funcs {
		if !f.Module || len(sites[id]) == 0 {
			continue
		}
		sym := codeindex.BenchSymbol{Name: f.qualified(), DefFile: d.Files[f.File], DefLine: f.Line, Sites: sites[id]}
		if names[f.Name] == 1 && !nonStatic[id] && !f.Interface && !inTestFile(sites[id]) {
			sym.GrepFiles = moduleFiles
		}
		out = append(out, sym)
	}
	return out
}

func (d indexData) moduleFiles() []string {
	seen := map[int]bool{}
	var files []string
	for _, f := range d.Funcs {
		if f.Module && !seen[f.File] && !strings.HasSuffix(d.Files[f.File], testSuffix) {
			seen[f.File] = true
			files = append(files, d.Files[f.File])
		}
	}
	return files
}

const testSuffix = "_test.go"

func inTestFile(sites []codeindex.TruthSite) bool {
	return slices.ContainsFunc(sites, func(s codeindex.TruthSite) bool { return strings.HasSuffix(s.File, testSuffix) })
}
