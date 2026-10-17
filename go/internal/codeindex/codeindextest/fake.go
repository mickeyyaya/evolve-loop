// Package codeindextest holds the fake backend and the conformance suite that every codeindex backend must pass.
// See docs/architecture/packages/internal-codeindex.md.
package codeindextest

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

type Fake struct {
	name   string
	caps   codeindex.Capabilities
	index  map[codeindex.Request][]codeindex.Hit
	absent bool
}

func NewFake(name string, caps codeindex.Capabilities, index map[codeindex.Request][]codeindex.Hit) Fake {
	return Fake{name: name, caps: caps, index: index}
}

func NewAbsentFake(name string, caps codeindex.Capabilities) Fake {
	return Fake{name: name, caps: caps, absent: true}
}

func (f Fake) Name() string { return f.name }

func (f Fake) Capabilities() codeindex.Capabilities { return f.caps }

func (f Fake) Callers(_ context.Context, symbol string, dir codeindex.Direction) ([]codeindex.Hit, error) {
	return f.answer(codeindex.Request{Query: codeindex.QueryCallers, Arg: symbol, Direction: dir})
}

func (f Fake) API(_ context.Context, file string) ([]codeindex.Hit, error) {
	return f.answer(codeindex.Request{Query: codeindex.QueryAPI, Arg: file})
}

func (f Fake) Find(_ context.Context, query string) ([]codeindex.Hit, error) {
	return f.answer(codeindex.Request{Query: codeindex.QueryFind, Arg: query})
}

func (f Fake) Map(_ context.Context, dir string) ([]codeindex.Hit, error) {
	return f.answer(codeindex.Request{Query: codeindex.QueryMap, Arg: dir})
}

func (f Fake) answer(req codeindex.Request) ([]codeindex.Hit, error) {
	switch hits := f.index[req]; {
	case !f.caps.Serves(req.Query):
		return nil, fmt.Errorf("%w: %s does not serve %s", codeindex.ErrUnsupported, f.name, req.Query)
	case f.absent:
		return nil, fmt.Errorf("%w: %s", codeindex.ErrAbsent, f.name)
	case len(hits) == 0:
		return nil, fmt.Errorf("%w: %s %q", codeindex.ErrNotFound, req.Query, req.Arg)
	default:
		return hits, nil
	}
}

func (f Fake) Refresh(context.Context) error {
	if f.absent {
		return fmt.Errorf("%w: %s", codeindex.ErrAbsent, f.name)
	}
	return nil
}
