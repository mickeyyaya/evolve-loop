package phasecontract

import "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"

type Resolver interface {
	Resolve(name string) (Contract, bool)
}

type BuiltinResolver struct{}

func (BuiltinResolver) Resolve(name string) (Contract, bool) { return For(name) }

type CatalogResolver struct {
	builtin Resolver
	lookup  func(name string) (phasespec.PhaseSpec, bool)
}

func NewCatalogResolver(lookup func(name string) (phasespec.PhaseSpec, bool)) CatalogResolver {
	return CatalogResolver{builtin: BuiltinResolver{}, lookup: lookup}
}

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
