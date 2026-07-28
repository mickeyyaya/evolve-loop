//go:build acs

// Package cycle1152 materialises the acceptance criteria for the single task
// triage committed to THIS cycle:
//
//   - artifact-name-ssot-remaining-callsites → route every remaining
//     hand-rolled report-filename literal in go/internal through the
//     phasecontract SSOT (ArtifactName / ArtifactFilename), completing the
//     migration cycle-1145 started and cycle-1149 left half-done.
//
// The deferred id (artifact-name-ssot-grep-guard) carries ZERO predicates —
// R9.3: predicates bind only to triage-committed work, and a predicate gating
// deferred work starves the committed task (the cycle-280 failure mode).
//
// Continuation context (ADR-0076). This worktree is a salvage continuation of
// cycle-1149, which already migrated SIX of the eight target files
// (consensusdispatch, core/phase_bindings, core/build_removal_check,
// coherence, phases/audit, phases/build). TWO literals survive and are what
// this cycle must close:
//
//	go/internal/phases/tdd/tdd.go:38      — the ArtifactFilename hook returns
//	                                        the literal, though the file already
//	                                        imports phasecontract.
//	go/internal/phases/ship/manifest.go:53 — the manifest list, whose comment
//	                                        wrongly claims "test" has no
//	                                        registry phase (it does: "tdd").
//
// Predicate strategy. This is a pure refactor: the literals currently EQUAL
// the registry's values, so no runtime observation can distinguish "reads the
// SSOT" from "carries an equal copy" — duplication is inherently a source-level
// property. The suite therefore pairs the two axes so neither half is gameable
// alone (go/acs/README.md sanctioned absence-check form, and the cycle-1147
// 001+005 precedent for this same task family):
//
//   - 001 is BEHAVIORAL over the SSOT itself. It calls ArtifactName /
//     ArtifactFilename and asserts their return values, including the phases
//     whose registry name DIVERGES from the "<phase>-report.md" convention.
//     It is the pairing anchor: a builder who greens 002-004 by deleting or
//     renaming the registry's tdd contract fails here.
//   - 002 is the duplication-ABSENCE check over the two surviving call sites.
//     RED today at both.
//   - 003 is the repo-wide invariant AC, enforced by PARSING go/internal with
//     go/parser and inspecting string literal AST nodes — not grep. Prose in
//     comments and error messages that merely mentions a report name is
//     correctly ignored; only a literal that IS the filename trips it. RED
//     today with exactly the two findings above.
//   - 004 is the anti-gaming NEGATIVE half. The obvious wrong fix — replacing
//     the literal with the hand-rolled `phase + "-report.md"` convention —
//     would green 002/003 while silently breaking tdd (whose registry name is
//     "test-report.md", not "tdd-report.md"). 004 rejects that fix, and also
//     pins the six files cycle-1149 already migrated so the salvaged work
//     cannot regress inside this cycle.
//   - 005 is BEHAVIORAL over the toolchain: `go build ./...` must succeed,
//     materialising the "no new import cycles" criterion.
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

// reportNames are the phase-report filenames the registry owns. Assembled from
// the registry at init rather than typed, so this predicate file does not itself
// contain the literals it forbids (a self-trip against 003's own scan, which
// deliberately does not exempt go/acs).
var reportNames = map[string]string{
	string(core.PhaseScout): phasecontract.ArtifactName(string(core.PhaseScout)),
	string(core.PhaseTDD):   phasecontract.ArtifactName(string(core.PhaseTDD)),
	string(core.PhaseBuild): phasecontract.ArtifactName(string(core.PhaseBuild)),
	string(core.PhaseAudit): phasecontract.ArtifactName(string(core.PhaseAudit)),
}

// remainingCallSites are the two files still carrying a hand-rolled literal.
var remainingCallSites = []string{
	"go/internal/phases/tdd/tdd.go",
	"go/internal/phases/ship/manifest.go",
}

// alreadyMigrated are the six files cycle-1149 salvaged. 004 pins them so this
// cycle cannot regress work it inherited.
var alreadyMigrated = []string{
	"go/internal/consensusdispatch/consensusdispatch.go",
	"go/internal/core/phase_bindings.go",
	"go/internal/core/build_removal_check.go",
	"go/internal/coherence/coherence.go",
	"go/internal/phases/audit/audit.go",
	"go/internal/phases/build/build.go",
}

