package phasecontract

import "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"

// Resolver resolves a deliverable Contract for a phase or agent name.
type Resolver interface {
	Resolve(name string) (Contract, bool)
}

// BuiltinResolver resolves only the built-in contracts, exactly as For does.
type BuiltinResolver struct{}

// Resolve delegates to For.
func (BuiltinResolver) Resolve(name string) (Contract, bool) { return For(name) }

// CatalogResolver serves built-in contracts first, then derives other phases via FromSpec; a nil lookup serves built-ins only.
type CatalogResolver struct {
	builtin Resolver
	lookup  func(name string) (phasespec.PhaseSpec, bool)
}

// NewCatalogResolver builds a CatalogResolver over the given spec lookup, typically Catalog.Get.
func NewCatalogResolver(lookup func(name string) (phasespec.PhaseSpec, bool)) CatalogResolver {
	return CatalogResolver{builtin: BuiltinResolver{}, lookup: lookup}
}

// Resolve returns the built-in contract overlaid with its registry declaration, else a spec-derived contract.
func (r CatalogResolver) Resolve(name string) (Contract, bool) {
	if c, ok := r.builtin.Resolve(name); ok {
		if r.lookup != nil {
			if spec, found := r.lookup(RegistryKey(name)); found {
				c = overlayDeclared(c, spec)
			}
		}
		return c, true
	}
	if r.lookup == nil {
		return Contract{}, false
	}
	if spec, ok := r.lookup(name); ok && SynthesizesContract(spec) {
		return FromSpec(spec), true
	}
	return Contract{}, false
}
