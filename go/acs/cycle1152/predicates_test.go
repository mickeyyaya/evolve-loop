//go:build acs

package cycle1152

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var reportNames = map[string]string{
	string(core.PhaseScout): phasecontract.ArtifactName(string(core.PhaseScout)),
	string(core.PhaseTDD):   phasecontract.ArtifactName(string(core.PhaseTDD)),
	string(core.PhaseBuild): phasecontract.ArtifactName(string(core.PhaseBuild)),
	string(core.PhaseAudit): phasecontract.ArtifactName(string(core.PhaseAudit)),
}

var remainingCallSites = []string{
	"go/internal/phases/tdd/tdd.go",
	"go/internal/phases/ship/manifest.go",
}

var alreadyMigrated = []string{
	"go/internal/consensusdispatch/consensusdispatch.go",
	"go/internal/core/phase_bindings.go",
	"go/internal/core/build_removal_check.go",
	"go/internal/coherence/coherence.go",
	"go/internal/phases/audit/audit.go",
	"go/internal/phases/build/build.go",
}

func TestC1152_001_registry_is_the_artifact_name_ssot(t *testing.T) {
	divergent := []struct{ phase, want string }{
		{string(core.PhaseTDD), "test-report.md"},
		{"retro", "retrospective-report.md"},
		{"build-planner", "build-plan.md"},
	}
	for _, tc := range divergent {
		if got := phasecontract.ArtifactName(tc.phase); got != tc.want {
			t.Errorf("ArtifactName(%q) = %q, want %q — the registry must own this name", tc.phase, got, tc.want)
		}
		if got := phasecontract.ArtifactFilename(tc.phase); got != tc.want {
			t.Errorf("ArtifactFilename(%q) = %q, want %q", tc.phase, got, tc.want)
		}
		if conv := tc.phase + "-report.md"; conv == tc.want {
			t.Errorf("phase %q was chosen as a DIVERGENT case but its convention name equals its registry name — "+
				"this predicate no longer proves what it claims; pick a phase that actually diverges", tc.phase)
		}
	}

	for _, tc := range []struct{ phase, want string }{
		{string(core.PhaseScout), "scout-report.md"},
		{string(core.PhaseBuild), "build-report.md"},
		{string(core.PhaseAudit), "audit-report.md"},
	} {
		if got := phasecontract.ArtifactName(tc.phase); got != tc.want {
			t.Errorf("ArtifactName(%q) = %q, want %q", tc.phase, got, tc.want)
		}
	}

	if got := phasecontract.ArtifactName("ship"); got != "" {
		t.Errorf("ArtifactName(\"ship\") = %q, want \"\" — ship is NoArtifact (its result is a pushed commit)", got)
	}
	const unregistered = "cycle1152-not-a-registered-phase"
	if got := phasecontract.ArtifactName(unregistered); got != "" {
		t.Errorf("ArtifactName(%q) = %q, want \"\"", unregistered, got)
	}
	if got := phasecontract.ArtifactFilename(unregistered); got != unregistered+"-report.md" {
		t.Errorf("ArtifactFilename(%q) = %q, want the convention fallback %q", unregistered, got, unregistered+"-report.md")
	}
}

func TestC1152_002_remaining_callsites_resolve_through_ssot(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range remainingCallSites {
		src, ok := readSource(t, root, rel)
		if !ok {
			continue
		}
		for phase, name := range reportNames {
			if containsStringLiteral(t, rel, src, name) {
				t.Errorf("%s still carries the literal %q — resolve it through "+
					"phasecontract.ArtifactName(%q) (or ArtifactFilename) so a registry rename "+
					"cannot strand this call site", rel, name, phase)
			}
		}
		if !strings.Contains(src, "phasecontract.ArtifactName(") &&
			!strings.Contains(src, "phasecontract.ArtifactFilename(") {
			t.Errorf("%s does not call the phasecontract SSOT — the registry must be the only "+
				"declaration of the artifact-name vocabulary", rel)
		}
	}
}

func TestC1152_003_no_report_filename_literals_outside_phasecontract(t *testing.T) {
	root := acsassert.RepoRoot(t)
	internal := filepath.Join(root, "go", "internal")

	type finding struct {
		rel, name, pos string
	}
	var findings []finding

	fset := token.NewFileSet()
	err := filepath.WalkDir(internal, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "phasecontract" {
				return fs.SkipDir
			}
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			for _, name := range reportNames {
				if val == name {
					findings = append(findings, finding{rel: rel, name: name, pos: fset.Position(lit.Pos()).String()})
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", internal, err)
	}

	for _, f := range findings {
		t.Errorf("%s declares the report filename %q as a string literal (%s) — "+
			"go/internal/phasecontract is the SSOT; resolve it via phasecontract.ArtifactName",
			f.rel, f.name, f.pos)
	}
	if len(findings) > 0 {
		t.Logf("%d hand-rolled report-filename literal(s) remain outside phasecontract", len(findings))
	}
}

func TestC1152_004_no_handrolled_convention_and_no_regression(t *testing.T) {
	root := acsassert.RepoRoot(t)

	const suffix = "-report.md"
	forbidden := []string{`+ "` + suffix + `"`, `+"` + suffix + `"`}

	for _, rel := range remainingCallSites {
		src, ok := readSource(t, root, rel)
		if !ok {
			continue
		}
		for _, frag := range forbidden {
			if strings.Contains(src, frag) {
				t.Errorf("%s hand-rolls the %q convention — for the tdd phase that yields %q, "+
					"but the registry (and every agent doc) names it %q. Call phasecontract instead",
					rel, suffix, string(core.PhaseTDD)+suffix, phasecontract.ArtifactName(string(core.PhaseTDD)))
			}
		}
	}

	for _, rel := range alreadyMigrated {
		src, ok := readSource(t, root, rel)
		if !ok {
			continue
		}
		if !strings.Contains(src, "phasecontract.ArtifactName(") &&
			!strings.Contains(src, "phasecontract.ArtifactFilename(") {
			t.Errorf("%s no longer calls the phasecontract SSOT — cycle-1149 migrated this file; "+
				"completing the migration must not regress it", rel)
		}
		for _, name := range reportNames {
			if containsStringLiteral(t, rel, src, name) {
				t.Errorf("%s reacquired the literal %q — it was migrated in cycle-1149", rel, name)
			}
		}
	}
}

func TestC1152_005_repo_builds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go build ./... failed in %s: %v\n%s", cmd.Dir, err, out)
	}
}

func readSource(t *testing.T, root, rel string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Errorf("read %s: %v", rel, err)
		return "", false
	}
	return string(data), true
}

func containsStringLiteral(t *testing.T, rel, src, want string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Errorf("parse %s: %v", rel, err)
		return false
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if val, uerr := strconv.Unquote(lit.Value); uerr == nil && val == want {
			found = true
		}
		return true
	})
	return found
}
