package phasecontract

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestBuiltinResolver(t *testing.T) {
	var r Resolver = BuiltinResolver{}

	if c, ok := r.Resolve("build"); !ok || c.ArtifactName != "build-report.md" {
		t.Errorf("BuiltinResolver.Resolve(build) = %+v, ok=%v; want build-report.md", c, ok)
	}
	if _, ok := r.Resolve("foo"); ok {
		t.Error("BuiltinResolver.Resolve(foo) should miss for an unknown phase")
	}
	if _, ok := r.Resolve(""); ok {
		t.Error("BuiltinResolver.Resolve(empty) must miss cleanly, not panic")
	}
	// Alias path preserved.
	if c, ok := r.Resolve("advisor"); !ok || c.ArtifactName != "routing-plan.json" {
		t.Errorf("BuiltinResolver.Resolve(advisor) = %+v, ok=%v; want routing-plan.json via alias", c, ok)
	}
}

func TestCatalogResolver(t *testing.T) {
	fooSpec := phasespec.PhaseSpec{
		Name:     "foo",
		Role:     "evaluate",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
		Outputs:  phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/foo-report.md"}},
	}
	// A spec that clashes with a built-in — must NOT override the built-in.
	buildShadow := phasespec.PhaseSpec{
		Name:    "build",
		Outputs: phasespec.IO{Files: []string{"hijacked.md"}},
	}
	lookup := func(name string) (phasespec.PhaseSpec, bool) {
		switch name {
		case "foo":
			return fooSpec, true
		case "build":
			return buildShadow, true
		default:
			return phasespec.PhaseSpec{}, false
		}
	}
	r := NewCatalogResolver(lookup)

	// User phase resolves via spec derivation.
	c, ok := r.Resolve("foo")
	if !ok {
		t.Fatal("CatalogResolver.Resolve(foo) should resolve via FromSpec")
	}
	if c.ArtifactName != "foo-report.md" || len(c.Sections) != 1 {
		t.Errorf("Resolve(foo) = %+v; want foo-report.md with 1 section", c)
	}

	// Built-in wins over a clashing user spec (override precedence).
	c, ok = r.Resolve("build")
	if !ok || c.ArtifactName != "build-report.md" {
		t.Errorf("Resolve(build) = %+v, ok=%v; want built-in build-report.md (not hijacked.md)", c, ok)
	}

	// Unknown everywhere → miss.
	if _, ok := r.Resolve("nope"); ok {
		t.Error("Resolve(nope) should miss when absent from built-ins and lookup")
	}

	// Alias still resolves through the built-in layer.
	if c, ok := r.Resolve("advisor"); !ok || c.ArtifactName != "routing-plan.json" {
		t.Errorf("Resolve(advisor) = %+v, ok=%v; want routing-plan.json", c, ok)
	}
}

func TestCatalogResolver_NilLookupIsBuiltinOnly(t *testing.T) {
	r := NewCatalogResolver(nil)
	if c, ok := r.Resolve("build"); !ok || c.ArtifactName != "build-report.md" {
		t.Errorf("nil-lookup resolver should still serve built-ins; got %+v ok=%v", c, ok)
	}
	if _, ok := r.Resolve("foo"); ok {
		t.Error("nil-lookup resolver must miss for user phases (no panic)")
	}
}
