// Package registry implements the phase Factory Method. Each
// internal/phases/<name>/ package self-registers in init(); the
// dispatcher (cmd_phase.go, cmd_compose.go) looks up factories by name
// without editing a switch statement.
//
// Pattern: Factory Method (GoF). The "product" is core.PhaseRunner;
// the "factory" is a closure that knows how to construct a runner
// given the per-request envelope (core.PhaseRequest).
//
// Goals:
//
//   - OCP: adding a new phase = new package + 1 init() line, no edit to dispatch
//   - DRY: every phase's "how do I build the runner" lives next to the runner
//   - Testability: ResetForTesting() lets tests construct controlled registry states
//
// Thread safety: registration happens at init() time (single-threaded
// program startup) but lookups can happen concurrently. Both are
// guarded by an RWMutex; lookups take the read lock.
package registry

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Factory builds a core.PhaseRunner from a phase request. The closure
// captures whatever deps it needs (bridge, prompts, etc.); the
// registry only knows about the PhaseRunner contract.
type Factory func(req core.PhaseRequest) core.PhaseRunner

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register publishes a phase under name. Panics on duplicate name so
// init-time conflicts surface at startup rather than turning into a
// hard-to-debug runtime mystery.
//
// Callers: in each internal/phases/<name>/ package,
//
//	func init() {
//	    registry.Register("build", func(req core.PhaseRequest) core.PhaseRunner {
//	        return New(Config{...})
//	    })
//	}
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

// For returns the factory for name. (factory, true) on hit; (nil,
// false) on miss — callers typically return exit code 10 + usage on
// miss.
func For(name string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := factories[name]
	return f, ok
}

// Names returns a sorted snapshot of registered phase names. Used by
// dispatcher usage output and by the docs-contract test.
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

// ResetForTesting clears all registered factories. ONLY for use in
// tests that need a controlled registry state — production code MUST
// NOT call this. Exported with the explicit suffix so its purpose is
// unmistakable.
func ResetForTesting() {
	mu.Lock()
	defer mu.Unlock()
	factories = map[string]Factory{}
}

// SnapshotForTest captures the currently-registered factories and returns a
// restore func that re-establishes exactly that set (discarding any
// registrations made in between). ONLY for tests that mutate the registry and
// must restore it afterward; pair with ResetForTesting + Register in the test
// body. Production code MUST NOT call this. Single home for the snapshot/restore
// idiom that the cmd test suites previously each re-implemented.
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
