package goload_test

import (
	"context"
	"errors"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/goload"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func module(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func replying(stdout string, code int, err error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		_, _ = io.WriteString(out, stdout)
		return code, err
	}
}

var real = goload.Loader{Run: sysexec.DefaultRunner}

func TestLoad_TypeChecksAModuleAndOrdersDependenciesFirst(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{
		"go.mod":      "module m\n\ngo 1.23\n",
		"a/a.go":      "package a\n\nimport \"strings\"\n\nfunc A() int { return len(strings.ToUpper(\"a\")) }\n",
		"b/b.go":      "package b\n\nimport \"m/a\"\n\nfunc B() int { return a.A() }\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
	})

	var prog goload.Program
	prog, err := real.Load(context.Background(), goload.Request{Dir: dir, Patterns: []string{"./..."}, Env: os.Environ()})

	if err != nil {
		t.Fatal(err)
	}
	var a, b *goload.Package = prog.Packages["m/a"], prog.Packages["m/b"]
	if a == nil || b == nil || !a.Module || !b.Module || a.Standard || a.Types == nil || len(b.Files) != 1 || len(b.Info.Uses) == 0 {
		t.Fatalf("packages = %+v %+v", a, b)
	}
	if slices.Index(prog.Order, "m/a") > slices.Index(prog.Order, "m/b") || slices.Index(prog.Order, "m/a") < 0 {
		t.Errorf("order = %v, want m/a before m/b", prog.Order)
	}
	if _, ok := prog.Packages["m/b [m/b.test]"]; ok {
		t.Error("Load listed a test variant")
	}
	fmtPkg := prog.Packages["strings"]
	if fmtPkg == nil || !fmtPkg.Standard || fmtPkg.Module || fmtPkg.Files != nil || prog.Fset == nil {
		t.Errorf("strings = %+v", fmtPkg)
	}
}

func TestLoadWithTests_ListsTheTestVariants(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{
		"go.mod":      "module m\n\ngo 1.23\n",
		"b/b.go":      "package b\n\nfunc B() int { return 1 }\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() }\n",
	})

	prog, err := real.LoadWithTests(context.Background(), goload.Request{Dir: dir, Patterns: []string{"./..."}, Env: os.Environ()})

	if err != nil {
		t.Fatal(err)
	}
	if _, ok := prog.Packages["m/b [m/b.test]"]; !ok {
		t.Errorf("order = %v, want the test variant of m/b", prog.Order)
	}
	for key := range prog.Packages {
		if strings.HasSuffix(key, ".test") {
			t.Errorf("package %q is a generated test main", key)
		}
	}
}

func TestLoad_FakesTheCgoImport(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{"x.go": "package x\n\nimport \"C\"\n\nvar _ = C.int(0)\n"})
	answer := `{"ImportPath":"x","Dir":"` + dir + `","GoFiles":["x.go"],"Module":{"Main":true}}`

	prog, err := goload.Loader{Run: replying(answer, 0, nil)}.Load(context.Background(), goload.Request{Dir: dir})

	if err != nil || len(prog.Packages["x"].Errors) != 0 {
		t.Errorf("Load = %v, errors = %v", err, prog.Packages["x"].Errors)
	}
}

func TestLoad_ALoaderWithNoRunnerIsAnError(t *testing.T) {
	t.Parallel()

	_, err := goload.Loader{}.Load(context.Background(), goload.Request{Dir: t.TempDir()})

	if err == nil || !strings.Contains(err.Error(), "no runner") {
		t.Errorf("Load = %v, want an error naming the missing runner", err)
	}
}

func walkModule(t *testing.T) (string, goload.Request) {
	t.Helper()
	dir := module(t, map[string]string{
		"go.mod": "module m\n\ngo 1.23\n",
		"a/a.go": "package a\n\nfunc A() int { return 1 }\n",
		"b/b.go": "package b\n\nimport \"m/a\"\n\nfunc B() int { return a.A() }\n",
		"c/c.go": "package c\n\nimport \"m/b\"\n\nfunc C() int { return b.B() }\n",
	})
	return dir, goload.Request{Dir: dir, Patterns: []string{"./c"}, Env: os.Environ()}
}

