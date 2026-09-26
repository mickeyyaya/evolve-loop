package phasecontract

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func lookupFor(specs ...phasespec.PhaseSpec) func(string) (phasespec.PhaseSpec, bool) {
	byName := map[string]phasespec.PhaseSpec{}
	for _, s := range specs {
		byName[s.Name] = s
	}
	return func(name string) (phasespec.PhaseSpec, bool) {
		s, ok := byName[name]
		return s, ok
	}
}

func TestResolveNativeExecutorWithoutOutputsHasNoContract(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "deploy-native", Kind: "native", Role: "orchestrator",
	}))
	if c, ok := r.Resolve("deploy-native"); ok {
		t.Fatalf("RED: native executor %q with no declared outputs got a synthesized contract (artifact=%q) — "+
			"a deterministic executor has no agent to write it; the enforce gate would block a phase it can "+
			"never satisfy (cycle-281)", "deploy-native", c.ArtifactName)
	}
}

func TestResolveNativeExecutorWithDeclaredOutputsKeepsContract(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "exporter", Kind: "native",
		Outputs: phasespec.IO{Files: []string{"export-manifest.json"}},
	}))
	c, ok := r.Resolve("exporter")
	if !ok {
		t.Fatal("native phase with explicit outputs.files must keep its derived contract")
	}
	if c.ArtifactName != "export-manifest.json" {
		t.Errorf("artifact=%q, want export-manifest.json", c.ArtifactName)
	}
}

func TestResolveLLMPhaseWithoutOutputsKeepsConventionContract(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "smell-scan", // empty Kind defaults to llm
	}))
	c, ok := r.Resolve("smell-scan")
	if !ok {
		t.Fatal("llm phase must keep the convention-derived contract")
	}
	if c.ArtifactName != "smell-scan-report.md" {
		t.Errorf("artifact=%q, want smell-scan-report.md", c.ArtifactName)
	}
}

func TestResolveBuiltinStaysAuthoritative(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "audit", Kind: "native", // contradicts the built-in audit contract
	}))
	c, ok := r.Resolve("audit")
	if !ok || c.ArtifactName != "audit-report.md" {
		t.Fatalf("built-in audit contract must stay authoritative, got ok=%v artifact=%q", ok, c.ArtifactName)
	}
}
