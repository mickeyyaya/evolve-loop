package codeindex

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Backends struct {
	ordered             []Backend
	fallThroughOnAbsent bool
}

type Option func(*Backends)

func WithAbsentFallThrough() Option {
	return func(b *Backends) {
		b.fallThroughOnAbsent = true
	}
}

func NewBackends(ordered ...Backend) Backends {
	return Backends{ordered: slices.Clone(ordered)}
}

func (b Backends) Names() []string {
	names := make([]string, len(b.ordered))
	for i, backend := range b.ordered {
		names[i] = backend.Name()
	}
	return names
}

func (b Backends) For(q Query) (Backend, error) {
	for _, backend := range b.ordered {
		if backend.Capabilities().Serves(q) {
			return backend, nil
		}
	}
	return nil, unsupported(q)
}

func (b Backends) Search(ctx context.Context, req Request) ([]Hit, error) {
	hits, _, err := b.SearchServed(ctx, req)
	return hits, err
}

func (b Backends) SearchServed(ctx context.Context, req Request) (hits []Hit, served string, err error) {
	var firstAbsentName string
	var firstAbsentErr error
	for _, backend := range b.ordered {
		if !backend.Capabilities().Serves(req.Query) {
			continue
		}
		hits, err = Dispatch(ctx, backend, req)
		if b.fallThroughOnAbsent && errors.Is(err, ErrAbsent) {
			if firstAbsentErr == nil {
				firstAbsentName = backend.Name()
				firstAbsentErr = err
			}
			continue
		}
		return hits, backend.Name(), err
	}
	if firstAbsentErr != nil {
		return nil, firstAbsentName, firstAbsentErr
	}
	return nil, "", unsupported(req.Query)
}

func unsupported(q Query) error {
	return fmt.Errorf("%w: no backend serves %s", ErrUnsupported, q)
}

type Factory func() (Backend, error)

func New(names []string, factories map[string]Factory, opts ...Option) (Backends, error) {
	built, err := Resolve(names, factories)
	if err != nil {
		return Backends{}, err
	}
	b := NewBackends(built...)
	for _, opt := range opts {
		opt(&b)
	}
	return b, nil
}

func Resolve(names []string, factories map[string]Factory) ([]Backend, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("codeindex: no backend named")
	}
	built := make([]Backend, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return nil, fmt.Errorf("codeindex: duplicate backend %q", name)
		}
		seen[name] = true
		backend, err := build(name, factories)
		if err != nil {
			return nil, err
		}
		built = append(built, backend)
	}
	return built, nil
}

func KnownBackends(factories map[string]Factory) []string {
	return slices.Sorted(maps.Keys(factories))
}

func build(name string, factories map[string]Factory) (Backend, error) {
	factory, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("codeindex: unknown backend %q (known: %s)", name, knownNames(factories))
	}
	backend, err := factory()
	if err != nil {
		return nil, fmt.Errorf("codeindex: backend %q: %w", name, err)
	}
	return backend, nil
}

func knownNames(factories map[string]Factory) string {
	return strings.Join(KnownBackends(factories), ", ")
}

func (b Backends) Refresh(ctx context.Context) ([]string, error) {
	var refreshed []string
	var failures []error
	for _, backend := range b.ordered {
		refresher, ok := backend.(Refresher)
		if !ok {
			continue
		}
		if err := refresher.Refresh(ctx); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", backend.Name(), err))
			continue
		}
		refreshed = append(refreshed, backend.Name())
	}
	return refreshed, errors.Join(failures...)
}
