package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// integrity_surface_explanation_callsites_test.go closes a real gap: #549
// relocated five explanation-lifecycle call sites out of files the manifest
// protected (cyclerun_review.go, resume.go, audit.go, runner.go) into files
// it does not (cyclerun_postreview.go, resume_execution.go, resume_bootstrap.go,
// phases/audit/classification.go, phases/runner/dispatch.go). Two independent
// beliefs about "where the protected call sites are" — the manifest and the
// four AST pin tables/columns in build_explanation_wiring_test.go,
// explanation_activation_belt_test.go and explanation_review_ssot_test.go —
// had drifted apart, which is exactly the class the repo's
// single-source-with-projection rule forbids.
//
// Rather than hand-pinning the five files (which only re-creates a fifth
// belief that can drift again), explanationLifecycleVocabulary derives the
// protected CALL-SITE vocabulary from beliefs that already exist: every
// callee, value and gate-entry-point column of those pin tables, plus the
// function names build_explanation_handoff.go and the explanationdocs
// package themselves declare (both already manifest rows whose stated
// purpose IS the lifecycle API). explanationCallSiteFiles then scans the
// tree for any file containing a call, or a struct-literal field assigned by
// value, that resolves to a name in that vocabulary. A file hit by the scan
// but absent from the manifest is the actual defect this closes.
//
// This closes the classes it can see: a call, or a struct-literal field
// assigned a call or a bare/qualified function value, whose resolved name is
// already known to be part of the lifecycle API. It does NOT make every
// future relocation structurally impossible — see
// TestExplanationHandoffVocabulary_IncludesKnownLifecycleFunctions and
// TestExplanationDocsVocabulary_IncludesKnownVerificationFunctions below,
// which pin that the two DERIVATION SOURCES themselves keep naming the
// functions this file's vocabulary depends on, so a future split of either
// source file is caught directly rather than by incidental file-level
// coverage. The scanner's root is go/internal only (root ".." from this
// package) — go/cmd and go/acs are not walked, matching the scope of the
// five files this change was written to protect; no call site into this
// vocabulary lives outside go/internal today.

// explanationLifecycleVocabulary is the union of every already-declared
// belief about which function names constitute the explanation-lifecycle
// API.
func explanationLifecycleVocabulary(t *testing.T) map[string]bool {
	t.Helper()
	vocab := map[string]bool{
		explanationActivationBeltCallee: true,
		explanationReviewGateCallee:     true,
	}
	for _, pin := range explanationLifecycleCallPins {
		vocab[pin.callee] = true
	}
	for _, pin := range explanationLifecycleAssignPins {
		vocab[pin.value] = true
	}
	// The activation belt's gate ENTRY POINTS (e.g. verifyExplanationDocumentation)
	// are function names too, even though the belt test pins them as the
	// function being checked rather than a callee — a struct-literal field
	// assigned one of them by value (audit.go's `CheckExplanation:
	// verifyExplanationDocumentation`) is exactly the wiring
	// explanationCallSiteFiles' KeyValueExpr match exists to catch
	// (architecture review 2026-09-12 round 2, HIGH: verifyNativeExplanation
	// happened to already be a call-pin callee; verifyExplanationDocumentation
	// was not in vocab anywhere, so this same class of wiring for Audit was
	// invisible).
	for _, pin := range explanationActivationBeltPins {
		vocab[pin.fn] = true
	}
	names, err := parseFuncDeclNames("../core/build_explanation_handoff.go")
	if err != nil {
		t.Fatalf("build_explanation_handoff.go must parse: %v", err)
	}
	for name := range names {
		vocab[name] = true
	}
	edocs, err := explanationDocsExportedNames("../explanationdocs")
	if err != nil {
		t.Fatalf("explanationdocs must parse: %v", err)
	}
	for name := range edocs {
		vocab["explanationdocs."+name] = true
	}
	return vocab
}

