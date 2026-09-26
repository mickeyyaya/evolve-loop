package phasecontract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestProperty_RenderBlockListsEveryEnforcedRequirement(t *testing.T) {
	for _, c := range Contracts() {
		block := RenderContractBlock(c)
		switch c.Kind {
		case KindJSON:
			for _, k := range c.RequiredKeys {
				if !strings.Contains(block, `"`+k+`"`) {
					t.Errorf("%s: enforced JSON key %q absent from the rendered contract block — agents are gated on a requirement they were never shown", c.Phase, k)
				}
			}
		default:
			for _, s := range c.Sections {
				if !strings.Contains(block, `"`+s.Canonical+`"`) {
					t.Errorf("%s: enforced section %q absent from the rendered contract block — agents are gated on a requirement they were never shown", c.Phase, s.Canonical)
				}
			}
			for _, v := range c.Verdicts {
				if !strings.Contains(block, v) {
					t.Errorf("%s: enforced verdict token %q absent from the rendered contract block", c.Phase, v)
				}
			}
		}
		if !strings.Contains(block, "evolve phase verify "+c.Phase) {
			t.Errorf("%s: rendered block lacks the self-check instruction", c.Phase)
		}
	}
}

func TestProperty_BuiltinAndCatalogResolversAgreeOnBuiltins(t *testing.T) {
	missLookup := func(string) (phasespec.PhaseSpec, bool) { return phasespec.PhaseSpec{}, false }
	catalog := NewCatalogResolver(missLookup)
	exercised := 0
	for _, c := range Contracts() {
		b, okB := BuiltinResolver{}.Resolve(c.Phase)
		if !okB {
			// An entry keyed by an alias resolves only by its key; resolver_test.go pins aliases.
			continue
		}
		exercised++
		k, okK := catalog.Resolve(c.Phase)
		if !okK {
			t.Errorf("%s: BuiltinResolver resolves but CatalogResolver misses — two policies", c.Phase)
			continue
		}
		if !reflect.DeepEqual(b, k) {
			t.Errorf("%s: resolvers disagree on the contract\nbuiltin: %+v\ncatalog: %+v", c.Phase, b, k)
		}
	}
	if exercised == 0 {
		t.Fatal("property loop exercised no contracts — a silently-empty sweep proves nothing")
	}
}
