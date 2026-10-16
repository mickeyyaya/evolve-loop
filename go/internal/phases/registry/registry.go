// Package registry implements the phase Factory Method.
package registry

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type Factory func(req core.PhaseRequest) core.PhaseRunner

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

func Register(name string, f Factory) {
	if name == "" {
		panic("phases/registry: Register requires a non-empty name")
	}
	if f == nil {
		panic(fmt.Sprintf("phases/registry: Register(%q) requires a non-nil Factory", name))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("phases/registry: duplicate Register(%q) — each phase registers exactly once", name))
	}
	factories[name] = f
}

func For(name string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := factories[name]
	return f, ok
}

func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(factories))
	for n := range factories {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func ResetForTesting() {
	mu.Lock()
	defer mu.Unlock()
	factories = map[string]Factory{}
}

func SnapshotForTest() func() {
	mu.Lock()
	snap := make(map[string]Factory, len(factories))
	for n, f := range factories {
		snap[n] = f
	}
	mu.Unlock()
	return func() {
		mu.Lock()
		defer mu.Unlock()
		factories = make(map[string]Factory, len(snap))
		for n, f := range snap {
			factories[n] = f
		}
	}
}