// parseFuncDeclNames returns every top-level FREE function name declared in
// path (methods excluded), by parsing it once rather than importing it. A
// method is never callable by its bare short name — `apply` or `Review`
// declared on a receiver can only ever appear as "x.apply"/"x.Review" at a
// call site — so including one in a bare-name vocabulary would be pure dead
// weight at best and a same-named free-function collision risk at worst
// (architecture review 2026-09-12 round 2, MEDIUM). It is a pure function —
// no *testing.T — precisely so its error path is a normal assertion (see
// TestParseFuncDeclNames_MissingFileErrors) rather than a Fatal a test can't
// observe.
func parseFuncDeclNames(path string) (map[string]bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Recv == nil {
			names[fn.Name.Name] = true
		}
	}
	return names, nil
}

func TestParseFuncDeclNames_MissingFileErrors(t *testing.T) {
	if _, err := parseFuncDeclNames(filepath.Join(t.TempDir(), "does-not-exist.go")); err == nil {
		t.Fatal("want an error for a file that does not exist, got nil")
	}
}

// explanationDocsExcludedDocumentPath is the one exported explanationdocs
// function deliberately NOT part of the call-site vocabulary: pure path
// formatting for a prompt hint (cycle+runID -> a filename string), making no
// trust decision — the actual verification a caller of this path would need
// is Verify/VerifyLanded/CheckBuild, which ARE in the vocabulary. Excluding
// it matters operationally, not just semantically: its only non-test caller,
// go/internal/phases/build/build.go, is deliberately NOT a protected
// surface, and including this name would force a manifest row for the whole
// build phase over a prompt hint (architecture review 2026-09-12 round 2).
const explanationDocsExcludedDocumentPath = "DocumentPath"

// explanationDocsExportedNames returns every exported top-level function
// name declared directly in dir's non-test .go files, minus
// explanationDocsExcludedDocumentPath. explanationdocs is already a
// manifest-protected directory whose entire stated purpose is the lifecycle
// contract, so its own exported surface is a source of vocabulary that
// requires no new belief.
func explanationDocsExportedNames(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		declared, err := parseFuncDeclNames(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		for fn := range declared {
			if !ast.IsExported(fn) || fn == explanationDocsExcludedDocumentPath {
				continue
			}
			names[fn] = true
		}
	}
	return names, nil
}

// TestExplanationDocsExportedNames_ExcludedDocumentPathStillExists is a
// self-check on explanationDocsExcludedDocumentPath: it scans the WHOLE
// package (the same scope explanationDocsExportedNames itself uses, not one
// arbitrarily chosen file) for the excluded name declared as a free
// function. If it ever moves or is renamed, this fails directly — a stale
// exclusion for a name that no longer exists is dead documentation, not a
// real narrowing.
func TestExplanationDocsExportedNames_ExcludedDocumentPathStillExists(t *testing.T) {
	entries, err := os.ReadDir("../explanationdocs")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		declared, err := parseFuncDeclNames(filepath.Join("../explanationdocs", name))
		if err != nil {
			t.Fatal(err)
		}
		if declared[explanationDocsExcludedDocumentPath] {
			return
		}
	}
	t.Fatalf("excluded name %q is not declared anywhere in explanationdocs — stale exclusion, remove it", explanationDocsExcludedDocumentPath)
}

// TestExplanationHandoffVocabulary_IncludesKnownLifecycleFunctions pins a
// FLOOR on build_explanation_handoff.go's own declarations, independent of
// which files currently call them. A future split of that file (the #549
// shape, one level up) that drops one of these names from the file — without
// updating this floor — fails here, rather than passing silently because
// some OTHER file happens to also be anchored via a different vocabulary
// term (see architecture review 2026-09-12, HIGH: file-level anchoring alone
// cannot detect this).
func TestExplanationHandoffVocabulary_IncludesKnownLifecycleFunctions(t *testing.T) {
	names, err := parseFuncDeclNames("../core/build_explanation_handoff.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"projectBuildExplanation",
		"activateBuildExplanationContract",
		"sealBuildExplanationContext",
		"requireResumeExplanationIdentity",
		"explanationBinding",
		"authoritativeResumeIdentity",
	} {
		if !names[want] {
			t.Errorf("build_explanation_handoff.go no longer declares %s — if it moved, add the new file to explanationLifecycleVocabulary's derivation and update this floor", want)
		}
	}
}

