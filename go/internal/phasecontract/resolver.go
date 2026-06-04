package phasecontract

import "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"

// Resolver resolves a deliverable Contract for a phase/agent name. It is the
// seam that lets the deliverable verifier and the bridge prompt-injection serve
// BOTH the hardcoded built-in contracts AND spec-derived contracts for
// user/minted phases, without a package-global that knows about the catalog.
//
// The package default is BuiltinResolver (built-in map only, byte-identical to
// the legacy For path). A spec-aware NewCatalogResolver falls back to FromSpec
// for any name the built-in map misses.
type Resolver interface {
	Resolve(name string) (Contract, bool)
}

// BuiltinResolver resolves only the hardcoded built-in contracts (with alias
// resolution). It is the back-compat default and preserves the exact behavior
// of the package-level For.
type BuiltinResolver struct{}

// Resolve delegates to For — built-in map plus human-facing aliases.
func (BuiltinResolver) Resolve(name string) (Contract, bool) { return For(name) }

// CatalogResolver serves built-in contracts first (so a spine phase's contract
// is authoritative and a clashing user spec can never weaken it), then falls
// back to deriving a Contract from a PhaseSpec via FromSpec for user/minted
// phases. lookup is typically Catalog.Get; a nil lookup degrades to built-in
// only (no panic).
type CatalogResolver struct {
	builtin Resolver
	lookup  func(name string) (phasespec.PhaseSpec, bool)
}

// NewCatalogResolver builds a CatalogResolver over the given spec lookup.
func NewCatalogResolver(lookup func(name string) (phasespec.PhaseSpec, bool)) CatalogResolver {
	return CatalogResolver{builtin: BuiltinResolver{}, lookup: lookup}
}

// Resolve returns the built-in contract if one exists for name (override), else
// the spec-derived contract when the lookup knows the phase, else a miss.
func (r CatalogResolver) Resolve(name string) (Contract, bool) {
	if c, ok := r.builtin.Resolve(name); ok {
		return c, true
	}
	if r.lookup == nil {
		return Contract{}, false
	}
	if spec, ok := r.lookup(name); ok {
		return FromSpec(spec), true
	}
	return Contract{}, false
}
