package deliverable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// fooSpecLookup returns a lookup serving a single user phase "foo" whose
// contract is derived from its spec (require_sections: ["Findings"]).
func fooSpecLookup() func(string) (phasespec.PhaseSpec, bool) {
	foo := phasespec.PhaseSpec{
		Name:     "foo",
		Role:     "evaluate",
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"Findings"}},
		Outputs:  phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/foo-report.md"}},
	}
	return func(name string) (phasespec.PhaseSpec, bool) {
		if name == "foo" {
			return foo, true
		}
		return phasespec.PhaseSpec{}, false
	}
}

func TestVerifyWith_UserPhaseWellFormed(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "foo-report.md"), []byte("# Foo\n\n## Findings\n- nothing alarming\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := VerifyWith("foo", phasecontract.Roots{Workspace: ws}, phasecontract.NewCatalogResolver(fooSpecLookup()))
	if err != nil {
		t.Fatalf("VerifyWith returned err: %v", err)
	}
	if !res.OK {
		t.Errorf("expected OK for well-formed user phase; got violations %+v", res.Violations)
	}
}

func TestVerifyWith_UserPhaseMissingSection(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "foo-report.md"), []byte("# Foo\n\nno required heading here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := VerifyWith("foo", phasecontract.Roots{Workspace: ws}, phasecontract.NewCatalogResolver(fooSpecLookup()))
	if err != nil {
		t.Fatalf("VerifyWith returned err: %v", err)
	}
	if res.OK {
		t.Fatal("expected a violation for the missing ## Findings section")
	}
	found := false
	for _, v := range res.Violations {
		if v.Code == CodeMissingSection {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CodeMissingSection; got %+v", res.Violations)
	}
}

func TestVerifyWith_BuiltinOverridesUserSpec(t *testing.T) {
	ws := t.TempDir()
	// Write the built-in build deliverable correctly.
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("## Changes\n- did things\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A resolver whose lookup would hijack "build" must still serve the built-in.
	lookup := func(name string) (phasespec.PhaseSpec, bool) {
		return phasespec.PhaseSpec{Name: "build", Outputs: phasespec.IO{Files: []string{"hijacked.md"}}}, true
	}
	res, err := VerifyWith("build", phasecontract.Roots{Workspace: ws}, phasecontract.NewCatalogResolver(lookup))
	if err != nil {
		t.Fatalf("VerifyWith(build) err: %v", err)
	}
	if res.ArtifactPath != filepath.Join(ws, "build-report.md") {
		t.Errorf("built-in contract should win: ArtifactPath=%s", res.ArtifactPath)
	}
}

func TestVerifyWith_UnknownPhaseFailsOpen(t *testing.T) {
	_, err := VerifyWith("nope", phasecontract.Roots{Workspace: t.TempDir()}, phasecontract.NewCatalogResolver(nil))
	if err == nil {
		t.Error("expected fail-open error for a phase with no contract anywhere")
	}
}

// TestVerify_BackCompat confirms the legacy Verify still resolves built-ins via
// the BuiltinResolver default.
func TestVerify_BackCompat(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("## Changes\n- x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil || !res.OK {
		t.Errorf("Verify(build) back-compat broke: ok=%v err=%v viol=%+v", res.OK, err, res.Violations)
	}
}
