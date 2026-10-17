package codeindex

import (
	"context"
	"errors"
)

var (
	ErrNotFound    = errors.New("no result")
	ErrAbsent      = errors.New("the code index is absent")
	ErrUnsupported = errors.New("query not supported")
)

type Capabilities struct {
	Callers, API, Find, Map bool
}

func AllCapabilities() Capabilities {
	return Capabilities{Callers: true, API: true, Find: true, Map: true}
}

func (c Capabilities) Serves(q Query) bool {
	switch q {
	case QueryCallers:
		return c.Callers
	case QueryAPI:
		return c.API
	case QueryFind:
		return c.Find
	case QueryMap:
		return c.Map
	}
	return false
}

type Backend interface {
	Name() string
	Capabilities() Capabilities
	Callers(ctx context.Context, symbol string, dir Direction) ([]Hit, error)
	API(ctx context.Context, file string) ([]Hit, error)
	Find(ctx context.Context, query string) ([]Hit, error)
	Map(ctx context.Context, dir string) ([]Hit, error)
}

func Dispatch(ctx context.Context, b Backend, req Request) ([]Hit, error) {
	switch req.Query {
	case QueryCallers:
		return b.Callers(ctx, req.Arg, req.Direction)
	case QueryAPI:
		return b.API(ctx, req.Arg)
	case QueryFind:
		return b.Find(ctx, req.Arg)
	case QueryMap:
		return b.Map(ctx, req.Arg)
	}
	return nil, unsupported(req.Query)
}

type Refresher interface {
	Refresh(ctx context.Context) error
}
