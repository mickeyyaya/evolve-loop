package phasecontract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// properties_test.go — cross-leg invariants swept over EVERY built-in
// contract (test-plan P0 #1/#2, 2026-06-12 contract-pipeline review).
// Property loops fail automatically when a new phase violates the
// invariant; hand-picked examples don't.

// TestProperty_RenderBlockListsEveryEnforcedRequirement — the prompt
// projection (leg A, RenderContractBlock) and the enforcement (leg C,
// VerifyWith via Contract.Sections/RequiredKeys) must never drift: every
// section heading / JSON key the gate enforces must be named in the block
// the agent is shown.
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
		// Every contract-bearing phase is told to self-check (ADR-0034).
		if !strings.Contains(block, "evolve phase verify "+c.Phase) {
			t.Errorf("%s: rendered block lacks the self-check instruction", c.Phase)
		}
	}
}

// TestProperty_BuiltinAndCatalogResolversAgreeOnBuiltins — the two resolver
// strategies must return byte-identical contracts for every built-in phase:
// a consumer wired with BuiltinResolver and one wired with a CatalogResolver
// (regardless of user-spec lookup) may never disagree about a spine phase's
// requirements.
func TestProperty_BuiltinAndCatalogResolversAgreeOnBuiltins(t *testing.T) {
	missLookup := func(string) (phasespec.PhaseSpec, bool) { return phasespec.PhaseSpec{}, false }
	catalog := NewCatalogResolver(missLookup)
	exercised := 0
	for _, c := range Contracts() {
		b, okB := BuiltinResolver{}.Resolve(c.Phase)
		if !okB {
			// Registry entries keyed by a wire alias (key != Phase) resolve
			// by key; the alias path is pinned in resolver_test.go. Skip —
			// this property is about parity, not key coverage.
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
