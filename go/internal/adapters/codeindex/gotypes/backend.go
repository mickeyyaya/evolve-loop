// Package gotypes builds a type-checked index of a Go module with the standard library only, and serves callers and api from it as a codeindex.Backend.
// See docs/architecture/packages/internal-adapters-codeindex-gotypes.md.
package gotypes

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

const backendName = "gotypes"

type Backend struct {
	path string
}

func New(indexPath string) Backend {
	return Backend{path: indexPath}
}

func (b Backend) Name() string { return backendName }

func (b Backend) Capabilities() codeindex.Capabilities {
	return codeindex.Capabilities{Callers: true, API: true}
}

func (b Backend) Callers(_ context.Context, symbol string, dir codeindex.Direction) ([]codeindex.Hit, error) {
	data, err := readIndex(b.path)
	if err != nil {
		return nil, err
	}
	defs := data.definitions(symbol)
	if len(defs) == 0 {
		return nil, fmt.Errorf("%w: gotypes has no definition named %q", codeindex.ErrNotFound, symbol)
	}
	var hits []codeindex.Hit
	for _, def := range defs {
		hits = append(hits, data.callersOf(def, dir)...)
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: gotypes found no %s call for %q", codeindex.ErrNotFound, dir, symbol)
	}
	return hits, nil
}

func (b Backend) API(_ context.Context, file string) ([]codeindex.Hit, error) {
	data, err := readIndex(b.path)
	if err != nil {
		return nil, err
	}
	want := filepath.ToSlash(filepath.Clean(file))
	id := slices.Index(data.Files, want)
	if id < 0 || len(data.Decls[id]) == 0 {
		return nil, fmt.Errorf("%w: gotypes has no declaration in %q", codeindex.ErrNotFound, file)
	}
	hits := make([]codeindex.Hit, 0, len(data.Decls[id]))
	for _, d := range data.Decls[id] {
		hits = append(hits, codeindex.Hit{File: want, Line: d.Line, Symbol: d.Name, Text: d.Kind})
	}
	return hits, nil
}

func (b Backend) Find(context.Context, string) ([]codeindex.Hit, error) {
	return nil, unsupported(codeindex.QueryFind)
}

func (b Backend) Map(context.Context, string) ([]codeindex.Hit, error) {
	return nil, unsupported(codeindex.QueryMap)
}

func unsupported(q codeindex.Query) error {
	return fmt.Errorf("%w: gotypes does not serve %s", codeindex.ErrUnsupported, q)
}

func (d indexData) definitions(symbol string) []int {
	var defs []int
	for id, f := range d.Funcs {
		if f.answers(symbol) {
			defs = append(defs, id)
		}
	}
	slices.SortStableFunc(defs, func(a, b int) int {
		return strings.Compare(d.Funcs[a].qualified(), d.Funcs[b].qualified())
	})
	return defs
}

func (d indexData) callersOf(def int, dir codeindex.Direction) []codeindex.Hit {
	if dir == codeindex.DirectionOut {
		return d.callees(def)
	}
	if hits := d.callersIn(def); len(hits) > 0 {
		return hits
	}
	return d.implementsHits(def)
}

func (d indexData) callersIn(def int) []codeindex.Hit {
	name := d.Funcs[def].qualified()
	var hits []codeindex.Hit
	for _, c := range d.Calls {
		if c.Callee == def {
			hits = append(hits, d.hit(c, d.Funcs[c.Caller].qualified(), name))
		}
	}
	return hits
}

func (d indexData) callees(def int) []codeindex.Hit {
	name := d.Funcs[def].qualified()
	var hits []codeindex.Hit
	for _, c := range d.Calls {
		if c.Caller != def {
			continue
		}
		callee := c.Value
		if c.Callee >= 0 {
			callee = d.Funcs[c.Callee].qualified()
		}
		hits = append(hits, d.hit(c, callee, name))
	}
	return hits
}

func (d indexData) hit(c callRecord, symbol, def string) codeindex.Hit {
	return codeindex.Hit{File: d.Files[c.File], Line: c.Line, Symbol: symbol, Text: def + " " + c.Kind.String()}
}

func (d indexData) implementsHits(def int) []codeindex.Hit {
	concrete := d.Funcs[def]
	var hits []codeindex.Hit
	for _, link := range d.Implements {
		if link.Concrete != def {
			continue
		}
		iface := d.Funcs[link.Interface].qualified()
		text := fmt.Sprintf("%s implements %s: %d call(s) through it", concrete.qualified(), iface, d.callsTo(link.Interface))
		hits = append(hits, codeindex.Hit{File: d.Files[concrete.File], Line: concrete.Line, Symbol: iface, Text: text})
	}
	return hits
}

func (d indexData) callsTo(def int) int {
	n := 0
	for _, c := range d.Calls {
		if c.Callee == def {
			n++
		}
	}
	return n
}
