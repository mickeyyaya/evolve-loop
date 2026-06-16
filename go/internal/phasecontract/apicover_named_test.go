package phasecontract

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5). Names AND
// exercises the exported symbols apicover flagged UNCOVERED. Every assertion
// drives a real producer/consumer (never a bare reference):
//
//	const FooterMarker         — the prefix RenderContractFooter emits; we render
//	                             a footer and assert it leads with the marker.
//	const SentinelSchemaVersion — the v1 payload version RenderVerdictSentinel
//	                             stamps; we round-trip a no-failure sentinel and
//	                             assert the parsed SchemaVersion equals it.
//	const TargetEvolveDir      — the WriteTarget that routes ArtifactPath to the
//	                             .evolve dir; asserted via the orchestrator
//	                             contract (its sole evolve_dir consumer).
//	func  SynthesizesContract  — invoked across llm / native-no-outputs /
//	                             native-with-outputs specs; the predicate's three
//	                             real branches.
//	type  CatalogResolver      — bound to the Resolver interface (satisfaction)
//	                             and exercised via Resolve over a spec lookup.
//	type  VerdictSentinel      — produced by RenderVerdictSentinelWithFailure and
//	                             materialized by ParseVerdictSentinelFull; we
//	                             assert every field of the parsed value.
//	vars  Audit/Build/Intent/Scout/Triage — the builtin phase Report values; each
//	                             is wired into its registry Contract.Sections, so
//	                             we assert For(phase).Sections == <Var>.Sections
//	                             (the registry is the real consumer) plus the
//	                             var's own load-bearing Phase field.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestFooterMarker_EmittedByFooter — RenderContractFooter is the producer of the
// volatile path line; it must emit FooterMarker directly before the path so
// tooling can locate it unambiguously at the end of the prompt.
func TestFooterMarker_EmittedByFooter(t *testing.T) {
	c := mustContract(t, "build")
	const path = "/abs/.evolve/runs/cycle-7/build-report.md"
	footer := RenderContractFooter(c, path)

	if !strings.Contains(footer, FooterMarker+" "+path) {
		t.Fatalf("footer must carry %q directly before the path; got:\n%q", FooterMarker, footer)
	}
}

// TestSentinelSchemaVersion_StampedOnV1 — RenderVerdictSentinel (the no-failure
// producer) stamps schema_version=SentinelSchemaVersion; ParseVerdictSentinelFull
// must read it back. This binds the const to its real producer→parser path.
func TestSentinelSchemaVersion_StampedOnV1(t *testing.T) {
	line := RenderVerdictSentinel("audit", "PASS")
	s, ok := ParseVerdictSentinelFull(line)
	if !ok {
		t.Fatalf("v1 sentinel did not parse: %q", line)
	}
	if s.SchemaVersion != SentinelSchemaVersion {
		t.Errorf("v1 sentinel SchemaVersion=%d, want SentinelSchemaVersion=%d", s.SchemaVersion, SentinelSchemaVersion)
	}
	// Sanity: the v1 baseline is the lower of the two versions (failure bumps it).
	if SentinelSchemaVersion >= SentinelSchemaVersionFailure {
		t.Errorf("SentinelSchemaVersion(%d) must be below the failure version(%d)", SentinelSchemaVersion, SentinelSchemaVersionFailure)
	}
}

// TestTargetEvolveDir_RoutesOrchestratorToEvolveDir — TargetEvolveDir is the
// WriteTarget the orchestrator contract uses; ArtifactPath must then resolve the
// artifact under Roots.EvolveDir (the const's only consumer in the registry).
func TestTargetEvolveDir_RoutesOrchestratorToEvolveDir(t *testing.T) {
	c := mustContract(t, "orchestrator")
	if c.WriteTarget != TargetEvolveDir {
		t.Fatalf("orchestrator WriteTarget=%q, want TargetEvolveDir=%q", c.WriteTarget, TargetEvolveDir)
	}
	r := Roots{Workspace: "/ws", Worktree: "/wt", EvolveDir: "/ev"}
	if got, want := c.ArtifactPath(r), "/ev/cycle-state.json"; got != want {
		t.Errorf("TargetEvolveDir must route ArtifactPath to EvolveDir: got %q, want %q", got, want)
	}
	// Distinct from the workspace target (proves the const selects a branch).
	if TargetEvolveDir == TargetWorkspace {
		t.Error("TargetEvolveDir must differ from TargetWorkspace")
	}
}