// TestExplanationDocsVocabulary_IncludesKnownVerificationFunctions is the
// same floor for the explanationdocs package: a future move of one of these
// exported functions out of the package (or a rename) must fail here
// directly, not rely on some caller file happening to also be reachable via
// a different vocabulary term.
func TestExplanationDocsVocabulary_IncludesKnownVerificationFunctions(t *testing.T) {
	names, err := explanationDocsExportedNames("../explanationdocs")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Verify", "VerifyLanded", "CheckBuild", "RefreshResult",
		"RecoverRebaseSplit", "SealBuild", "SealResult",
		"Activate", "RequireActivation",
		"CrossCheckActivation", "ValidateReviewedHandoff",
		"ArchiveUnpublishedContinuationRecords",
	} {
		if !names[want] {
			t.Errorf("explanationdocs no longer exports %s — if it moved, add the new package/file to explanationLifecycleVocabulary's derivation and update this floor", want)
		}
	}
}

// explanationLifecycleImportAliases reports every file under root that
// imports "…/explanationdocs" under an alias, or dot-imports it. Either form
// defeats the qualified-name matching explanationCallSiteFiles and
// expressionName rely on (a call through an alias resolves to "alias.Fn", a
// dot-import resolves to bare "Fn") — this is a documented precondition
// (architecture review 2026-09-12, MEDIUM); this function enforces it rather
// than leaving it merely asserted in a comment. It skips _test.go files, like
// explanationCallSiteFiles does, since a fixture file's own import style has
// no bearing on whether it can defeat that scanner (which never reads
// _test.go files in the first place).
func explanationLifecycleImportAliases(root string) ([]string, error) {
	var offenders []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil // malformed file: a build/vet concern, not an alias-detection one
		}
		for _, imp := range file.Imports {
			// Path.Value is the raw source token, quoted per Go's own lexical
			// rules (interpreted "..." or raw `...`); Unquote is the correct
			// way to recover the literal value for either form, rather than
			// trimming one specific quote character. Realistically
			// unreachable — imp.Path.Value came from an AST that already
			// parsed successfully — kept as defensive code, not a signal to
			// investigate.
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if !strings.HasSuffix(importPath, "/explanationdocs") {
				continue
			}
			if imp.Name == nil {
				continue // ordinary "explanationdocs.Fn" — matched as-is
			}
			if imp.Name.Name == "_" {
				continue // blank import: no call sites to obscure
			}
			// path is always root joined with a WalkDir-yielded descendant,
			// so it always has root as a relatable prefix — Rel cannot fail
			// here (same reasoning as explanationCallSiteFiles below).
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(offenders)
	return offenders, err
}

// TestExplanationLifecycleImportAliases_NoneExist enforces the alias/dot-import
// precondition explanationCallSiteFiles depends on. It is not a floor with
// headroom — ANY hit here is a real blind spot introduced the moment it
// appears, so the assertion is exact: zero offenders.
func TestExplanationLifecycleImportAliases_NoneExist(t *testing.T) {
	offenders, err := explanationLifecycleImportAliases("..")
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("%d file(s) import explanationdocs under an alias or dot-import, which defeats the call-site scanner's qualified-name matching: %v", len(offenders), offenders)
	}
}

// TestExplanationLifecycleImportAliases_SyntheticAliasTrips is the detector's
// own self-proof: a genuine alias and a genuine dot-import must both be
// named; an ordinary import, a blank import, and an unrelated package whose
// import path merely ENDS similarly must not.
func TestExplanationLifecycleImportAliases_SyntheticAliasTrips(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "ok/plain.go", "package pkg\nimport \"x/explanationdocs\"\nvar _ = explanationdocs.Verify\n")
	writeGoFile(t, root, "ok/blank.go", "package pkg\nimport _ \"x/explanationdocs\"\n")
	writeGoFile(t, root, "ok/near_miss.go", "package pkg\nimport ed \"x/notexplanationdocs\"\nvar _ = ed.X\n")
	writeGoFile(t, root, "bad/alias.go", "package pkg\nimport ed \"x/explanationdocs\"\nvar _ = ed.Verify\n")
	writeGoFile(t, root, "bad/dot.go", "package pkg\nimport . \"x/explanationdocs\"\n")

	offenders, err := explanationLifecycleImportAliases(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bad/alias.go", "bad/dot.go"}; !slices.Equal(offenders, want) {
		t.Fatalf("offenders = %v, want %v", offenders, want)
	}
}

