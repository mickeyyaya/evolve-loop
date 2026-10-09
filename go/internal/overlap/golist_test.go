package overlap

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func fixtureRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "repo"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return root
}

func findPackage(t *testing.T, m Module, importPath string) Package {
	t.Helper()
	for _, p := range m.Packages {
		if p.ImportPath == importPath {
			return p
		}
	}
	t.Fatalf("package %s not in %+v", importPath, m.Packages)
	return Package{}
}

func TestGraph_IsTheUnionOfTheFourTagSets(t *testing.T) {
	m, err := LoadModule(context.Background(), sysexec.DefaultRunner, fixtureRepo(t))
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}

	a := findPackage(t, m, "example.com/fix/a")

	want := []string{"example.com/fix/b", "example.com/fix/c", "example.com/fix/d", "example.com/fix/e"}
	if !slices.Equal(a.Deps, want) {
		t.Fatalf("deps of a = %v, want %v (module-internal, the union of none, integration, acs and e2e+evolve_test_phases)", a.Deps, want)
	}
}

func TestLoadModule_ListsEveryFileOfAPackageRepoRelative(t *testing.T) {
	m, err := LoadModule(context.Background(), sysexec.DefaultRunner, fixtureRepo(t))
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}

	a := findPackage(t, m, "example.com/fix/a")
	b := findPackage(t, m, "example.com/fix/b")

	if m.Path != "example.com/fix" || a.Dir != "go/a" {
		t.Fatalf("module %q dir %q, want example.com/fix go/a", m.Path, a.Dir)
	}
	want := []string{"go/a/a.go", "go/a/a_acs.go", "go/a/a_e2e.go", "go/a/a_e2e_only.go", "go/a/a_integration.go", "go/a/a_test.go", "go/a/ax_test.go", "go/a/data.txt"}
	if !slices.Equal(a.Files, want) {
		t.Fatalf("files of a = %v, want %v", a.Files, want)
	}
	if len(b.Deps) != 0 || len(m.Packages) != 6 {
		t.Fatalf("deps of b = %v, packages = %d, want none and 6", b.Deps, len(m.Packages))
	}
}

func TestLoadModule_TheProofMapsTheFixtureTestdataAndEmbedToTheirPackage(t *testing.T) {
	m, err := LoadModule(context.Background(), sysexec.DefaultRunner, fixtureRepo(t))
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	in := Input{Lane: []string{"go/b/b.go"}, Peer: []string{"go/a/testdata/x.json", "go/a/data.txt"}, Merge: MergeClean, Module: m, Catalogs: fakeCatalogs{}}

	got := Prove(in)

	assertTier(t, got, T3, RulePackageEdge)
	assertEdges(t, "edges_peer_to_lane", got.Evidence.EdgesPeerToLane, Edge{From: "example.com/fix/a", To: "example.com/fix/b"})
	assertStrings(t, "unknown", got.Evidence.Unknown)
}

func fakeRun(code int, stdout string, err error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, errOut io.Writer) (int, error) {
		_, _ = io.WriteString(out, stdout)
		_, _ = io.WriteString(errOut, "boom")
		return code, err
	}
}

func TestLoadModule_FailsLoudly(t *testing.T) {
	cases := []struct {
		name string
		run  sysexec.RunFunc
		want string
	}{
		{"run error", fakeRun(-1, "", errors.New("no go")), "no go"},
		{"non-zero exit", fakeRun(1, "", nil), "exit 1: boom"},
		{"bad json", fakeRun(0, "{", nil), "decode"},
		{"no module", fakeRun(0, `{"ImportPath":"x","Dir":"/r/go/x"}`, nil), "x: no module"},
		{"dir outside the module", fakeRun(0, `{"ImportPath":"x","Dir":"/elsewhere/x","Module":{"Path":"x","Dir":"/r/go"}}`, nil), "outside the module"},
		{"relative dir", fakeRun(0, `{"ImportPath":"x","Dir":"x","Module":{"Path":"x","Dir":"/r/go"}}`, nil), "x:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadModule(context.Background(), tc.run, "/r")

			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "go list") {
				t.Fatalf("err = %v, want a go list error holding %q", err, tc.want)
			}
		})
	}
}

func TestLoadModule_TheModuleRootPackageIsTheGoDirectory(t *testing.T) {
	run := fakeRun(0, `{"ImportPath":"m","Dir":"/r/go","Module":{"Path":"m","Dir":"/r/go"},"GoFiles":["main.go"],"Deps":["fmt","m/x"]}`, nil)

	m, err := LoadModule(context.Background(), run, "/r")

	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	if len(m.Packages) != 1 || m.Packages[0].Dir != "go" || !slices.Equal(m.Packages[0].Files, []string{"go/main.go"}) || !slices.Equal(m.Packages[0].Deps, []string{"m/x"}) {
		t.Fatalf("packages = %+v, want one at go with go/main.go and dep m/x", m.Packages)
	}
}

func TestLoadModule_RunsTheFourTagSets(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name, dir string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		calls = append(calls, append([]string{name, dir}, args...))
		return 0, nil
	}

	if _, err := LoadModule(context.Background(), run, "/r"); err != nil {
		t.Fatalf("LoadModule: %v", err)
	}

	want := [][]string{
		{"go", "/r/go", "list", "-json", "./..."},
		{"go", "/r/go", "list", "-json", "-tags", "integration", "./..."},
		{"go", "/r/go", "list", "-json", "-tags", "acs", "./..."},
		{"go", "/r/go", "list", "-json", "-tags", "e2e,evolve_test_phases", "./..."},
	}
	if !slices.EqualFunc(calls, want, slices.Equal[[]string]) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}
