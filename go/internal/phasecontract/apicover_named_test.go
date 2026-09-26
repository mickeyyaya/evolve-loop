package phasecontract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestFooterMarker_EmittedByFooter(t *testing.T) {
	c := mustContract(t, "build")
	const path = "/abs/.evolve/runs/cycle-7/build-report.md"
	footer := RenderContractFooter(c, path)

	if !strings.Contains(footer, FooterMarker+" "+path) {
		t.Fatalf("footer must carry %q directly before the path; got:\n%q", FooterMarker, footer)
	}
}

func TestSentinelSchemaVersion_StampedOnV1(t *testing.T) {
	line := RenderVerdictSentinel("audit", "PASS")
	s, ok := ParseVerdictSentinelFull(line)
	if !ok {
		t.Fatalf("v1 sentinel did not parse: %q", line)
	}
	if s.SchemaVersion != SentinelSchemaVersion {
		t.Errorf("v1 sentinel SchemaVersion=%d, want SentinelSchemaVersion=%d", s.SchemaVersion, SentinelSchemaVersion)
	}
	if SentinelSchemaVersion >= SentinelSchemaVersionFailure {
		t.Errorf("SentinelSchemaVersion(%d) must be below the failure version(%d)", SentinelSchemaVersion, SentinelSchemaVersionFailure)
	}
}

func TestTargetEvolveDir_RoutesOrchestratorToEvolveDir(t *testing.T) {
	c := mustContract(t, "orchestrator")
	if c.WriteTarget != TargetEvolveDir {
		t.Fatalf("orchestrator WriteTarget=%q, want TargetEvolveDir=%q", c.WriteTarget, TargetEvolveDir)
	}
	r := Roots{Workspace: "/ws", Worktree: "/wt", EvolveDir: "/ev"}
	if got, want := c.ArtifactPath(r), "/ev/cycle-state.json"; got != want {
		t.Errorf("TargetEvolveDir must route ArtifactPath to EvolveDir: got %q, want %q", got, want)
	}
	if TargetEvolveDir == TargetWorkspace {
		t.Error("TargetEvolveDir must differ from TargetWorkspace")
	}
}

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

	emptyFirst := phasespec.PhaseSpec{Name: "x", Kind: "native", Outputs: phasespec.IO{Files: []string{""}}}
	if SynthesizesContract(emptyFirst) {
		t.Error("an empty outputs.files[0] must not count as a declared output")
	}
}

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

	var r Resolver = NewCatalogResolver(lookup)
	if _, isCatalog := r.(CatalogResolver); !isCatalog {
		t.Fatalf("NewCatalogResolver must return a CatalogResolver; got %T", r)
	}

	c, ok := r.Resolve("lint-scan")
	if !ok || c.ArtifactName != "lint-scan-report.md" || len(c.Sections) != 1 {
		t.Fatalf("CatalogResolver.Resolve(lint-scan) = %+v, ok=%v; want lint-scan-report.md with 1 section", c, ok)
	}

	if bc, ok := r.Resolve("build"); !ok || bc.ArtifactName != "build-report.md" {
		t.Errorf("CatalogResolver must serve builtins first; got %+v ok=%v", bc, ok)
	}
}

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
		if want := alwaysOn(tc.report.Sections); !reflect.DeepEqual(c.Sections, want) {
			t.Errorf("%s: registry Contract.Sections not wired from alwaysOn(%s var)\n contract: %+v\n want:     %+v",
				tc.phase, tc.phase, c.Sections, want)
		}
	}
}