// explanationCallSiteFiles walks every non-test .go file under root and
// returns the slash-separated paths (relative to root) of every file
// containing a call, or a struct-literal field assigned by value (a call, a
// bare function, or a qualified function), whose resolved name is in vocab.
// It reuses expressionName (build_explanation_wiring_test.go), the same
// resolver the existing pin tables are checked against, so a qualified name
// like "explanationdocs.RefreshResult" is matched identically in both
// places, for both a call and a value position. vendor/ and testdata/
// subtrees are never a cycle's own dispatch code, so they are skipped; a
// file that fails to parse is skipped rather than aborting the whole scan,
// since a malformed file is a build/vet concern, not a protected-surface
// one; _test.go files are skipped because a fixture exercising a lifecycle
// function (e.g. build_explanation_state_test.go, which calls
// projectBuildExplanation directly) is not a dispatch call site and does not
// itself need protection.
//
// The struct-literal-value case matters because that assignment IS the call
// site if it is ever carved into its own file: the decision of which
// function runs lives at the assignment, not at the point something later
// invokes the resulting field (architecture review 2026-09-12, MEDIUM;
// example: audit.go's `CheckExplanation: verifyExplanationDocumentation`).
// expressionName resolves a value uniformly whether it is a bare identifier
// or a qualified selector, so a const/data-field reference that happens to
// share a vocabulary NAME (there are exactly two on this tree today —
// orchestrator.go's explanationdocs.CurrentContractVersion and cyclerun.go's
// o.explanationContractVersion, both already protected manifest rows for
// unrelated reasons) is treated no differently from a genuine function
// reference; verified empirically that this introduces no unprotected hit.
//
// Known limitation: this cannot resolve a call through an interface value or
// a method expression whose receiver type isn't syntactically named at the
// call site (e.g. `a.hooks.explanationCheck(a.req)` resolves to
// "a.hooks.explanationCheck", never to a vocabulary function name) — the
// WIRING site this function DOES catch is what matters, because that is
// where the decision of which function to run is actually made.
func explanationCallSiteFiles(root string, vocab map[string]bool) ([]string, error) {
	var hits []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		found := false
		ast.Inspect(file, func(node ast.Node) bool {
			if found {
				return false
			}
			switch n := node.(type) {
			case *ast.CallExpr:
				if vocab[expressionName(n.Fun)] {
					found = true
					return false
				}
			case *ast.KeyValueExpr:
				if vocab[expressionName(n.Value)] {
					found = true
					return false
				}
			}
			return true
		})
		if found {
			// path is always root joined with a WalkDir-yielded descendant, so
			// it always has root as a relatable prefix — Rel cannot fail here.
			rel, _ := filepath.Rel(root, path)
			hits = append(hits, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(hits)
	return hits, err
}

// TestExplanationCallSiteScanner_SyntheticCallSiteTrips is the scanner's own
// self-proof, independent of this repo's real files: on a synthetic fixture
// it must name exactly the files that call a vocabulary function or assign
// one by value (bare or qualified), and no others.
func TestExplanationCallSiteScanner_SyntheticCallSiteTrips(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "pkg/a.go", "package pkg\nfunc user() { projectBuildExplanation() }\n")
	writeGoFile(t, root, "pkg/b.go", "package pkg\nfunc other() { somethingElse() }\n")
	writeGoFile(t, root, "pkg/c.go", "package pkg\nvar cfg = struct{ Check func() }{Check: projectBuildExplanation}\n")
	writeGoFile(t, root, "pkg/d.go", "package pkg\nvar cfg = struct{ Check func() }{Check: other.Qualified}\n")

	hits, err := explanationCallSiteFiles(root, map[string]bool{"projectBuildExplanation": true, "other.Qualified": true})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"pkg/a.go", "pkg/c.go", "pkg/d.go"}; !slices.Equal(hits, want) {
		t.Fatalf("hits = %v, want %v", hits, want)
	}
}

