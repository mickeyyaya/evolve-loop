package testkey

import (
	"path"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
)

const (
	goModPath  = "go/go.mod"
	goSumPath  = "go/go.sum"
	testdata   = "testdata"
	repoMarker = "."
	opKindOpen = "open"
)

type scope struct {
	exact     map[string]bool
	trees     []string
	ancestors map[string]bool
}

func newScope(in Input, pkgs []overlap.Package) scope {
	s := scope{exact: map[string]bool{goModPath: true, goSumPath: true}, ancestors: map[string]bool{}}
	for _, p := range pkgs {
		for _, f := range p.Files {
			s.exact[f] = true
		}
		s.trees = append(s.trees, path.Join(p.Dir, testdata))
		s.trees = append(s.trees, in.DataRoots[p.ImportPath]...)
	}
	for covered := range s.exact {
		s.addAncestors(covered)
	}
	for _, t := range s.trees {
		s.addAncestors(t)
	}
	return s
}

func (s scope) addAncestors(p string) {
	for dir := path.Dir(p); ; dir = path.Dir(dir) {
		s.ancestors[dir] = true
		if dir == repoMarker {
			return
		}
	}
}

func (s scope) covers(rel, kind string) bool {
	if s.exact[rel] || (s.ancestors[rel] && kind != opKindOpen) {
		return true
	}
	return slices.ContainsFunc(s.trees, func(t string) bool { return rel == t || strings.HasPrefix(rel, t+"/") })
}
