package repocontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func fixtureRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	tree := fstest.MapFS{}
	for name, body := range files {
		tree[name] = &fstest.MapFile{Data: []byte(body)}
	}
	root := t.TempDir()
	if err := os.CopyFS(root, tree); err != nil {
		t.Fatal(err)
	}
	return root
}

func declaredTests(pkgDir string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no _test.go file in %s", pkgDir)
	}
	declared := map[string]bool{}
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
				declared[fn.Name.Name] = true
			}
		}
	}
	return declared, nil
}

func missingTestProblems(moduleRoot string, rows []DataFileRead) []string {
	var problems []string
	for _, row := range rows {
		declared, err := declaredTests(filepath.Join(moduleRoot, filepath.FromSlash(strings.TrimPrefix(row.Package, "./"))))
		if err != nil {
			problems = append(problems, fmt.Sprintf("row %q names package %s that has no tests to read: %v", row.Path, row.Package, err))
			continue
		}
		for _, test := range row.Tests {
			if !declared[test] {
				problems = append(problems, fmt.Sprintf("row %q names %s.%s, which the package does not declare; go test -run of an absent name passes with nothing run, so rename or remove the row", row.Path, row.Package, test))
			}
		}
	}
	return problems
}

func missingPathProblems(repoRoot string, rows []DataFileRead) []string {
	var problems []string
	for _, row := range rows {
		if row.Path == WholeRepository {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSuffix(row.Path, "/")))); err != nil {
			problems = append(problems, fmt.Sprintf("row %s %v has Path %q, which is absent from the tree; rename or remove the row", row.Package, row.Tests, row.Path))
		}
	}
	return problems
}

func rowFor(path, pkg string, tests ...string) DataFileRead {
	return DataFileRead{Path: path, TestSelection: TestSelection{Package: pkg, Tests: tests}}
}

func TestMissingTestProblems_FlagsAnAbsentTestAndAnAbsentPackage(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t, map[string]string{
		"internal/x/a_test.go":   "package x\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n\nfunc (s suite) TestMethod(t *testing.T) {}\n\nfunc helper() {}\n",
		"internal/bad/a_test.go": "package bad\nfunc (\n",
	})
	cases := []struct {
		name string
		row  DataFileRead
		want int
	}{
		{"a declared test", rowFor("docs/", "./internal/x", "TestA"), 0},
		{"an absent test", rowFor("docs/", "./internal/x", "TestA", "TestGone"), 1},
		{"a method is not a test function", rowFor("docs/", "./internal/x", "TestMethod"), 1},
		{"a package with no test file", rowFor("docs/", "./internal/none", "TestA"), 1},
	}
	for _, tc := range cases {
		if got := missingTestProblems(root, []DataFileRead{tc.row}); len(got) != tc.want {
			t.Errorf("%s: problems = %q, want %d", tc.name, got, tc.want)
		}
	}
	if got := missingTestProblems(root, []DataFileRead{rowFor("docs/", "./internal/bad", "TestA")}); len(got) != 1 {
		t.Errorf("an unparsable test file is one problem, got %q", got)
	}
}

func TestMissingPathProblems_FlagsAnAbsentPathAndExemptsTheWholeRepository(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t, map[string]string{"docs/a.md": "x\n", ".github/workflows/required.yml": "x\n"})
	rows := []DataFileRead{
		rowFor("docs/", "./internal/x", "TestA"),
		rowFor("docs/a.md", "./internal/x", "TestA"),
		rowFor(".github/workflows/required.yml", "./internal/x", "TestA"),
		rowFor(WholeRepository, "./internal/x", "TestA"),
		rowFor("docs/gone.md", "./internal/x", "TestA"),
		rowFor("gone/", "./internal/x", "TestA"),
	}
	got := missingPathProblems(root, rows)
	if len(got) != 2 || !strings.Contains(got[0], "docs/gone.md") || !strings.Contains(got[1], "gone/") {
		t.Errorf("only the two absent paths are problems, got %q", got)
	}
}

func TestRowPathProblem_FlagsAKindMismatchOnAFixture(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t, map[string]string{"docs/a.md": "x\n"})
	cases := map[string]bool{"docs/": false, "docs/a.md": false, WholeRepository: false, "docs": true, "docs/a.md/": true, "gone/": false}
	for path, want := range cases {
		if got := rowPathProblem(root, path) != ""; got != want {
			t.Errorf("rowPathProblem(%q) problem = %v, want %v", path, got, want)
		}
	}
}

func TestDataFileReads_EveryNamedTestExists(t *testing.T) {
	t.Parallel()
	for _, problem := range missingTestProblems(realModuleRoot(t), DataFileReads()) {
		t.Error(problem)
	}
}

func TestDataFileReads_EveryPathExists(t *testing.T) {
	t.Parallel()
	for _, problem := range missingPathProblems(filepath.Dir(realModuleRoot(t)), DataFileReads()) {
		t.Error(problem)
	}
}

func realModuleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
