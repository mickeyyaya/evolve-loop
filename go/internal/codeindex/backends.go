package codeindex

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type Backends struct {
	ordered []Backend
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
	backend, err := b.For(req.Query)
	if err != nil {
		return nil, err
	}
	return Dispatch(ctx, backend, req)
}

func unsupported(q Query) error {
	return fmt.Errorf("%w: no backend serves %s", ErrUnsupported, q)
}

type Factory func() (Backend, error)

func New(names []string, factories map[string]Factory) (Backends, error) {
	if len(names) == 0 {
		return Backends{}, fmt.Errorf("codeindex: no backend named")
	}
	built := make([]Backend, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return Backends{}, fmt.Errorf("codeindex: duplicate backend %q", name)
		}
		seen[name] = true
		backend, err := build(name, factories)
		if err != nil {
			return Backends{}, err
		}
		built = append(built, backend)
	}
	return NewBackends(built...), nil
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
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
