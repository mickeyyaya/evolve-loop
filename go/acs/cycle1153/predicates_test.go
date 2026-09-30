//go:build acs

package cycle1153

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var artifactLiterals = []string{"audit-report.md", "build-report.md"}

var quotedArtifactLiterals = []string{`"audit-report.md"`, `"build-report.md"`}

func TestC1153_001_SSOTAccessorsReturnRegisteredNames(t *testing.T) {
	for phase, want := range map[string]string{
		"audit": "audit-report.md",
		"build": "build-report.md",
	} {
		if got := phasecontract.ArtifactName(phase); got != want {
			t.Errorf("ArtifactName(%q) = %q, want %q — the SSOT the migrated call sites read is gone or renamed", phase, got, want)
		}
		if got := phasecontract.ArtifactFilename(phase); got != want {
			t.Errorf("ArtifactFilename(%q) = %q, want %q", phase, got, want)
		}
	}

	if got := phasecontract.ArtifactName(string(core.PhaseAudit)); got != "audit-report.md" {
		t.Errorf("ArtifactName(string(core.PhaseAudit)) = %q, want %q", got, "audit-report.md")
	}
	if got := phasecontract.ArtifactName(string(core.PhaseBuild)); got != "build-report.md" {
		t.Errorf("ArtifactName(string(core.PhaseBuild)) = %q, want %q", got, "build-report.md")
	}
	if got := phasecontract.ArtifactName(string(core.PhaseTDD)); got != "test-report.md" {
		t.Errorf("ArtifactName(string(core.PhaseTDD)) = %q, want %q — manifestReportFiles would silently resolve the wrong file", got, "test-report.md")
	}

	if got := phasecontract.ArtifactName("ship"); got != "" {
		t.Errorf("ArtifactName(\"ship\") = %q, want \"\" (NoArtifact) — the empty-vs-fallback distinction the gates rely on is broken", got)
	}
	if got := phasecontract.ArtifactFilename("ship"); got != "ship-report.md" {
		t.Errorf("ArtifactFilename(\"ship\") = %q, want the convention fallback %q", got, "ship-report.md")
	}

	for phase, want := range map[string]string{
		"no-such-phase": "no-such-phase-report.md",
		"":              "-report.md",
	} {
		if got := phasecontract.ArtifactFilename(phase); got != want {
			t.Errorf("ArtifactFilename(%q) = %q, want fallback %q", phase, got, want)
		}
		if got := phasecontract.ArtifactName(phase); got != "" {
			t.Errorf("ArtifactName(%q) = %q, want \"\" for an unregistered phase", phase, got)
		}
	}
}

func TestC1153_002_HookAndLedgerDelegateToSSOT(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cases := []struct {
		file, fn, why string
	}{
		{"go/internal/phases/audit/audit.go", "ArtifactFilename",
			"the audit hook implements the very interface whose job is to answer 'what file does this phase write'"},
		{"go/internal/phases/build/build.go", "ArtifactFilename",
			"same interface, build side"},
		{"go/internal/core/phase_bindings.go", "recordAuditBinding",
			"the audit-binding ledger writer hashes the artifact it names"},
		{"go/internal/core/phase_bindings.go", "recordBuildBinding",
			"the build-binding ledger writer hashes the artifact it names"},
	}
	for _, c := range cases {
		assertFuncDelegates(t, filepath.Join(root, c.file), c.fn, c.why)
	}
}

func TestC1153_003_DispatchAndGatesDelegateToSSOT(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cases := []struct {
		file, fn, why string
	}{
		{"go/internal/consensusdispatch/consensusdispatch.go", "Run",
			"the cross-CLI aggregator writes its output to the audit artifact path"},
		{"go/internal/coherence/coherence.go", "ReadCycleVerdicts",
			"the verdict reader the coherence gate depends on"},
		{"go/internal/core/build_removal_check.go", "readBuildReport",
			"the build-removal gate reads the build report and its promoted copy"},
	}
	for _, c := range cases {
		assertFuncDelegates(t, filepath.Join(root, c.file), c.fn, c.why)
	}

	manifestPath := filepath.Join(root, "go/internal/phases/ship/manifest.go")
	elems := varElements(t, manifestPath, "manifestReportFiles")
	if len(elems) == 0 {
		t.Fatalf("manifestReportFiles not found (or empty) in %s — the ship manifest declaration this task migrates is gone", manifestPath)
	}
	for i, e := range elems {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			t.Errorf("manifestReportFiles[%d] is the literal %s in %s — must resolve through phasecontract.ArtifactName", i, lit.Value, manifestPath)
			continue
		}
		call, ok := e.(*ast.CallExpr)
		if !ok || !isPhasecontractArtifactCall(call) {
			t.Errorf("manifestReportFiles[%d] in %s is neither a literal nor a phasecontract.ArtifactName/ArtifactFilename call — cannot confirm SSOT delegation", i, manifestPath)
		}
	}
}