// TestSynthesizesContract_Branches — exercises the predicate's three real
// outcomes: an llm phase synthesizes; a native/command phase with no declared
// outputs does NOT (the unsatisfiable-ship-contract guard, cycle-281); a native
// phase that declares outputs.files DOES.
func TestSynthesizesContract_Branches(t *testing.T) {
	llm := phasespec.PhaseSpec{Name: "review", Kind: "llm"}
	if !SynthesizesContract(llm) {
		t.Error("an llm-kind phase must synthesize a contract")
	}

	nativeNoOutputs := phasespec.PhaseSpec{Name: "ship", Kind: "native"}
	if SynthesizesContract(nativeNoOutputs) {
		t.Error("a native phase with no declared outputs must NOT synthesize a contract (cycle-281)")
	}

	nativeWithOutputs := phasespec.PhaseSpec{
		Name: "emit", Kind: "command",
		Outputs: phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/emit.json"}},
	}
	if !SynthesizesContract(nativeWithOutputs) {
		t.Error("a native/command phase that declares outputs.files must synthesize a contract")
	}

	// An empty first output file is treated as no output (the [0] != "" guard).
	emptyFirst := phasespec.PhaseSpec{Name: "x", Kind: "native", Outputs: phasespec.IO{Files: []string{""}}}
	if SynthesizesContract(emptyFirst) {
		t.Error("an empty outputs.files[0] must not count as a declared output")
	}
}

// TestCatalogResolver_SatisfiesResolverAndResolves — CatalogResolver must satisfy
// the Resolver interface (compile-time + runtime binding) and, when exercised,
// derive a Contract from a spec the builtin map misses while never overriding a
// builtin (the override-precedence invariant the type exists to guarantee).
func TestCatalogResolver_SatisfiesResolverAndResolves(t *testing.T) {
	userSpec := phasespec.PhaseSpec{
		Name:     "lint-scan",
		Role:     "evaluate",
		Kind:     "llm",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
		Outputs:  phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/lint-scan-report.md"}},
	}
	lookup := func(name string) (phasespec.PhaseSpec, bool) {
		if name == "lint-scan" {
			return userSpec, true
		}
		return phasespec.PhaseSpec{}, false
	}

	// Interface satisfaction: assigned to the Resolver seam and used through it.
	var r Resolver = NewCatalogResolver(lookup)
	if _, isCatalog := r.(CatalogResolver); !isCatalog {
		t.Fatalf("NewCatalogResolver must return a CatalogResolver; got %T", r)
	}

	// Exercise: a user phase resolves via FromSpec derivation.
	c, ok := r.Resolve("lint-scan")
	if !ok || c.ArtifactName != "lint-scan-report.md" || len(c.Sections) != 1 {
		t.Fatalf("CatalogResolver.Resolve(lint-scan) = %+v, ok=%v; want lint-scan-report.md with 1 section", c, ok)
	}

	// Exercise: a builtin is served from the builtin layer, not the spec lookup.
	if bc, ok := r.Resolve("build"); !ok || bc.ArtifactName != "build-report.md" {
		t.Errorf("CatalogResolver must serve builtins first; got %+v ok=%v", bc, ok)
	}
}

// TestVerdictSentinel_ProducedAndParsed — VerdictSentinel is the parsed sentinel
// payload. We produce a v2 line via RenderVerdictSentinelWithFailure and
// materialize the struct via ParseVerdictSentinelFull, asserting every field
// (Phase/Verdict/SchemaVersion/Failure) round-trips. This exercises the type, not
// just names it.
func TestVerdictSentinel_ProducedAndParsed(t *testing.T) {
	fb := &FailureBlock{
		Class:         "code-audit-fail",
		Defects:       []string{"nil deref in walk()"},
		EvidencePaths: []string{"acs-verdict.json"},
	}
	line := RenderVerdictSentinelWithFailure("audit", "FAIL", fb)

	var got VerdictSentinel
	got, ok := ParseVerdictSentinelFull("# Audit Report\n" + line + "\n")
	if !ok {
		t.Fatalf("VerdictSentinel did not parse from producer output: %q", line)
	}
	want := VerdictSentinel{
		Phase:         "audit",
		Verdict:       "FAIL",
		SchemaVersion: SentinelSchemaVersionFailure,
		Failure:       fb,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("VerdictSentinel round-trip mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// TestBuiltinReportVars_WiredIntoRegistry — Audit/Build/Intent/Scout/Triage are
// the builtin phase Report values; the registry contract for each phase wires its
// Sections from the matching var (contract_registry.go: Sections: Build.Sections,
// etc.). Asserting For(phase).Sections is the SAME slice the var declares proves
// the var is the live single source consumed by the registry — and pins each
// var's Phase field as load-bearing.
func TestBuiltinReportVars_WiredIntoRegistry(t *testing.T) {
	cases := []struct {
		phase  string
		report Report
	}{
		{"build", Build},
		{"scout", Scout},
		{"audit", Audit},
		{"intent", Intent},
		{"triage", Triage},
	}
	for _, tc := range cases {
		c := mustContract(t, tc.phase)
		if tc.report.Phase != tc.phase {
			t.Errorf("%s var: Phase=%q, want %q", tc.phase, tc.report.Phase, tc.phase)
		}
		if len(tc.report.Sections) == 0 {
			t.Errorf("%s var declares no sections", tc.phase)
		}
		if !reflect.DeepEqual(c.Sections, tc.report.Sections) {
			t.Errorf("%s: registry Contract.Sections not wired from the %s var\n contract: %+v\n var:      %+v",
				tc.phase, tc.phase, c.Sections, tc.report.Sections)
		}
	}
}
