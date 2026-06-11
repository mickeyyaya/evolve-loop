package phasecontract

// resolver_native_skip_test.go — RED contract for the cycle-281 ship-contract
// noise: ship is a pure NATIVE executor (no LLM agent writes markdown), yet the
// spec-derived fallback invented a `ship-report.md` contract for it, so every
// shipping cycle hit 3 enforce-stage [missing_artifact] BLOCKs and survived
// only because the circuit breaker demoted enforce→advisory. The operator
// policy this pins: audit-PASS ⇒ ship must complete WITHOUT depending on a
// safety valve.
//
// Rule (single home: SynthesizesContract): a derived contract exists only when
// an LLM agent actually writes the artifact (kind "llm", the default) OR the
// spec explicitly declares outputs.files. Native/command executors with no
// declared outputs resolve to NO contract — the gate skips them, exactly like
// any other phase the resolver misses.

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

// TestResolveNativeExecutorWithoutOutputsHasNoContract: the ship shape. A
// native-kind spec with no outputs.files must resolve to a MISS. RED today:
// the fallback synthesizes ship-report.md and the enforce gate blocks a phase
// that can never satisfy it.
func TestResolveNativeExecutorWithoutOutputsHasNoContract(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "ship", Kind: "native", Role: "orchestrator",
	}))
	if c, ok := r.Resolve("ship"); ok {
		t.Fatalf("RED: native executor %q with no declared outputs got a synthesized contract (artifact=%q) — "+
			"a deterministic executor has no agent to write it; the enforce gate blocks 3× and only the "+
			"circuit breaker lets the cycle ship (cycle-281)", "ship", c.ArtifactName)
	}
}

// TestResolveNativeExecutorWithDeclaredOutputsKeepsContract: a native phase
// that EXPLICITLY declares its output file keeps a derived contract — the rule
// only kills convention-invented artifacts, never declared ones.
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

// TestResolveLLMPhaseWithoutOutputsKeepsConventionContract: the default LLM
// convention (<name>-report.md) is unchanged — user/minted agent phases still
// get the derived well-formedness contract.
func TestResolveLLMPhaseWithoutOutputsKeepsConventionContract(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "smell-scan", // Kind empty → defaults to "llm"
	}))
	c, ok := r.Resolve("smell-scan")
	if !ok {
		t.Fatal("llm phase must keep the convention-derived contract")
	}
	if c.ArtifactName != "smell-scan-report.md" {
		t.Errorf("artifact=%q, want smell-scan-report.md", c.ArtifactName)
	}
}

// TestResolveBuiltinStaysAuthoritative: built-in contracts are untouched by
// the synthesis rule (audit resolves from the hardcoded map regardless of any
// catalog spec shape).
func TestResolveBuiltinStaysAuthoritative(t *testing.T) {
	t.Parallel()
	r := NewCatalogResolver(lookupFor(phasespec.PhaseSpec{
		Name: "audit", Kind: "native", // adversarial: catalog lies about audit
	}))
	c, ok := r.Resolve("audit")
	if !ok || c.ArtifactName != "audit-report.md" {
		t.Fatalf("built-in audit contract must stay authoritative, got ok=%v artifact=%q", ok, c.ArtifactName)
	}
}