// TestC1152_001_registry_is_the_artifact_name_ssot is the behavioral anchor.
//
// The whole task rests on one claim: the registry ALREADY knows every report
// filename, so no consumer needs its own copy. This predicate proves that claim
// by calling the SSOT — in particular for "tdd", whose registry name is
// "test-report.md". ship/manifest.go:49 currently asserts the opposite in a
// comment ("'test' has no registry phase, so its report name has no SSOT to
// resolve against"); that comment is factually wrong and this predicate is the
// refutation.
//
// Behavioral: cannot be greened by adding a magic string anywhere — the
// registry entries must exist and resolve.
func TestC1152_001_registry_is_the_artifact_name_ssot(t *testing.T) {
	// The divergent phases: registry name != "<phase>-report.md". These are the
	// reason hand-rolling the convention is a BUG and not merely duplication.
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

	// Convention-matching phases must keep resolving identically, so completing
	// the migration is behaviour-preserving everywhere except tdd.
	for _, tc := range []struct{ phase, want string }{
		{string(core.PhaseScout), "scout-report.md"},
		{string(core.PhaseBuild), "build-report.md"},
		{string(core.PhaseAudit), "audit-report.md"},
	} {
		if got := phasecontract.ArtifactName(tc.phase); got != tc.want {
			t.Errorf("ArtifactName(%q) = %q, want %q", tc.phase, got, tc.want)
		}
	}

	// Edge / OOD: NoArtifact and unregistered phases. ArtifactName carries the
	// "no registered artifact" distinction as ""; ArtifactFilename supplies the
	// convention instead. A call site that needs to TELL THEM APART must use
	// ArtifactName, so this distinction has to survive the migration.
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

// TestC1152_002_remaining_callsites_resolve_through_ssot is the duplication
// absence check over the two files this cycle must fix.
//
// Paired with 001: greening this by deleting the registry's tdd contract fails
// 001, so the two cannot be satisfied together by anything except the real fix.
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

// TestC1152_003_no_report_filename_literals_outside_phasecontract is the
// repo-wide invariant AC and the regression guard against this drift class
// recurring a third time (cycle-1145 → cycle-1149 → here).
//
// It PARSES every non-test .go file under go/internal and inspects string
// literal AST nodes, so a comment or an error message that merely mentions a
// report name — of which the tree has dozens, legitimately — is not a finding.
// Only a literal whose value IS the filename counts: that is a path being
// constructed, which is exactly what must route through the registry.
//
// The phasecontract package is exempt: it is the SSOT and the one place the
// names are allowed to be typed.
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
			// The SSOT itself is the one sanctioned declaration site.
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
		file, perr := parser.ParseFile(fset, path, nil, 0) // 0 ⇒ comments dropped
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

// TestC1152_004_no_handrolled_convention_and_no_regression is the negative,
// anti-gaming half.
//
// The tempting wrong fix for tdd.go is `string(core.PhaseTDD) + "-report.md"`,
// which greens 002 and 003 while producing "tdd-report.md" — a file the agent
// never writes, which is precisely the exit-81 timeout the tdd.go doc comment
// records as already having happened once. This predicate forbids that shape at
// the migrated sites.
//
// It also pins the six files cycle-1149 already migrated: they must still call
// the SSOT and must not have reacquired a literal. Without this, "completing the
// migration" could silently trade one set of literals for another.
func TestC1152_004_no_handrolled_convention_and_no_regression(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// Assembled at runtime so this file does not contain the fragment it forbids.
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

// TestC1152_005_repo_builds materialises the "no new import cycles" criterion.
// phasecontract is a leaf-ish package, but adding an import to ship/manifest.go
// or tdd.go is the one way this task could introduce a cycle, and the compiler
// is the only authority on that.
//
// Behavioral: runs the real toolchain and asserts its exit code.
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

// readSource reads a repo-relative source file, reporting rather than fataling
// so one missing file does not mask findings in the others.
func readSource(t *testing.T, root, rel string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Errorf("read %s: %v", rel, err)
		return "", false
	}
	return string(data), true
}

// containsStringLiteral reports whether src declares want as a Go string
// literal. Parsing (not substring search) is what keeps the dozens of legitimate
// prose mentions in comments and error messages from registering as findings.
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