// TestExplanationCallSiteScanner_SkipsNoiseAndSurvivesMalformedFiles proves
// the three defensive branches a synthetic-hit test alone doesn't reach: a
// vendor/ subtree is never descended into, a testdata/ subtree is never
// descended into, and a file that fails to parse is skipped rather than
// aborting the scan — all three alongside one real hit, so a walker that
// silently skipped EVERYTHING would also be caught.
func TestExplanationCallSiteScanner_SkipsNoiseAndSurvivesMalformedFiles(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, "pkg/real.go", "package pkg\nfunc user() { projectBuildExplanation() }\n")
	writeGoFile(t, root, "vendor/thirdparty/v.go", "package thirdparty\nfunc user() { projectBuildExplanation() }\n")
	writeGoFile(t, root, "pkg/testdata/fixture.go", "package testdata\nfunc user() { projectBuildExplanation() }\n")
	writeGoFile(t, root, "pkg/broken.go", "package pkg\nfunc unterminated( {\n")

	hits, err := explanationCallSiteFiles(root, map[string]bool{"projectBuildExplanation": true})
	if err != nil {
		t.Fatalf("a malformed file must be skipped, not fail the scan: %v", err)
	}
	if want := []string{"pkg/real.go"}; !slices.Equal(hits, want) {
		t.Fatalf("hits = %v, want %v (vendor/testdata/malformed must all be absent)", hits, want)
	}
}

// TestExplanationCallSiteScanner_BadRootErrors: a root that does not exist is
// a caller mistake, not "found nothing" — the scanner must say so.
func TestExplanationCallSiteScanner_BadRootErrors(t *testing.T) {
	_, err := explanationCallSiteFiles(filepath.Join(t.TempDir(), "does-not-exist"), map[string]bool{"x": true})
	if err == nil {
		t.Fatal("want an error for a root that does not exist, got nil")
	}
}

func writeGoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestExplanationCallSiteScanner_SeesEveryPinnedFile is an anti-vacuity check:
// every file any of the four pin sources already names must appear in the
// scanner's own output, so a broken walker (wrong root, wrong skip rule) or
// an accidentally-shrunk vocabulary cannot pass silently by finding nothing.
// All three explanationLifecycleAssignPins are anchored (not only the one
// whose value is a bare call): the scanner now resolves a KeyValueExpr value
// uniformly via expressionName, so orchestrator.go and cyclerun.go are
// genuine hits too, not merely protected for unrelated reasons.
func TestExplanationCallSiteScanner_SeesEveryPinnedFile(t *testing.T) {
	vocab := explanationLifecycleVocabulary(t)
	hits, err := explanationCallSiteFiles("..", vocab)
	if err != nil {
		t.Fatal(err)
	}
	hitSet := map[string]bool{}
	for _, h := range hits {
		hitSet[h] = true
	}

	var pinned []string
	for _, pin := range explanationLifecycleCallPins {
		pinned = append(pinned, pin.path)
	}
	for _, pin := range explanationLifecycleAssignPins {
		pinned = append(pinned, pin.path)
	}
	for _, pin := range explanationActivationBeltPins {
		pinned = append(pinned, pin.path)
	}
	pinned = append(pinned, explanationReviewGateFiles...)

	for _, p := range pinned {
		rel := strings.TrimPrefix(p, "../")
		if !hitSet[rel] {
			t.Errorf("scanner missed already-pinned file %s (walker or vocabulary regressed)", rel)
		}
	}
}

// TestProtectedSurface_CoversEveryExplanationLifecycleCallSite is the durable
// tripwire: every file the scanner finds must be denied by IsProtectedSurface.
// It is the projection of explanationLifecycleVocabulary onto the manifest,
// so a future relocation the pin tables don't yet name is still caught by
// what it CALLS or ASSIGNS, not by whether someone remembered to list it —
// within the limits stated in the file comment above.
func TestProtectedSurface_CoversEveryExplanationLifecycleCallSite(t *testing.T) {
	vocab := explanationLifecycleVocabulary(t)
	hits, err := explanationCallSiteFiles("..", vocab)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("scanner found zero explanation-lifecycle call sites — vocabulary or walker is broken")
	}
	for _, rel := range hits {
		path := "go/internal/" + rel
		if !IsProtectedSurface(path) {
			t.Errorf("explanation-lifecycle call site %s is not a protected surface — add a manifest row via a manual `evolve ship --class manual`", path)
		}
	}
}
