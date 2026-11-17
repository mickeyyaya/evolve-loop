package scopedset

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

const fixtureModule = "example.com/m"

func fixtureFiles() map[string]string {
	return map[string]string{
		"go/go.mod":     "module example.com/m\n\ngo 1.23\n",
		"go/Makefile":   "build:\n\t@true\n",
		"go/a/a.go":     "package a\n",
		"go/b/b.go":     "package b\n\nimport _ \"example.com/m/a\"\n",
		"go/d/d.go":     "package d\n",
		"go/e/e.go":     "package e\n\nimport (\n\t_ \"example.com/m/a\"\n\t_ \"example.com/m/d\"\n)\n",
		"go/f/f.go":     "package f\n\nimport _ \"example.com/m/b\"\n",
		"go/g/g.go":     "package g\n",
		"docs/spec.md":  "spec\n",
		"docs/other.md": "other\n",
	}
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScopedSet_IsTheIntersectionOfBothReverseClosures(t *testing.T) {
	t.Parallel()
	root := writeTree(t, fixtureFiles())
	cases := []struct {
		name       string
		lane, main []string
		want       []string
	}{
		{"a lane change and a main change meet only in the package that imports both", []string{"go/a/a.go"}, []string{"go/d/d.go"}, []string{"./e/..."}},
		{"the same package changed on both sides selects its whole reverse closure", []string{"go/a/a.go"}, []string{"go/a/a.go"}, []string{"./a/...", "./b/...", "./e/...", "./f/..."}},
		{"changes with disjoint reverse closures select nothing", []string{"go/g/g.go"}, []string{"go/d/d.go"}, nil},
		{"a lane that changed nothing selects nothing", nil, []string{"go/d/d.go"}, nil},
	}
	for _, c := range cases {
		got := Compute(root, c.lane, c.main)
		if got.All {
			t.Errorf("%s: All=true (%s), want a scoped set", c.name, got.Reason)
		}
		if !reflect.DeepEqual(got.Packages, c.want) {
			t.Errorf("%s: Packages=%v, want %v", c.name, got.Packages, c.want)
		}
	}
}

func TestScopedSet_AGlobalInputSelectsAll(t *testing.T) {
	t.Parallel()
	root := writeTree(t, fixtureFiles())
	cases := []struct {
		name       string
		root       string
		lane, main []string
	}{
		{"go.mod on the lane", root, []string{"go/go.mod"}, []string{"go/d/d.go"}},
		{"go.sum on main", root, []string{"go/a/a.go"}, []string{"go/go.sum"}},
		{"a vendored file on the lane", root, []string{"go/vendor/x/x.go"}, []string{"go/d/d.go"}},
		{"the module Makefile on main", root, []string{"go/a/a.go"}, []string{"go/Makefile"}},
		{"an import graph that cannot be read", "", []string{"go/a/a.go"}, []string{"go/d/d.go"}},
	}
	for _, c := range cases {
		got := Compute(c.root, c.lane, c.main)
		if !got.All || got.Reason == "" {
			t.Errorf("%s: All=%v Reason=%q, want All with a reason", c.name, got.All, got.Reason)
		}
	}
	docsOnly := Compute(root, []string{"docs/other.md"}, []string{"docs/other.md"})
	if docsOnly.All || len(docsOnly.Packages) != 0 {
		t.Errorf("a change outside the Go module selected All=%v Packages=%v, want an empty scoped set", docsOnly.All, docsOnly.Packages)
	}
}

func TestScopedSet_ATreeReadingTestJoinsWhenItsInputChanged(t *testing.T) {
	t.Parallel()
	files := fixtureFiles()
	files["go/reader/reader.go"] = "package reader\n"
	files["go/reader/reader_test.go"] = "package reader\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestSpec(t *testing.T) {\n\tif _, err := os.ReadFile(\"../../docs/spec.md\"); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	root := writeTree(t, files)
	cases := []struct {
		name       string
		lane, main []string
		want       []string
	}{
		{"the file the test reads changed on main", []string{"go/a/a.go"}, []string{"go/d/d.go", "docs/spec.md"}, []string{"./e/...", "./reader/..."}},
		{"the file the test reads changed on the lane", []string{"go/a/a.go", "docs/spec.md"}, []string{"go/d/d.go"}, []string{"./e/...", "./reader/..."}},
		{"a file no test reads changed", []string{"go/a/a.go"}, []string{"go/d/d.go", "docs/other.md"}, []string{"./e/..."}},
	}
	for _, c := range cases {
		got := Compute(root, c.lane, c.main)
		if got.All || !reflect.DeepEqual(got.Packages, c.want) {
			t.Errorf("%s: All=%v Packages=%v, want %v", c.name, got.All, got.Packages, c.want)
		}
	}
}

func TestShadow_AFailureOutsideTheSetIsAnEscape(t *testing.T) {
	t.Parallel()
	root := writeTree(t, fixtureFiles())
	scoped := Compute(root, []string{"go/a/a.go"}, []string{"go/d/d.go"})
	inside := baseattr.Key{Package: fixtureModule + "/e", Test: "TestE"}
	outside := baseattr.Key{Package: fixtureModule + "/b", Test: "TestB"}

	if got, want := Escapes(scoped, "test", baseattr.Reds{Keys: []baseattr.Key{inside, outside}}),
		[]Escape{{Gate: "test", Package: outside.Package, Test: outside.Test}}; !reflect.DeepEqual(got, want) {
		t.Errorf("a red outside the set: Escapes=%+v, want %+v", got, want)
	}
	if got := Escapes(scoped, "test", baseattr.Reds{Keys: []baseattr.Key{inside}}); len(got) != 0 {
		t.Errorf("a red inside the set is no escape, got %+v", got)
	}
	if got := Escapes(Set{All: true, Reason: "go.mod changed"}, "test", baseattr.Reds{Keys: []baseattr.Key{outside}}); len(got) != 0 {
		t.Errorf("the full set holds every package, so nothing escapes it, got %+v", got)
	}
	if got, want := Escapes(scoped, "compile", baseattr.Reds{Unnamed: true}),
		[]Escape{{Gate: "compile"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("a red that names no package cannot be proven inside the set: Escapes=%+v, want %+v", got, want)
	}
}

func TestScopedSet_AnUnreadableModuleSelectsAll(t *testing.T) {
	t.Parallel()
	noMod := fixtureFiles()
	delete(noMod, "go/go.mod")
	noModule := fixtureFiles()
	noModule["go/go.mod"] = "go 1.23\n\nmodule\n"
	brokenMod := fixtureFiles()
	brokenMod["go/go.mod"] = "module example.com/m\n\nbogus directive\n"
	cases := []struct {
		name       string
		files      map[string]string
		lane, main []string
	}{
		{"no go.mod", noMod, []string{"go/a/a.go"}, []string{"go/d/d.go"}},
		{"a go.mod without a module line", noModule, []string{"go/a/a.go"}, []string{"go/d/d.go"}},
		{"a graph that fails on the lane", brokenMod, []string{"go/a/a.go"}, []string{"go/d/d.go"}},
		{"a graph that fails on main", brokenMod, nil, []string{"go/d/d.go"}},
	}
	for _, c := range cases {
		got := Compute(writeTree(t, c.files), c.lane, c.main)
		if !got.All || got.Reason == "" {
			t.Errorf("%s: All=%v Reason=%q, want All with a reason", c.name, got.All, got.Reason)
		}
	}
}

func TestShadow_APackageOutsideTheModuleIsAnEscape(t *testing.T) {
	t.Parallel()
	root := writeTree(t, fixtureFiles())
	scoped := Compute(root, []string{"go/a/a.go"}, []string{"go/d/d.go"})
	foreign := baseattr.Key{Package: "other.org/x/e", Test: "TestX"}
	if got := Escapes(scoped, "test", baseattr.Reds{Keys: []baseattr.Key{foreign}}); len(got) != 1 {
		t.Errorf("a package of another module cannot be inside the set, got %+v", got)
	}
}

func TestScopedSet_OverlapKeepsTheNarrowerPatternOfEitherSide(t *testing.T) {
	t.Parallel()
	if got, want := overlap([]string{"./a/..."}, []string{"./a/b/..."}), []string{"./a/b/..."}; !reflect.DeepEqual(got, want) {
		t.Errorf("outer lane: overlap=%v, want %v", got, want)
	}
	if got, want := overlap([]string{"./a/b/..."}, []string{"./a/..."}), []string{"./a/b/..."}; !reflect.DeepEqual(got, want) {
		t.Errorf("outer main: overlap=%v, want %v", got, want)
	}
	if got := overlap([]string{"./a/..."}, []string{"./c/..."}); len(got) != 0 {
		t.Errorf("disjoint patterns overlap=%v, want none", got)
	}
}

func TestScopedSet_TheEnforceListIsNotAPackageChange(t *testing.T) {
	t.Parallel()
	root := writeTree(t, fixtureFiles())
	got, ok := closureOf(root, []string{enforceListPath, "go/a/a.go"})
	if !ok || !reflect.DeepEqual(got, []string{"./a/...", "./b/...", "./e/...", "./f/..."}) {
		t.Errorf("closureOf = (%v, %v), want the closure of a alone", got, ok)
	}
	if set := Compute(root, []string{enforceListPath}, []string{enforceListPath}); set.All || len(set.Packages) != 0 {
		t.Errorf("the enforce list alone selected All=%v Packages=%v", set.All, set.Packages)
	}
}

func TestScopedSet_TreeReadersSkipsUnreadableAndIrrelevantTests(t *testing.T) {
	t.Parallel()
	files := fixtureFiles()
	files["go/bad/bad_test.go"] = "package bad\n\nfunc (\n"
	files["go/vendor/v/v_test.go"] = "package v\n\nvar _ = \"../../../docs/spec.md\"\n"
	files["go/testdata/t_test.go"] = "package t\n\nvar _ = \"../../docs/spec.md\"\n"
	files["go/.hidden/h_test.go"] = "package h\n\nvar _ = \"../../docs/spec.md\"\n"
	files["go/num/num_test.go"] = "package num\n\nvar _ = 7\nvar _ = \"plain\"\nvar _ = \"../../docs/other.md\"\n"
	files["go/whole/whole_test.go"] = "package whole\n\nvar _ = \"../..\"\n"
	root := writeTree(t, files)
	if got := treeReaders(root, nil); got != nil {
		t.Errorf("no changed file: treeReaders=%v, want nil", got)
	}
	if got := treeReaders(filepath.Join(root, "missing"), []string{"docs/spec.md"}); len(got) != 0 {
		t.Errorf("an absent tree: treeReaders=%v, want none", got)
	}
	got := treeReaders(root, []string{"docs/spec.md"})
	if want := []string{"./whole/..."}; !reflect.DeepEqual(got, want) {
		t.Errorf("treeReaders=%v, want %v", got, want)
	}
}
