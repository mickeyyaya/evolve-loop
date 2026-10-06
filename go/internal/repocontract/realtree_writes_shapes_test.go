package repocontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const climbingRoot = "func repoRoot() string { return filepath.Join(\"..\", \"..\", \"..\") }\n"

func flaggedWrites(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x_test.go", "package p\n\n"+src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, finding := range newTreeWriteScan([]*ast.File{file}).findings(fset) {
		calls = append(calls, finding[strings.LastIndex(finding, ": ")+2:])
	}
	return calls
}

func TestRealTreeWrites_FlagsEveryShapeThatReachesTheRealTree(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"the phasespec decoy under a climbing root", climbingRoot + `func TestX(t *testing.T) {
	dir := filepath.Join(repoRoot(), ".evolve", "phases", "zz")
	os.MkdirAll(dir, 0o755)
	t.Cleanup(func() { os.RemoveAll(dir) })
	os.WriteFile(filepath.Join(dir, "phase.json"), nil, 0o644)
}`, []string{"os.MkdirAll", "os.RemoveAll", "os.WriteFile"}},
		{"a root found from this file's path", `func TestX(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	os.WriteFile(filepath.Join(filepath.Dir(file), "x"), nil, 0o644)
}`, []string{"os.WriteFile"}},
		{"the working directory", `func TestX(t *testing.T) {
	wd, _ := os.Getwd()
	os.Create(filepath.Join(wd, "out"))
}`, []string{"os.Create"}},
		{"git's top level, renamed into", `func TestX(t *testing.T) {
	out, _ := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	root := strings.TrimSpace(string(out))
	os.Rename(filepath.Join(t.TempDir(), "a"), filepath.Join(root, "b"))
}`, []string{"os.Rename"}},
		{"a helper that writes the root it is given", `func mustRoot() string { wd, _ := os.Getwd(); return filepath.Dir(wd) }
func capture(t *testing.T, root, label string) { os.MkdirAll(filepath.Join(root, "testdata", label), 0o755) }
func TestX(t *testing.T) { capture(t, mustRoot(), "x") }`, []string{"capture"}},
		{"a root threaded through a join helper", climbingRoot + `func under(root, rel string) string { return filepath.Join(root, rel) }
func TestX(t *testing.T) { os.WriteFile(under(repoRoot(), "x"), nil, 0o644) }`, []string{"os.WriteFile"}},
		{"a package-level root", `var root = filepath.Join("..", "..")
func TestX(t *testing.T) { os.Remove(filepath.Join(root, "x")) }`, []string{"os.Remove"}},
		{"a file opened for writing", climbingRoot + `func TestX(t *testing.T) { os.OpenFile(filepath.Join(repoRoot(), "x"), os.O_CREATE|os.O_WRONLY, 0o644) }`, []string{"os.OpenFile"}},
		{"a method on a fixture type", climbingRoot + `type fx struct{}
func (fx) plant() { os.WriteFile(filepath.Join(repoRoot(), "x"), nil, 0o644) }`, []string{"os.WriteFile"}},
		{"a variadic helper ranging over its paths", climbingRoot + `func touch(paths ...string) { for _, p := range paths { os.WriteFile(p, nil, 0o644) } }
func TestX(t *testing.T) { touch(t.TempDir(), filepath.Join(repoRoot(), "x")) }`, []string{"touch"}},
		{"a list of real paths", climbingRoot + `func TestX(t *testing.T) {
	dirs := []string{filepath.Join(repoRoot(), "a")}
	os.MkdirAll(dirs[0], 0o755)
}`, []string{"os.MkdirAll"}},
		{"a writer two calls deep, declared after its caller", climbingRoot + `func TestX(t *testing.T) { outer(repoRoot()) }
func outer(r string) { inner(r) }
func inner(r string) { os.WriteFile(r, nil, 0o644) }`, []string{"outer"}},
		{"the working directory read before it moves", `func TestX(t *testing.T) {
	wd, _ := os.Getwd()
	os.Chdir(t.TempDir())
	os.WriteFile(filepath.Join(wd, "x"), nil, 0o644)
}`, []string{"os.WriteFile"}},
		{"the working directory after a move into the real tree", `func TestX(t *testing.T) {
	os.Chdir(filepath.Join("..", ".."))
	wd, _ := os.Getwd()
	os.WriteFile(filepath.Join(wd, "x"), nil, 0o644)
}`, []string{"os.WriteFile"}},
		{"a package-level root used outside the subtest that shadows it", `var root = filepath.Join("..", "..")
func TestX(t *testing.T) {
	t.Run("a", func(t *testing.T) {
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "inside"), nil, 0o644)
	})
	os.WriteFile(filepath.Join(root, "outside"), nil, 0o644)
}`, []string{"os.WriteFile"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flaggedWrites(t, tc.src); !slices.Equal(got, tc.want) {
				t.Errorf("flagged %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRealTreeWrites_FlagsEachSinkAtItsPathArgumentOnly(t *testing.T) {
	sinks := []struct {
		call    string
		at      int
		flagged bool
	}{
		{"os.WriteFile", 0, true}, {"os.MkdirAll", 0, true}, {"os.Mkdir", 0, true}, {"os.Create", 0, true},
		{"os.Rename", 0, true}, {"os.Rename", 1, true}, {"os.Remove", 0, true}, {"os.RemoveAll", 0, true},
		{"os.Symlink", 1, true}, {"os.Link", 1, true}, {"os.Chmod", 0, true}, {"os.Chtimes", 0, true},
		{"os.Truncate", 0, true}, {"os.CopyFS", 0, true}, {"os.MkdirTemp", 0, true}, {"os.CreateTemp", 0, true},
		{"ioutil.WriteFile", 0, true}, {"ioutil.TempDir", 0, true}, {"ioutil.TempFile", 0, true},
		{"os.Symlink", 0, false}, {"os.Link", 0, false}, {"os.WriteFile", 1, false},
	}
	for _, sink := range sinks {
		t.Run(fmt.Sprintf("%s argument %d", sink.call, sink.at), func(t *testing.T) {
			args := []string{`"a"`, `"b"`, `"c"`}
			args[sink.at] = `filepath.Join(repoRoot(), "x")`
			got := flaggedWrites(t, climbingRoot+"func TestX(t *testing.T) { "+sink.call+"("+strings.Join(args, ", ")+") }")
			if want := sink.flagged; slices.Equal(got, []string{sink.call}) != want || (!want && len(got) != 0) {
				t.Errorf("flagged %v, want flagged=%v", got, want)
			}
		})
	}
}

func TestRealTreeWrites_SparesReadsTempDirsAndFixtures(t *testing.T) {
	cases := []struct{ name, src string }{
		{"reads under the real root", climbingRoot + `func TestX(t *testing.T) {
	os.ReadFile(filepath.Join(repoRoot(), "x"))
	os.ReadDir(repoRoot())
	os.Stat(repoRoot())
}`},
		{"writes under t.TempDir()", `func TestX(t *testing.T) { os.WriteFile(filepath.Join(t.TempDir(), "x"), nil, 0o644) }`},
		{"a read-only open of the real root", climbingRoot + `func TestX(t *testing.T) { os.OpenFile(filepath.Join(repoRoot(), "x"), os.O_RDONLY, 0) }`},
		{"restoring the working directory", `func TestX(t *testing.T) {
	wd, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(wd)
}`},
		{"real files mirrored into a gittest fixture", climbingRoot + `func TestX(t *testing.T) {
	repo := gittest.Fixture(t)
	spec, _ := os.ReadFile(filepath.Join(repoRoot(), "phase.json"))
	os.MkdirAll(filepath.Join(repo.Dir, ".evolve"), 0o755)
	os.WriteFile(filepath.Join(repo.Dir, ".evolve", "phase.json"), spec, 0o644)
}`},
		{"a writing helper given a temp dir", `func capture(root string) { os.MkdirAll(filepath.Join(root, "x"), 0o755) }
func TestX(t *testing.T) { capture(t.TempDir()) }`},
		{"a fixture's own top level", `func TestX(t *testing.T) {
	repo := gittest.Fixture(t)
	out, _ := exec.Command("git", "-C", repo.Dir, "rev-parse", "--show-toplevel").Output()
	os.WriteFile(filepath.Join(strings.TrimSpace(string(out)), "x"), nil, 0o644)
}`},
		{"a path made relative to the real root, joined under a temp dir", climbingRoot + `func TestX(t *testing.T) {
	rel, _ := filepath.Rel(repoRoot(), "/abs/x")
	os.WriteFile(filepath.Join(t.TempDir(), rel), nil, 0o644)
}`},
		{"a temp dir's parent", `func TestX(t *testing.T) { os.MkdirAll(filepath.Join(t.TempDir(), "..", "sibling"), 0o755) }`},
		{"a package-level root shadowed by a parameter", `var root = filepath.Join("..", "..")
func write(root string) { os.WriteFile(filepath.Join(root, "x"), nil, 0o644) }
func TestX(t *testing.T) { write(t.TempDir()) }`},
		{"a package-level root shadowed by a local temp dir", `var root = filepath.Join("..", "..")
func TestX(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "x"), nil, 0o644)
}`},
		{"the working directory after a move to a temp dir", `func TestX(t *testing.T) {
	os.Chdir(t.TempDir())
	wd, _ := os.Getwd()
	os.WriteFile(filepath.Join(wd, "x"), nil, 0o644)
}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flaggedWrites(t, tc.src); len(got) != 0 {
				t.Errorf("flagged %v, want nothing", got)
			}
		})
	}
}

func TestParseTestPackages_KeepsAnExternalTestPackageApartFromItsPackage(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"p/a_test.go": "package p\n\nfunc repoRoot() string { return filepath.Join(\"..\", \"..\") }\n",
		"p/b_test.go": "package p_test\n\nfunc repoRoot(dir string) string { return dir }\n\nfunc TestX(t *testing.T) { os.WriteFile(repoRoot(t.TempDir()), nil, 0o644) }\n",
	}
	for rel, src := range files {
		if err := os.MkdirAll(filepath.Join(root, "p"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fset, byPackage, err := parseTestPackages(root, slices.Sorted(maps.Keys(files)))
	if err != nil {
		t.Fatal(err)
	}
	if !newTreeWriteScan(byPackage[packageKey("p", "p")]).returnsTheRealTree("repoRoot") {
		t.Error("package p's repoRoot climbs to the real tree")
	}
	if got := newTreeWriteScan(byPackage[packageKey("p", "p_test")]).findings(fset); len(got) != 0 {
		t.Errorf("p_test's own repoRoot returns its argument, a temp dir; the scan used p's repoRoot instead and flagged %v", got)
	}
}
