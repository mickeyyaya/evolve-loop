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

// explanationLifecycleVocabulary derives the explanation-lifecycle API names from beliefs that already
// exist: the pin tables and the declarations of build_explanation_handoff.go and explanationdocs.
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
	// Gate entry points count too: audit.go wires verifyExplanationDocumentation as a struct-literal value.
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

// parseFuncDeclNames returns path's top-level free function names. Methods are left out: one is never
// called by its bare name, so a bare-name entry could only collide with a same-named free function.
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

// explanationDocsExcludedDocumentPath stays out of the vocabulary: it only formats a prompt-hint path,
// and its one caller, phases/build, is deliberately not a protected surface.
const explanationDocsExcludedDocumentPath = "DocumentPath"

// explanationDocsExportedNames returns the exported free functions of dir's non-test files, minus the excluded name.
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

// explanationLifecycleImportAliases lists the non-test files under root that alias or dot-import
// explanationdocs; either form hides a call site from the scanner's qualified-name match.
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
			// Rel cannot fail: path is root joined with a WalkDir descendant.
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(offenders)
	return offenders, err
}

func TestExplanationLifecycleImportAliases_NoneExist(t *testing.T) {
	offenders, err := explanationLifecycleImportAliases("..")
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("%d file(s) import explanationdocs under an alias or dot-import, which defeats the call-site scanner's qualified-name matching: %v", len(offenders), offenders)
	}
}

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

// explanationCallSiteFiles returns the root-relative paths of non-test files under root that call a vocab
// name or assign one as a struct-literal value, since the assignment is where the wiring decision lives.
// It resolves names with expressionName, like the pin tables, and skips vendor/, testdata/ and unparsable files.
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
			// Rel cannot fail: path is root joined with a WalkDir descendant.
			rel, _ := filepath.Rel(root, path)
			hits = append(hits, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(hits)
	return hits, err
}

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

// The one real hit keeps a walker that skips everything from passing.
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
