//go:build acs

// Package cycle1153 materialises the acceptance criteria for the two tasks
// triage committed to THIS cycle, both under the fleet-scoped todo-id
// `artifact-name-ssot-remaining-callsites`:
//
//   - audit-name-ssot-hook-and-ledger  → the phase-hook (phases/audit,
//     phases/build) and ledger-binding (core/phase_bindings) layers must
//     derive their report filename from the phasecontract SSOT.
//   - audit-name-ssot-dispatch-and-gates → the cross-CLI dispatch
//     (consensusdispatch), verdict reader (coherence), build-removal gate
//     (core/build_removal_check) and ship manifest (phases/ship) must do the
//     same.
//
// The deferred id (artifact-name-lint-guard) carries ZERO predicates — R9.3:
// predicates bind only to triage-committed work, and a predicate gating
// deferred work starves the committed task (the cycle-280 failure mode).
//
// Continuation context (ADR-0076). This worktree is a salvage continuation:
// commit bd9d408d ("salvage snapshot") already carries a candidate
// implementation for both tasks, so these predicates are pre-existing GREEN on
// HEAD. Their RED was demonstrated against the pre-salvage tree state (parent
// commit 77dfdbc9) — see test-report.md § RED Run Output. They remain
// load-bearing: they are the contract the audit gate replays, and they fail
// LOUDLY if the salvaged implementation is reverted, partially landed, or
// re-drifts.
//
// Predicate strategy. This is a pure refactor: every literal being removed is
// currently EQUAL to the registry's value, so no runtime observation can
// distinguish "reads the SSOT" from "carries an equal copy" — duplication is
// inherently a source-level property. The suite therefore pairs three axes so
// no single one is gameable alone:
//
//   - 001 and 005 are BEHAVIORAL: they invoke the SSOT accessors and the
//     exported consumer (coherence.ReadCycleVerdicts) and assert on returned
//     values and real side effects, including negative and edge inputs. 001 is
//     the pairing anchor — a builder who greens the absence checks by deleting
//     or renaming the registry's audit/build contracts fails here.
//   - 002 and 003 are duplication-ABSENCE checks scoped to the exact functions
//     and var the two tasks name, paired with a positive "delegates to the
//     accessor" assertion so deleting the call site cannot green them.
//   - 004 is the repo-wide invariant AC, enforced by PARSING go/internal with
//     go/parser and inspecting string-literal AST nodes — not grep. Prose in
//     comments and error messages that merely mentions a report name is
//     correctly ignored; only a literal that IS the filename trips it.
//
// The absence checks are un-gameable by string insertion: adding the magic
// string makes them FAIL, never pass (the inverse of the cycle-85 degenerate
// predicate failure mode).
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

// artifactLiterals are the two filenames this cycle removes from every
// non-registry, non-test call site in go/internal.
var artifactLiterals = []string{"audit-report.md", "build-report.md"}

// quotedArtifactLiterals is the source form the absence checks look for, so a
// mention inside a comment or an error-message sentence does not trip them.
var quotedArtifactLiterals = []string{`"audit-report.md"`, `"build-report.md"`}

// ── 001 — BEHAVIORAL anchor over the SSOT itself ─────────────────────────────

