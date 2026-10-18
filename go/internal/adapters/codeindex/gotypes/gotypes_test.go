package gotypes_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/codeindextest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func fixtureProbes() []codeindextest.Probe {
	return []codeindextest.Probe{
		{
			Request:    codeindex.Request{Query: codeindex.QueryCallers, Arg: "a.Digest", Direction: codeindex.DirectionIn},
			WantSymbol: "c.Main",
			WantFile:   "go/c/c.go",
			Missing:    codeindex.Request{Query: codeindex.QueryCallers, Arg: "zzzNoSuchSym", Direction: codeindex.DirectionIn},
		},
		{
			Request:    codeindex.Request{Query: codeindex.QueryAPI, Arg: "go/a/a.go"},
			WantSymbol: "Engine.Run",
			Missing:    codeindex.Request{Query: codeindex.QueryAPI, Arg: "nosuch.go"},
		},
		{
			Request: codeindex.Request{Query: codeindex.QueryFind, Arg: "digest"},
			Missing: codeindex.Request{Query: codeindex.QueryFind, Arg: "qqqzzzxxx"},
		},
		{
			Request: codeindex.Request{Query: codeindex.QueryMap, Arg: "go/"},
			Missing: codeindex.Request{Query: codeindex.QueryMap, Arg: "nosuchdir/"},
		},
	}
}

func TestGotypesBackend_PassesTheConformanceSuite(t *testing.T) {
	t.Parallel()
	absent := filepath.Join(t.TempDir(), "index.gob")
	codeindextest.Run(t, codeindextest.Harness{
		New:       func() codeindex.Backend { return fixtureBackend() },
		NewAbsent: func() codeindex.Backend { return gotypes.New(absent) },
		Probes:    fixtureProbes(),
	})
}

func TestGotypesBackend_NamesItselfAndServesCallersAndAPIOnly(t *testing.T) {
	t.Parallel()
	b := fixtureBackend()

	if b.Name() != "gotypes" || b.Capabilities() != (codeindex.Capabilities{Callers: true, API: true}) {
		t.Errorf("Name = %q, Capabilities = %+v", b.Name(), b.Capabilities())
	}
}

func TestGotypesBackend_FindAndMapAreUnsupported(t *testing.T) {
	t.Parallel()
	b := fixtureBackend()

	_, findErr := b.Find(context.Background(), "digest")
	_, mapErr := b.Map(context.Background(), "go/")

	if !errors.Is(findErr, codeindex.ErrUnsupported) || !errors.Is(mapErr, codeindex.ErrUnsupported) {
		t.Errorf("Find = %v, Map = %v, want ErrUnsupported", findErr, mapErr)
	}
}

func TestGotypesBackend_AQualifiedNameGivesOnlyTheCallersOfItsOwnDefinition(t *testing.T) {
	t.Parallel()
	for symbol, want := range map[string][]place{
		"a.Digest": {
			{"go/a/a.go", 13, "a.Engine.Run", "a.Digest static"},
			{"go/c/c.go", 9, "c.Main", "a.Digest static"},
			{"go/c/c_test.go", 9, "c.helper", "a.Digest static"},
		},
		"b.Digest": {
			{"go/c/c.go", 9, "c.Main", "b.Digest static"},
			{"go/c/c.go", 12, "c.OnlyB", "b.Digest static"},
		},
	} {
		t.Run(symbol, func(t *testing.T) {
			t.Parallel()
			if got := places(callers(t, symbol, codeindex.DirectionIn)); !slices.Equal(got, want) {
				t.Errorf("callers of %s = %+v, want %+v", symbol, got, want)
			}
		})
	}
}

func TestGotypesBackend_ABareNameWithTwoDefinitionsNamesTheDefinitionOfEachHit(t *testing.T) {
	t.Parallel()

	hits := callers(t, "Digest", codeindex.DirectionIn)

	count := map[string]int{}
	for _, h := range hits {
		def, _, _ := strings.Cut(h.Text, " ")
		count[def]++
	}
	if len(hits) != 5 || count["a.Digest"] != 3 || count["b.Digest"] != 2 {
		t.Errorf("hits = %+v, want 3 named a.Digest and 2 named b.Digest", places(hits))
	}
}

