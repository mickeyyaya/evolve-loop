package overlap

import (
	"path"
	"strings"
)

type moduleIndex struct {
	modulePath string
	fileOwners map[string][]string
	dirPackage map[string]string
	deps       map[string][]string
	deleted    map[string]bool
}

func newModuleIndex(m Module, deleted []string) moduleIndex {
	idx := moduleIndex{
		modulePath: m.Path,
		fileOwners: map[string][]string{},
		dirPackage: map[string]string{},
		deps:       map[string][]string{},
		deleted:    map[string]bool{},
	}
	for _, p := range m.Packages {
		idx.dirPackage[p.Dir] = p.ImportPath
		idx.deps[p.ImportPath] = p.Deps
		for _, f := range p.Files {
			idx.fileOwners[f] = append(idx.fileOwners[f], p.ImportPath)
		}
	}
	for _, d := range deleted {
		idx.deleted[d] = true
	}
	return idx
}

func (idx moduleIndex) owners(p string) []string {
	set := map[string]bool{}
	for _, o := range idx.fileOwners[p] {
		set[o] = true
	}
	for _, o := range idx.testdataOwners(p) {
		set[o] = true
	}
	if o, ok := idx.deletedOwner(p); ok {
		set[o] = true
	}
	return sortedKeys(set)
}

func (idx moduleIndex) testdataOwners(p string) []string {
	parts := strings.Split(p, "/")
	var out []string
	for i, part := range parts {
		if part != "testdata" {
			continue
		}
		if o, ok := idx.dirPackage[strings.Join(parts[:i], "/")]; ok {
			out = append(out, o)
		}
	}
	return out
}

func (idx moduleIndex) deletedOwner(p string) (string, bool) {
	if !idx.deleted[p] || path.Ext(p) != ".go" {
		return "", false
	}
	if idx.modulePath == "" {
		return "", false
	}
	dir := path.Dir(p)
	if dir == moduleRoot {
		return idx.modulePath, true
	}
	return idx.modulePath + "/" + strings.TrimPrefix(dir, moduleRoot+"/"), true
}

func (idx moduleIndex) closure(pkg string) []string {
	return append([]string{pkg}, idx.deps[pkg]...)
}
