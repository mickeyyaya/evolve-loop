//go:build integration

package acssuite

import (
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func writeInventoryModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module fixture\n\ngo 1.23\n"
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func inventoryEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOFLAGS=")
}

func stubGoOnPath(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	fakeclitest.Install(t, filepath.Join(dir, "go"), "#!/bin/sh\n"+script+"\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDeclaredPredicateTests_KeepsOnlyTopLevelExportedTestFunctions(t *testing.T) {
	root := writeInventoryModule(t, map[string]string{
		"acs/cycle7/in_test.go": "package cycle7\nimport (\"os\"; \"testing\")\n" +
			"type suite struct{}\n" +
			"func (suite) TestMethod(t *testing.T) {}\n" +
			"func TestMain(m *testing.M) { os.Exit(m.Run()) }\n" +
			"func Testlowercase(t *testing.T) {}\n" +
			"func Test(t *testing.T) {}\n" +
			"func TestÜnicode(t *testing.T) {}\n" +
			"func TestRequired(t *testing.T) {}\n" +
			"func helper() {}\n" +
			"var TestVariable = 1\n",
		"acs/cycle7/ext_test.go": "package cycle7_test\nimport \"testing\"\nfunc TestExternal(t *testing.T) {}\n",
		"acs/cycle7/doc.go":      "package cycle7\n",
	})

	got, err := declaredPredicateTests(context.Background(), root, "./acs/cycle7", inventoryEnv())

	if err != nil {
		t.Fatalf("declaredPredicateTests: %v", err)
	}
	want := []string{
		"fixture/acs/cycle7/Test",
		"fixture/acs/cycle7/TestExternal",
		"fixture/acs/cycle7/TestRequired",
		"fixture/acs/cycle7/TestÜnicode",
	}
	if keys := slices.Sorted(maps.Keys(got)); !slices.Equal(keys, want) {
		t.Fatalf("declared tests = %v, want %v", keys, want)
	}
}

func TestDeclaredPredicateTests_SpansEveryPackageOfARecursivePattern(t *testing.T) {
	root := writeInventoryModule(t, map[string]string{
		"acs/regression/a/a_test.go": "package a\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n",
		"acs/regression/b/b_test.go": "package b\nimport \"testing\"\nfunc TestB(t *testing.T) {}\n",
	})

	got, err := declaredPredicateTests(context.Background(), root, "./acs/regression/...", inventoryEnv())

	if err != nil {
		t.Fatalf("declaredPredicateTests: %v", err)
	}
	want := []string{"fixture/acs/regression/a/TestA", "fixture/acs/regression/b/TestB"}
	if keys := slices.Sorted(maps.Keys(got)); !slices.Equal(keys, want) {
		t.Fatalf("declared tests = %v, want %v", keys, want)
	}
}

func TestDeclaredPredicateTests_ScopeWithOnlyHarnessFunctionsIsAnError(t *testing.T) {
	root := writeInventoryModule(t, map[string]string{
		"acs/cycle7/in_test.go": "package cycle7\nimport (\"os\"; \"testing\")\nfunc TestMain(m *testing.M) { os.Exit(m.Run()) }\nfunc Testhelper(t *testing.T) {}\n",
	})

	got, err := declaredPredicateTests(context.Background(), root, "./acs/cycle7", inventoryEnv())

	if err == nil || !strings.Contains(err.Error(), "predicate scope ./acs/cycle7 has no declared tests") {
		t.Fatalf("err = %v, want the no-declared-tests error", err)
	}
	if got != nil {
		t.Fatalf("got = %v, want nil on error", got)
	}
}

func TestDeclaredPredicateTests_GoListFailureNamesThePattern(t *testing.T) {
	root := writeInventoryModule(t, map[string]string{})

	got, err := declaredPredicateTests(context.Background(), root, "./acs/missing", inventoryEnv())

	if err == nil || !strings.Contains(err.Error(), "discover compiled predicate inventory ./acs/missing") {
		t.Fatalf("err = %v, want the discover error naming the pattern", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want the wrapped go-list exit error", err)
	}
	if got != nil {
		t.Fatalf("got = %v, want nil on error", got)
	}
}

func TestDeclaredPredicateTests_CanceledContextNeverListsAnInventory(t *testing.T) {
	root := writeInventoryModule(t, map[string]string{
		"acs/cycle7/in_test.go": "package cycle7\nimport \"testing\"\nfunc TestRequired(t *testing.T) {}\n",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := declaredPredicateTests(ctx, root, "./acs/cycle7", inventoryEnv())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Fatalf("got = %v, want nil", got)
	}
}

func TestDeclaredPredicateTests_MalformedGoListStreamIsADecodeError(t *testing.T) {
	stubGoOnPath(t, `printf '{"ImportPath":"fixture/a","TestGoFiles":[]}{"ImportPath":'`)

	got, err := declaredPredicateTests(context.Background(), t.TempDir(), "./...", os.Environ())

	if err == nil || !strings.Contains(err.Error(), "decode predicate inventory") {
		t.Fatalf("err = %v, want the decode error", err)
	}
	if got != nil {
		t.Fatalf("got = %v, want nil", got)
	}
}

func TestDeclaredPredicateTests_UnparseableTestFileIsAParseError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken_test.go"), []byte("package broken\nfunc TestX(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubGoOnPath(t, `printf '{"Dir":"`+dir+`","ImportPath":"fixture/broken","TestGoFiles":["broken_test.go"]}'`)

	got, err := declaredPredicateTests(context.Background(), t.TempDir(), "./...", os.Environ())

	if err == nil || !strings.Contains(err.Error(), "parse predicate inventory") {
		t.Fatalf("err = %v, want the parse error", err)
	}
	if got != nil {
		t.Fatalf("got = %v, want nil", got)
	}
}

func TestDeclaredPredicateTests_MissingTestFileIsAParseError(t *testing.T) {
	dir := t.TempDir()
	stubGoOnPath(t, `printf '{"Dir":"`+dir+`","ImportPath":"fixture/gone","XTestGoFiles":["gone_test.go"]}'`)

	_, err := declaredPredicateTests(context.Background(), t.TempDir(), "./...", os.Environ())

	if err == nil || !strings.Contains(err.Error(), "parse predicate inventory") || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want a parse error wrapping ErrNotExist", err)
	}
}