func TestWalk_VisitsEachModulePackageInDependencyOrder(t *testing.T) {
	t.Parallel()
	_, req := walkModule(t)
	var seen []string

	fset, err := real.Walk(context.Background(), req, false, func(f *token.FileSet, p *goload.Package) error {
		if f == nil || p.Files == nil || p.Info == nil || !p.Module {
			t.Errorf("visit got %v %+v", f, p)
		}
		seen = append(seen, p.Path)
		return nil
	})

	if err != nil || fset == nil || !slices.Equal(seen, []string{"m/a", "m/b", "m/c"}) {
		t.Errorf("Walk = %v, %v; visited %v", fset, err, seen)
	}
}

func TestWalk_AVisitErrorStopsTheWalkAndIsReturned(t *testing.T) {
	t.Parallel()
	_, req := walkModule(t)
	boom := errors.New("boom")
	var seen []string

	fset, err := real.Walk(context.Background(), req, false, func(_ *token.FileSet, p *goload.Package) error {
		seen = append(seen, p.Path)
		return boom
	})

	if !errors.Is(err, boom) || fset != nil || !slices.Equal(seen, []string{"m/a"}) {
		t.Errorf("Walk = %v, %v; visited %v; want the visit error after one visit", fset, err, seen)
	}
}

func TestWalk_AListFailureIsReturnedBeforeAnyVisit(t *testing.T) {
	t.Parallel()

	_, err := goload.Loader{Run: replying("", 2, nil)}.Walk(context.Background(), goload.Request{Dir: t.TempDir()}, false, func(*token.FileSet, *goload.Package) error {
		t.Error("visited")
		return nil
	})

	if err == nil {
		t.Error("Walk = nil, want the list error")
	}
}

func TestLoad_ReportsATypeErrorAndAParseErrorOnTheModulePackage(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{
		"go.mod": "module m\n\ngo 1.23\n",
		"x/x.go": "package x\n\nvar V int = \"s\"\n",
		"x/y.go": "package x\n\nfunc (\n",
	})

	prog, err := real.Load(context.Background(), goload.Request{Dir: dir, Patterns: []string{"./x"}, Env: os.Environ()})

	if err != nil || len(prog.Packages["m/x"].Errors) < 2 {
		t.Errorf("Load = %v, errors = %v; want two errors on m/x", err, prog.Packages["m/x"].Errors)
	}
}

func TestLoad_FailsLoudly(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		run  sysexec.RunFunc
		want string
	}{
		"runner error":    {replying("", -1, errors.New("boom")), "boom"},
		"bad json":        {replying("{", 0, nil), "decode"},
		"exit code":       {replying("", 2, nil), "exit 2"},
		"unlisted import": {replying(`{"ImportPath":"x","Dir":"`+t.TempDir()+`","GoFiles":[],"Module":{"Main":true}}`+"\n"+`{"ImportPath":"y","Dir":"`+t.TempDir()+`","GoFiles":[],"Module":{"Main":true}}`, 0, nil), ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := goload.Loader{Run: tc.run}.Load(context.Background(), goload.Request{Dir: t.TempDir(), Patterns: []string{"./..."}})
			if tc.want == "" {
				if err != nil {
					t.Errorf("Load = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Load = %v, want an error naming %q", err, tc.want)
			}
		})
	}
}