// TestC1153_001_SSOTAccessorsReturnRegisteredNames invokes ArtifactName and
// ArtifactFilename and asserts their return values. This is the pairing anchor
// for 002/003/004: those tests assert the literals are GONE, which a builder
// could also achieve by deleting the registry entries they are supposed to
// delegate to. This test fails in that case.
func TestC1153_001_SSOTAccessorsReturnRegisteredNames(t *testing.T) {
	// Positive: the two names this cycle's call sites must resolve through.
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

	// The core.Phase constants the hooks pass must key the same contracts —
	// a hook that delegates with the wrong key resolves to the convention
	// fallback and silently keeps working until a phase name diverges.
	if got := phasecontract.ArtifactName(string(core.PhaseAudit)); got != "audit-report.md" {
		t.Errorf("ArtifactName(string(core.PhaseAudit)) = %q, want %q", got, "audit-report.md")
	}
	if got := phasecontract.ArtifactName(string(core.PhaseBuild)); got != "build-report.md" {
		t.Errorf("ArtifactName(string(core.PhaseBuild)) = %q, want %q", got, "build-report.md")
	}
	// ship/manifest.go resolves the TDD report through the same accessor, and
	// its registered name DIVERGES from the <phase>-report.md convention — the
	// one case where ArtifactName and the fallback disagree.
	if got := phasecontract.ArtifactName(string(core.PhaseTDD)); got != "test-report.md" {
		t.Errorf("ArtifactName(string(core.PhaseTDD)) = %q, want %q — manifestReportFiles would silently resolve the wrong file", got, "test-report.md")
	}

	// NEGATIVE: ArtifactName must keep returning "" for a NoArtifact phase,
	// which is how ship/manifest.go's choice of ArtifactName over
	// ArtifactFilename stays meaningful (a lost registration must surface as
	// empty, not as a plausible-but-wrong "ship-report.md").
	if got := phasecontract.ArtifactName("ship"); got != "" {
		t.Errorf("ArtifactName(\"ship\") = %q, want \"\" (NoArtifact) — the empty-vs-fallback distinction the gates rely on is broken", got)
	}
	if got := phasecontract.ArtifactFilename("ship"); got != "ship-report.md" {
		t.Errorf("ArtifactFilename(\"ship\") = %q, want the convention fallback %q", got, "ship-report.md")
	}

	// EDGE / OOD: unregistered and empty phase keys take the convention
	// fallback rather than panicking or returning a registered name.
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

// ── 002 — Task 1: phase-hook + ledger-binding call sites ─────────────────────

// TestC1153_002_HookAndLedgerDelegateToSSOT asserts the four call sites of
// task `audit-name-ssot-hook-and-ledger` carry NO artifact-filename literal and
// DO call the SSOT accessor. Both halves are required: absence alone is greened
// by deleting the code, presence alone by leaving the literal beside the call.
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

// ── 003 — Task 2: dispatch, verdict reader and ship-side gates ───────────────

// TestC1153_003_DispatchAndGatesDelegateToSSOT is 002's counterpart for task
// `audit-name-ssot-dispatch-and-gates`. ship/manifest.go's call site is a
// package-level var rather than a function, so it is checked by inspecting that
// var's AST value directly.
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

	// ship/manifest.go — package-level var, checked via AST.
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

// ── 004 — repo-wide invariant (the cycle's headline AC) ─────────────────────

// TestC1153_004_NoArtifactNameLiteralsInGoInternal is the acceptance criterion
// both tasks share: zero remaining occurrences of the two literals in
// go/internal, outside the registry that DEFINES them and outside _test.go
// files (tests legitimately pin the expected string). Enforced by parsing every
// file and inspecting string-literal AST nodes, so a filename mentioned in a
// comment or an error-message sentence does not trip it.
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
	// Guard against a vacuous pass: an empty or unreachable scan root would
	// otherwise report "no offenders" and green this predicate for free.
	if scanned < 100 {
		t.Fatalf("only %d non-test .go files scanned under %s — the scan is vacuous, not clean", scanned, scanRoot)
	}
	if len(offenders) > 0 {
		t.Errorf("%d hand-rolled artifact-filename literal(s) remain in go/internal (must resolve through phasecontract):\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
	t.Logf("scanned %d non-test .go files under go/internal", scanned)
}

// ── 005 — BEHAVIORAL: the migrated consumer still reads the right file ───────

// TestC1153_005_CoherenceReadsRegistryNamedArtifact invokes the exported
// consumer that task 2 migrated and asserts on its real behavior against a real
// workspace on disk: a file named by the REGISTRY is found and its verdict
// parsed; a differently-named file is not. This is the no-behavior-change half
// of the AC — the refactor must not have changed WHICH file is read.
func TestC1153_005_CoherenceReadsRegistryNamedArtifact(t *testing.T) {
	name := phasecontract.ArtifactName("audit")
	if name == "" {
		t.Fatalf("ArtifactName(\"audit\") is empty — cannot construct the workspace this predicate observes")
	}
	sentinel := phasecontract.RenderVerdictSentinelWithFailure("audit", "PASS", nil)

	// POSITIVE: the registry-named artifact is read and its verdict parsed.
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

	// NEGATIVE: a plausible near-miss filename must NOT satisfy the reader —
	// proves the assertion above is load-bearing on the name, not on any file.
	wsWrong := t.TempDir()
	if err := os.WriteFile(filepath.Join(wsWrong, "auditor-report.md"), []byte("# Audit\n"+sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("write near-miss artifact: %v", err)
	}
	if _, _, ran := coherence.ReadCycleVerdicts(wsWrong); ran {
		t.Errorf("ReadCycleVerdicts: auditRan=true for a workspace holding only \"auditor-report.md\" — the reader is not keyed on the registry name")
	}

	// EDGE: an empty workspace yields no verdict and no error path — never a
	// fabricated verdict (the cycle-603 echo bug this reader guards).
	if v, _, ran := coherence.ReadCycleVerdicts(t.TempDir()); ran || v != "" {
		t.Errorf("ReadCycleVerdicts(empty workspace) = (%q, ran=%v), want (\"\", false)", v, ran)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// assertFuncDelegates asserts that the named top-level function of a Go file
// carries no quoted artifact-filename literal AND calls a phasecontract
// artifact-name accessor. A missing file or missing function fails LOUDLY —
// a renamed function must never satisfy the ==0 half silently.
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

// varElements returns the composite-literal elements of the named package-level
// var, or nil when the var (or its literal) is absent.
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

// isPhasecontractArtifactCall reports whether e is a call to
// phasecontract.ArtifactName or phasecontract.ArtifactFilename.
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