func TestGotypesBackend_EachHitNamesItsKind(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		symbol string
		dir    codeindex.Direction
		want   []place
	}{
		"static callees": {"c.Main", codeindex.DirectionOut, []place{
			{"go/c/c.go", 9, "a.Digest", "c.Main static"},
			{"go/c/c.go", 9, "b.Digest", "c.Main static"},
			{"go/c/c.go", 9, "b.Drive", "c.Main static"},
			{"go/c/c.go", 9, "b.Apply", "c.Main static"},
		}},
		"interface callee":   {"b.Drive", codeindex.DirectionOut, []place{{"go/b/b.go", 7, "a.Runner.Run", "b.Drive interface"}}},
		"func-value callee":  {"b.Apply", codeindex.DirectionOut, []place{{"go/b/b.go", 9, "f", "b.Apply func-value"}}},
		"interface caller":   {"Runner.Run", codeindex.DirectionIn, []place{{"go/b/b.go", 7, "b.Drive", "a.Runner.Run interface"}}},
		"method of a type":   {"Engine.Run", codeindex.DirectionOut, []place{{"go/a/a.go", 13, "a.Digest", "a.Engine.Run static"}}},
		"universe interface": {"b.Describe", codeindex.DirectionOut, []place{{"go/b/b.go", 11, "builtin.error.Error", "b.Describe interface"}}},
		"unnamed interface":  {"b.Shut", codeindex.DirectionOut, []place{{"go/b/b.go", 13, "b.interface.Close", "b.Shut interface"}}},
		"package-level func": {"helper", codeindex.DirectionOut, []place{{"go/c/c_test.go", 9, "a.Digest", "c.helper static"}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := places(callers(t, tc.symbol, tc.dir)); !slices.Equal(got, tc.want) {
				t.Errorf("callers %s %s = %+v, want %+v", tc.dir, tc.symbol, got, tc.want)
			}
		})
	}
}

func TestGotypesBackend_AConcreteMethodReachedOnlyThroughAnInterfaceGetsItsImplementsLine(t *testing.T) {
	t.Parallel()
	want := []place{{"go/a/a.go", 13, "a.Runner.Run", "a.Engine.Run implements a.Runner.Run: 1 call(s) through it"}}

	for _, symbol := range []string{"Engine.Run", "a.Engine.Run"} {
		if got := places(callers(t, symbol, codeindex.DirectionIn)); !slices.Equal(got, want) {
			t.Errorf("callers of %s = %+v, want %+v", symbol, got, want)
		}
	}
}

func TestGotypesBackend_ADefinitionWithNoCallerIsNotFound(t *testing.T) {
	t.Parallel()

	_, err := fixtureBackend().Callers(context.Background(), "c.OnlyB", codeindex.DirectionIn)

	if !errors.Is(err, codeindex.ErrNotFound) || !strings.Contains(err.Error(), "c.OnlyB") {
		t.Errorf("err = %v, want ErrNotFound naming c.OnlyB", err)
	}
}

func TestGotypesBackend_AnUnknownSymbolIsNotFoundAndNamesTheSymbol(t *testing.T) {
	t.Parallel()

	_, err := fixtureBackend().Callers(context.Background(), "zzzNoSuchSym", codeindex.DirectionOut)

	if !errors.Is(err, codeindex.ErrNotFound) || !strings.Contains(err.Error(), "zzzNoSuchSym") {
		t.Errorf("err = %v, want ErrNotFound naming the symbol", err)
	}
}

func TestGotypesBackend_APIListsTheTopLevelDeclarationsOfTheFileInOrder(t *testing.T) {
	t.Parallel()
	want := []place{
		{"go/a/a.go", 3, "Version", "const"},
		{"go/a/a.go", 5, "Default", "var"},
		{"go/a/a.go", 7, "Runner", "type"},
		{"go/a/a.go", 11, "Engine", "type"},
		{"go/a/a.go", 13, "Engine.Run", "method"},
		{"go/a/a.go", 15, "Digest", "func"},
		{"go/a/a.go", 17, "Engine.Reset", "method"},
	}

	for _, file := range []string{"go/a/a.go", "./go/a/a.go"} {
		hits, err := fixtureBackend().API(context.Background(), file)
		if err != nil || !slices.Equal(places(hits), want) {
			t.Errorf("API(%q) = %+v, %v; want %+v", file, places(hits), err, want)
		}
	}
}

func TestGotypesBackend_APIOfATestFileIsServed(t *testing.T) {
	t.Parallel()

	hits, err := fixtureBackend().API(context.Background(), "go/c/c_test.go")

	want := []place{{"go/c/c_test.go", 9, "helper", "func"}, {"go/c/c_test.go", 11, "TestTotal", "func"}}
	if err != nil || !slices.Equal(places(hits), want) {
		t.Errorf("API = %+v, %v; want %+v", places(hits), err, want)
	}
}