func TestLoad_AnImportThatGoListDidNotListIsATypeError(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{"x.go": "package x\n\nimport _ \"missing\"\n"})
	answer := `{"ImportPath":"x","Dir":"` + dir + `","GoFiles":["x.go"],"Module":{"Main":true}}`

	prog, err := goload.Loader{Run: replying(answer, 0, nil)}.Load(context.Background(), goload.Request{Dir: dir})

	if err != nil || len(prog.Packages["x"].Errors) == 0 || !strings.Contains(prog.Packages["x"].Errors[0].Error(), "did not list the import missing") {
		t.Errorf("Load = %v, errors = %v", err, prog.Packages["x"].Errors)
	}
}

func TestLoad_ResolvesUnsafeAndTheImportMap(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{
		"x.go": "package x\n\nimport (\n\t\"unsafe\"\n\t\"alias\"\n)\n\nvar _ = unsafe.Sizeof(0)\nvar _ = real.V\n",
		"r.go": "package real\n\nvar V = 1\n",
	})
	answer := `{"ImportPath":"x","Dir":"` + dir + `","GoFiles":["x.go"],"ImportMap":{"alias":"real"},"Module":{"Main":true}}` + "\n" +
		`{"ImportPath":"real","Dir":"` + dir + `","GoFiles":["r.go"]}`

	prog, err := goload.Loader{Run: replying(answer, 0, nil)}.Load(context.Background(), goload.Request{Dir: dir})

	if err != nil || len(prog.Packages["x"].Errors) != 0 || prog.Packages["real"].Module {
		t.Errorf("Load = %v, errors = %v", err, prog.Packages["x"].Errors)
	}
}

func TestLoad_ACancelledContextIsAnError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	answer := `{"ImportPath":"x","Dir":"` + t.TempDir() + `","Module":{"Main":true}}`
	run := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		_, _ = io.WriteString(out, answer)
		cancel()
		return 0, nil
	}

	_, err := goload.Loader{Run: run}.Load(ctx, goload.Request{Dir: t.TempDir()})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Load = %v, want the context error", err)
	}
}

func TestLoad_PassesTheEnvironmentAndCgoOff(t *testing.T) {
	t.Parallel()
	var gotEnv, gotArgs []string
	run := func(_ context.Context, _, _ string, args, env []string, _ io.Reader, _, _ io.Writer) (int, error) {
		gotEnv, gotArgs = env, args
		return 0, nil
	}

	_, err := goload.Loader{Run: run}.Load(context.Background(), goload.Request{Dir: "d", Patterns: []string{"p"}, Env: []string{"A=1"}})

	if err != nil || !slices.Equal(gotEnv, []string{"A=1", "CGO_ENABLED=0"}) || !slices.Equal(gotArgs, []string{"list", "-buildvcs=false", "-json", "-deps", "p"}) {
		t.Errorf("env = %v, args = %v, err = %v", gotEnv, gotArgs, err)
	}
}

func TestWalk_AVisitErrorInsideAnImportStopsTheLaterImports(t *testing.T) {
	t.Parallel()
	dir := module(t, map[string]string{
		"b.go": "package b\n\nimport (\n\t_ \"a\"\n\t_ \"d\"\n)\n",
		"a.go": "package a\n",
		"d.go": "package d\n",
	})
	entry := func(path, file string, main bool) string {
		return `{"ImportPath":"` + path + `","Dir":"` + dir + `","GoFiles":["` + file + `"],"Module":{"Main":` + strconv.FormatBool(main) + `}}` + "\n"
	}
	answer := entry("b", "b.go", true) + entry("a", "a.go", true) + entry("d", "d.go", true)
	boom := errors.New("boom")
	var seen []string

	_, err := goload.Loader{Run: replying(answer, 0, nil)}.Walk(context.Background(), goload.Request{Dir: dir}, false, func(_ *token.FileSet, p *goload.Package) error {
		seen = append(seen, p.Path)
		return boom
	})

	if !errors.Is(err, boom) || !slices.Equal(seen, []string{"a"}) {
		t.Errorf("Walk = %v; visited %v; want boom after the visit of a", err, seen)
	}
}
