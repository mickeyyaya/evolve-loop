package admission_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
)

func full(short ...string) []string {
	out := make([]string, len(short))
	for i, s := range short {
		out[i] = admission.ModulePath + "/" + s
	}
	return out
}

func TestModuleGraph_HubsAreThePackagesWithAtLeastTheMinimumImporters(t *testing.T) {
	g := testGraph(t)
	if got, want := g.Hubs(), map[string]bool{admission.ModulePath + "/internal/sysexec": true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Hubs() = %v, want %v: sysexec has 4 importers, log has 1", got, want)
	}
	low, err := admission.NewModuleGraph(listing(testModule), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := low.Hubs(); len(got) != 3 || !got[admission.ModulePath+"/internal/log"] || !got[admission.ModulePath+"/internal/inboxrank"] {
		t.Errorf("Hubs() at a minimum of 1 = %v, want sysexec, log and inboxrank", got)
	}
}

func TestModuleGraph_DepsAreTheModuleInternalTransitiveImports(t *testing.T) {
	g := testGraph(t)
	if got, want := g.Deps(admission.ModulePath+"/internal/fleet"), full("internal/inboxrank", "internal/sysexec"); !reflect.DeepEqual(got, want) {
		t.Fatalf("Deps(fleet) = %v, want %v: the standard library is not a cell", got, want)
	}
	if got := g.Deps(admission.ModulePath + "/internal/nosuch"); len(got) != 0 {
		t.Errorf("Deps of an unknown package = %v, want none", got)
	}
}

func TestModuleGraph_PackagesMapsEachPathUnderGoToItsPackage(t *testing.T) {
	g := testGraph(t)
	cases := []struct {
		name  string
		files []string
		want  []string
	}{
		{"a go file", []string{"go/internal/fleet/pool.go"}, full("internal/fleet")},
		{"a test file in the same package", []string{"go/internal/fleet/pool_test.go", "go/internal/fleet/pool.go"}, full("internal/fleet")},
		{"a go file of a new package", []string{"go/internal/admission/admission.go"}, full("internal/admission")},
		{"a package directory", []string{"go/internal/inboxrank/"}, full("internal/inboxrank")},
		{"package data", []string{"go/internal/policy/testdata/fleet.json"}, full("internal/policy")},
		{"a module-root go file", []string{"go/doc.go"}, []string{admission.ModulePath}},
		{"a file outside go", []string{"docs/a.md", "README.md"}, nil},
		{"a go-tree file of no package", []string{"go/.cover-strict"}, nil},
		{"two packages sorted", []string{"go/internal/quotastate/w.go", "go/internal/fleet/pool.go"}, full("internal/fleet", "internal/quotastate")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.Packages(tc.files); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Packages(%v) = %v, want %v", tc.files, got, tc.want)
			}
		})
	}
}

func TestNewModuleGraph_RefusesALineWithoutTheSeparator(t *testing.T) {
	_, err := admission.NewModuleGraph(admission.ModulePath+"/internal/fleet\n", 4)
	if err == nil || !strings.Contains(err.Error(), "internal/fleet") {
		t.Fatalf("err = %v, want a refusal that names the malformed line", err)
	}
}

func TestNewModuleGraph_SkipsBlankLines(t *testing.T) {
	g, err := admission.NewModuleGraph("\n"+listing(testModule)+"\n\n", testHubMin)
	if err != nil || len(g.Hubs()) != 1 {
		t.Fatalf("g = %v, err = %v: blank lines carry no package", g, err)
	}
}

func fakeGoList(out string, code int, runErr error, calls *[]string) func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return func(_ context.Context, name, dir string, args, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		*calls = append(*calls, dir+": "+name+" "+strings.Join(args, " "))
		io.WriteString(stdout, out)
		io.WriteString(stderr, "go: boom")
		return code, runErr
	}
}

func TestLoadModuleGraph_RunsOneGoListInTheModuleDir(t *testing.T) {
	var calls []string
	g, err := admission.LoadModuleGraph(context.Background(), fakeGoList(listing(testModule), 0, nil, &calls), "/repo/go", testHubMin)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/repo/go: go list -f {{.ImportPath}}|{{join .Deps \" \"}} ./..."}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
	if !g.Hubs()[admission.ModulePath+"/internal/sysexec"] {
		t.Errorf("the loaded graph has no hub: %v", g.Hubs())
	}
}

func TestLoadModuleGraph_ReportsAGoListFailure(t *testing.T) {
	var calls []string
	if _, err := admission.LoadModuleGraph(context.Background(), fakeGoList("", 1, nil, &calls), "/repo/go", 4); err == nil || !strings.Contains(err.Error(), "go: boom") {
		t.Fatalf("err = %v, want the go list stderr", err)
	}
	boom := errors.New("no go binary")
	if _, err := admission.LoadModuleGraph(context.Background(), fakeGoList("", -1, boom, &calls), "/repo/go", 4); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap %v", err, boom)
	}
}