func TestGotypesBackend_AnAbsentIndexIsErrAbsentAndNamesThePath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "index.gob")

	_, err := gotypes.New(path).Callers(context.Background(), "a.Digest", codeindex.DirectionIn)

	if !errors.Is(err, codeindex.ErrAbsent) || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want ErrAbsent naming %s", err, path)
	}
}

func TestGotypesBackend_AnIndexOfAnotherVersionIsErrAbsent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "index.gob")
	if err := gotypes.WriteIndexVersion(path, 999); err != nil {
		t.Fatal(err)
	}

	_, err := gotypes.New(path).API(context.Background(), "go/a/a.go")

	if !errors.Is(err, codeindex.ErrAbsent) || !strings.Contains(err.Error(), "999") {
		t.Errorf("err = %v, want ErrAbsent naming version 999", err)
	}
}

func TestGotypesBackend_ACorruptIndexIsAPlainErrorThatNamesThePath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "index.gob")
	if err := os.WriteFile(path, []byte("not a gob"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := gotypes.New(path).API(context.Background(), "go/a/a.go")

	if err == nil || errors.Is(err, codeindex.ErrAbsent) || errors.Is(err, codeindex.ErrNotFound) || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want a plain error naming %s", err, path)
	}
}

func TestGotypesBackend_AnUnreadableIndexIsAPlainErrorThatNamesThePath(t *testing.T) {
	t.Parallel()
	path := t.TempDir()

	_, err := gotypes.New(path).Callers(context.Background(), "a.Digest", codeindex.DirectionIn)

	if err == nil || errors.Is(err, codeindex.ErrAbsent) || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want a plain error naming %s", err, path)
	}
}

func TestGotypesBackend_GenericAndParenthesizedCallsResolveToTheirOriginAndBuiltinsConversionsAndLiteralsAreSkipped(t *testing.T) {
	t.Parallel()
	want := []place{
		{"go/b/extra.go", 22, "b.Resetter.Reset", "b.Misc interface"},
		{"go/b/extra.go", 23, "b.First", "b.Misc static"},
		{"go/b/extra.go", 24, "b.Both", "b.Misc static"},
		{"go/b/extra.go", 27, "b.Describe", "b.Misc static"},
		{"go/b/extra.go", 28, "b.Box.Get", "b.Misc static"},
	}

	if got := places(callers(t, "b.Misc", codeindex.DirectionOut)); !slices.Equal(got, want) {
		t.Errorf("callees of b.Misc = %+v, want %+v", got, want)
	}
}

func TestGotypesBackend_APointerReceiverMethodImplementsTheInterfaceOfAnotherPackage(t *testing.T) {
	t.Parallel()
	want := []place{{"go/a/a.go", 17, "b.Resetter.Reset", "a.Engine.Reset implements b.Resetter.Reset: 1 call(s) through it"}}

	if got := places(callers(t, "Engine.Reset", codeindex.DirectionIn)); !slices.Equal(got, want) {
		t.Errorf("callers of Engine.Reset = %+v, want %+v", got, want)
	}
}

func TestGotypesBackend_APINamesTheMethodsOfGenericTypesByTheirTypeName(t *testing.T) {
	t.Parallel()

	hits, err := fixtureBackend().API(context.Background(), "go/b/extra.go")

	got := places(hits)
	for _, want := range []place{{"go/b/extra.go", 11, "Box.Get", "method"}, {"go/b/extra.go", 15, "Pair.Swap", "method"}} {
		if err != nil || !slices.Contains(got, want) {
			t.Errorf("API = %+v, %v; want it to hold %+v", got, err, want)
		}
	}
}

func TestGotypesBackend_AnExternalTestPackageIsIndexedThroughItsImportMap(t *testing.T) {
	t.Parallel()

	hits := callers(t, "c_test.callMain", codeindex.DirectionOut)

	if !slices.Equal(places(hits), []place{{"go/c/c_ext_test.go", 5, "c.Main", "c_test.callMain static"}}) {
		t.Errorf("callees = %+v", places(hits))
	}
}

func TestBuild_AnIndexThatCannotBeEncodedIsAnErrorAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "m.go": "package m\n"})
	out := filepath.Join(t.TempDir(), "index.gob")
	refuse := func(io.Writer, any) error { return errors.New("encode refused") }

	err := gotypes.BuildWith(context.Background(), sysexec.DefaultRunner, os.Environ(), refuse, io.Discard, dir, out)

	if err == nil || !strings.Contains(err.Error(), "encode refused") {
		t.Errorf("BuildWith = %v, want the encode error", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("a failed build wrote %s", out)
	}
}

