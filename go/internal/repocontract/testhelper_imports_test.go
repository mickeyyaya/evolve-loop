package repocontract

import (
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

func testOnlyHelpers() []string {
	module := evolveLoopModule()
	return []string{module + "/internal/fakeclitest", module + "/internal/tmuxtest"}
}

func productionImporters(root string, forbidden []string) ([]string, error) {
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if slices.Contains(forbidden, imported) {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, filepath.ToSlash(rel)+" imports "+imported)
			}
		}
		return nil
	})
	return offenders, err
}

func TestTestOnlyHelpersAreNeverImportedByProductionCode(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	offenders, err := productionImporters(root, testOnlyHelpers())

	if err != nil {
		t.Fatal(err)
	}
	for _, o := range offenders {
		t.Errorf("%s: a test-only helper in production code is a trust-boundary hole (fakeclitest's init runs a script that sits beside the binary; tmuxtest starts and kills tmux servers)", o)
	}
}

func TestProductionImporters_FlagsOnlyNonTestFiles(t *testing.T) {
	root := t.TempDir()
	helper := testOnlyHelpers()[0]
	for rel, body := range map[string]string{
		"internal/prod/prod.go":             "package prod\n\nimport _ \"" + helper + "\"\n",
		"internal/prod/prod_test.go":        "package prod\n\nimport _ \"" + helper + "\"\n",
		"internal/prod/testdata/fixture.go": "package fixture\n\nimport _ \"" + helper + "\"\n",
		"internal/clean/clean.go":           "package clean\n\nimport _ \"fmt\"\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	offenders, err := productionImporters(root, testOnlyHelpers())

	if err != nil || !slices.Equal(offenders, []string{"internal/prod/prod.go imports " + helper}) {
		t.Fatalf("productionImporters = %v, %v; want only the production file", offenders, err)
	}
}