func TestC1153_004_NoArtifactNameLiteralsInGoInternal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scanRoot := filepath.Join(root, "go", "internal")
	registry := filepath.Join(scanRoot, "phasecontract", "contract_registry.go")

	scanned := 0
	var offenders []string
	err := filepath.Walk(scanRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") || path == registry {
			return nil
		}
		scanned++
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			offenders = append(offenders, path+": parse error: "+perr.Error())
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			for _, bad := range artifactLiterals {
				if val == bad {
					rel, _ := filepath.Rel(root, path)
					offenders = append(offenders, rel+":"+
						strconv.Itoa(fset.Position(lit.Pos()).Line)+": "+lit.Value)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", scanRoot, err)
	}
	if scanned < 100 {
		t.Fatalf("only %d non-test .go files scanned under %s — the scan is vacuous, not clean", scanned, scanRoot)
	}
	if len(offenders) > 0 {
		t.Errorf("%d hand-rolled artifact-filename literal(s) remain in go/internal (must resolve through phasecontract):\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
	t.Logf("scanned %d non-test .go files under go/internal", scanned)
}

func TestC1153_005_CoherenceReadsRegistryNamedArtifact(t *testing.T) {
	name := phasecontract.ArtifactName("audit")
	if name == "" {
		t.Fatalf("ArtifactName(\"audit\") is empty — cannot construct the workspace this predicate observes")
	}
	sentinel := phasecontract.RenderVerdictSentinelWithFailure("audit", "PASS", nil)

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, name), []byte("# Audit\n"+sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	audit, _, auditRan := coherence.ReadCycleVerdicts(ws)
	if !auditRan {
		t.Errorf("ReadCycleVerdicts: auditRan=false with %q present — the migrated reader no longer finds the registry-named artifact", name)
	}
	if audit != "PASS" {
		t.Errorf("ReadCycleVerdicts: audit verdict = %q, want %q", audit, "PASS")
	}

	wsWrong := t.TempDir()
	if err := os.WriteFile(filepath.Join(wsWrong, "auditor-report.md"), []byte("# Audit\n"+sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("write near-miss artifact: %v", err)
	}
	if _, _, ran := coherence.ReadCycleVerdicts(wsWrong); ran {
		t.Errorf("ReadCycleVerdicts: auditRan=true for a workspace holding only \"auditor-report.md\" — the reader is not keyed on the registry name")
	}

	if v, _, ran := coherence.ReadCycleVerdicts(t.TempDir()); ran || v != "" {
		t.Errorf("ReadCycleVerdicts(empty workspace) = (%q, ran=%v), want (\"\", false)", v, ran)
	}
}

func assertFuncDelegates(t *testing.T, path, fn, why string) {
	t.Helper()
	lits, err := acsassert.CountInGoFunc(path, fn, quotedArtifactLiterals...)
	if err != nil {
		t.Errorf("%s / %s: %v (%s)", path, fn, err, why)
		return
	}
	if lits != 0 {
		t.Errorf("%s / %s: %d hand-rolled artifact-filename literal(s) remain — %s", path, fn, lits, why)
	}
	calls, err := acsassert.CountInGoFunc(path, fn,
		"phasecontract.ArtifactFilename(", "phasecontract.ArtifactName(")
	if err != nil {
		t.Errorf("%s / %s: %v", path, fn, err)
		return
	}
	if calls == 0 {
		t.Errorf("%s / %s: no phasecontract.ArtifactName/ArtifactFilename call — the literal was removed without delegating to the SSOT", path, fn)
	}
}

func varElements(t *testing.T, path, name string) []ast.Expr {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []ast.Expr
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, id := range vs.Names {
				if id.Name != name || i >= len(vs.Values) {
					continue
				}
				if cl, ok := vs.Values[i].(*ast.CompositeLit); ok {
					out = append(out, cl.Elts...)
				}
			}
		}
	}
	return out
}

func isPhasecontractArtifactCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "phasecontract" {
		return false
	}
	return sel.Sel.Name == "ArtifactName" || sel.Sel.Name == "ArtifactFilename"
}