func TestGotypesBackend_CallsInPackageLevelInitializersBelongToTheInitCallerOfThePackage(t *testing.T) {
	t.Parallel()
	for symbol, want := range map[string][]place{
		"c.Seed": {{"go/c/init.go", 7, "c.<init>", "c.Seed static"}},
		"b.Apply": {
			{"go/c/c.go", 9, "c.Main", "b.Apply static"},
			{"go/c/init.go", 5, "c.<init>", "b.Apply static"},
		},
	} {
		if got := places(callers(t, symbol, codeindex.DirectionIn)); !slices.Equal(got, want) {
			t.Errorf("callers of %s = %+v, want %+v", symbol, got, want)
		}
	}
	want := []place{{"go/c/init.go", 5, "b.Apply", "c.<init> static"}, {"go/c/init.go", 7, "c.Seed", "c.<init> static"}}
	if got := places(callers(t, "c.<init>", codeindex.DirectionOut)); !slices.Equal(got, want) {
		t.Errorf("callees of c.<init> = %+v, want %+v", got, want)
	}
}

func TestBuild_ABrokenPackageWarnsOnceAndTheOtherPackagesAreStillIndexed(t *testing.T) {
	t.Parallel()
	dir := writeModule(t, map[string]string{
		"go.mod":      "module m\n\ngo 1.23\n",
		"ok/ok.go":    "package ok\n\nfunc F() int { return G() }\n\nfunc G() int { return 1 }\n",
		"typed/t.go":  "package typed\n\nfunc H() int { return \"x\" }\n\nfunc I() int { return H() + H() }\n",
		"parsed/p.go": "package parsed\n\nfunc (\n",
		"parsed/q.go": "package parsed\n\nfunc J() int { return 1 }\n\nfunc K() int { return J() }\n",
	})
	out := filepath.Join(t.TempDir(), "index.gob")
	var warn strings.Builder

	err := gotypes.BuildWith(context.Background(), sysexec.DefaultRunner, os.Environ(), gotypes.EncodeGob, &warn, dir, out)

	if err != nil {
		t.Fatalf("BuildWith = %v, want an index despite the broken packages", err)
	}
	lines := strings.Split(strings.TrimSpace(warn.String()), "\n")
	if len(lines) != 2 || !strings.Contains(warn.String(), "WARN m/typed: ") || !strings.Contains(warn.String(), "WARN m/parsed: ") {
		t.Errorf("warnings = %q, want one WARN for m/typed and one for m/parsed", warn.String())
	}
	b := gotypes.New(out)
	for symbol, want := range map[string]place{"ok.G": {"ok/ok.go", 3, "ok.F", "ok.G static"}, "typed.H": {"typed/t.go", 5, "typed.I", "typed.H static"}, "parsed.J": {"parsed/q.go", 5, "parsed.K", "parsed.J static"}} {
		hits, err := b.Callers(context.Background(), symbol, codeindex.DirectionIn)
		if err != nil || !slices.Contains(places(hits), want) {
			t.Errorf("callers of %s = %+v, %v; want %+v", symbol, places(hits), err, want)
		}
	}
}

func TestBuild_ATypeErrorInAStandardPackageIsNotAWarning(t *testing.T) {
	t.Parallel()
	std := writeModule(t, map[string]string{"s.go": "package s\n\nvar V int = \"x\"\n\nfunc F() int { return V }\n"})
	mod := writeModule(t, map[string]string{"x.go": "package x\n\nimport \"s\"\n\nfunc X() int { return s.F() }\n"})
	mod, err := filepath.EvalSymlinks(mod)
	if err != nil {
		t.Fatal(err)
	}
	answer := `{"ImportPath":"s","Dir":"` + std + `","Standard":true,"GoFiles":["s.go"]}` +
		`{"ImportPath":"x","Dir":"` + mod + `","GoFiles":["x.go"],"Module":{"Main":true}}`
	out := filepath.Join(t.TempDir(), "i.gob")
	var warn strings.Builder

	err = gotypes.BuildWith(context.Background(), replying(answer, 0, nil), nil, gotypes.EncodeGob, &warn, mod, out)

	hits, apiErr := gotypes.New(out).API(context.Background(), "x.go")
	if err != nil || warn.Len() != 0 || apiErr != nil || !slices.Equal(places(hits), []place{{"x.go", 5, "X", "func"}}) {
		t.Errorf("BuildWith = %v, warnings = %q, API = %+v, %v", err, warn.String(), places(hits), apiErr)
	}
}

func TestBuild_AnUnlistedImportAndAMissingFileAreWarningsOfTheirPackage(t *testing.T) {
	t.Parallel()
	dir := writeModule(t, map[string]string{"x.go": "package x\n\nimport \"nope/y\"\n\nvar _ = y.Z\n"})
	answer := `{"ImportPath":"x","Dir":"` + dir + `","GoFiles":["x.go","gone.go"],"Module":{"Main":true}}`
	out := filepath.Join(t.TempDir(), "i.gob")
	var warn strings.Builder

	err := gotypes.BuildWith(context.Background(), replying(answer, 0, nil), nil, gotypes.EncodeGob, &warn, dir, out)

	if err != nil || !strings.Contains(warn.String(), "WARN x: 2 error(s)") {
		t.Errorf("BuildWith = %v, warnings = %q; want one WARN for x with the 2 errors", err, warn.String())
	}
}

func TestBuild_RunsGoListWithTheGivenEnvironmentAndCgoOff(t *testing.T) {
	t.Parallel()
	var env []string
	run := func(_ context.Context, _, _ string, _, e []string, _ io.Reader, _, _ io.Writer) (int, error) {
		env = e
		return 0, nil
	}
	base := []string{"PATH=/usr/bin"}

	err := gotypes.BuildWith(context.Background(), run, base, gotypes.EncodeGob, io.Discard, t.TempDir(), filepath.Join(t.TempDir(), "i.gob"))

	if err != nil || !slices.Equal(env, []string{"PATH=/usr/bin", "CGO_ENABLED=0"}) || len(base) != 1 {
		t.Errorf("BuildWith = %v, env = %v, base = %v", err, env, base)
	}
}

func TestIndexPath_IsBesideTheIndexWorktree(t *testing.T) {
	t.Parallel()

	if got := gotypes.IndexPath("/hub/dev/code-index"); got != "/hub/dev/code-index-gotypes/index.gob" {
		t.Errorf("IndexPath = %q", got)
	}
}

func TestBuild_WithNoCheckoutAboveTheModuleThePathsAreRelativeToTheModule(t *testing.T) {
	t.Parallel()
	dir := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "m.go": "package m\n\nfunc F() int { return G() }\n\nfunc G() int { return 1 }\n"})
	out := filepath.Join(t.TempDir(), "index.gob")

	if err := gotypes.Build(dir, out); err != nil {
		t.Fatalf("Build: %v", err)
	}
	hits, err := gotypes.New(out).Callers(context.Background(), "m.G", codeindex.DirectionIn)

	if err != nil || !slices.Equal(places(hits), []place{{"m.go", 3, "m.F", "m.G static"}}) {
		t.Errorf("callers = %+v, %v", places(hits), err)
	}
}

func TestBuild_ARebuildReplacesTheIndex(t *testing.T) {
	t.Parallel()
	dir := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "m.go": "package m\n\nfunc F() int { return 1 }\n"})
	out := filepath.Join(t.TempDir(), "index.gob")
	if err := gotypes.Build(dir, out); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "m.go"), []byte("package m\n\nfunc H() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := gotypes.Build(dir, out); err != nil {
		t.Fatal(err)
	}
	hits, err := gotypes.New(out).API(context.Background(), "m.go")

	if err != nil || !slices.Equal(places(hits), []place{{"m.go", 3, "H", "func"}}) {
		t.Errorf("API = %+v, %v; want only H", places(hits), err)
	}
}

func TestBuild_FailsLoudly(t *testing.T) {
	t.Parallel()
	notModule := t.TempDir()
	good := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "m.go": "package m\n"})
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		dir, out, want string
	}{
		"absent module dir":    {filepath.Join(notModule, "gone"), filepath.Join(t.TempDir(), "i.gob"), "gone"},
		"not a module":         {notModule, filepath.Join(t.TempDir(), "i.gob"), "go list"},
		"unwritable index dir": {good, filepath.Join(blocker, "i.gob"), "i.gob"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := gotypes.Build(tc.dir, tc.out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Build = %v, want an error naming %q", err, tc.want)
			}
			if _, statErr := os.Stat(tc.out); statErr == nil {
				t.Errorf("a failed build wrote %s", tc.out)
			}
		})
	}
}

func TestBuild_FailsLoudlyOnABadGoListAnswer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, tc := range map[string]struct {
		stdout string
		exit   int
		err    error
		want   string
	}{
		"runner error": {"", -1, errors.New("boom"), "boom"},
		"bad json":     {"{", 0, nil, "go list"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(t.TempDir(), "i.gob")
			err := gotypes.BuildWith(context.Background(), replying(tc.stdout, tc.exit, tc.err), nil, gotypes.EncodeGob, io.Discard, dir, out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("BuildWith = %v, want an error naming %q", err, tc.want)
			}
		})
	}
}
