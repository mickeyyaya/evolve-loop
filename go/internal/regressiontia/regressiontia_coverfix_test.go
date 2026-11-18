package regressiontia

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

func resolveDepsCoverfixRunner(listing string) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, _ string, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		if len(args) > 1 && args[1] == "-m" {
			_, _ = io.WriteString(stdout, "example.com/x\n")
			return 0, nil
		}
		_, _ = io.WriteString(stdout, listing)
		return 0, nil
	}
}

func withResolveDepsCoverfixListing(t *testing.T, listing string) {
	t.Helper()
	orig := depsRunner
	t.Cleanup(func() { depsRunner = orig })
	depsRunner = resolveDepsCoverfixRunner(listing)
}

func TestResolveDepsCoverfix_EmptyInputsYieldNil(t *testing.T) {
	if got := skippableDeps(listCorpusDeps("", []string{"./acs/regression/a"})); got != nil {
		t.Fatalf("empty moduleDir must yield nil, got %v", got)
	}
}

func TestResolveDepsCoverfix_DerivesSortedDedupedPatternsAndDropsUnderivable(t *testing.T) {
	listing := strings.Join([]string{
		"example.com/x/acs/regression/derivable [example.com/x/acs/regression/derivable.test]|fmt example.com/x/internal/foo example.com/x/internal/bar example.com/x/internal/foo",
		"example.com/x/acs/regression/derivable.test|example.com/x/internal/foo",
		"example.com/x/acs/regression/hatch|example.com/x/internal/foo os",
		"example.com/x/acs/regression/plain|fmt strings",
		"not-a-record",
		"example.com/x/acs/regression/unwanted|example.com/x/internal/foo",
		"",
	}, "\n")
	withResolveDepsCoverfixListing(t, listing)
	got := skippableDeps(listCorpusDeps(t.TempDir(), []string{"./acs/regression/derivable", "./acs/regression/hatch", "./acs/regression/plain"}))
	want := map[string][]string{
		"./acs/regression/derivable": {"./internal/bar/...", "./internal/foo/..."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveDepsCoverfix_NothingDerivableYieldsNil(t *testing.T) {
	withResolveDepsCoverfixListing(t, "example.com/x/acs/regression/hatch|os\nexample.com/x/acs/regression/plain|fmt\n")
	if got := skippableDeps(listCorpusDeps(t.TempDir(), []string{"./acs/regression/hatch", "./acs/regression/plain"})); got != nil {
		t.Fatalf("no derivable package must yield nil, got %v", got)
	}
}

func TestListCorpusDepsCoverfix_EmptyModulePathYieldsNothing(t *testing.T) {
	orig := depsRunner
	t.Cleanup(func() { depsRunner = orig })
	depsRunner = func(_ context.Context, _ string, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		return 0, nil
	}
	deps, mod := listCorpusDeps(t.TempDir(), []string{"./acs/regression/a"})
	if deps != nil || mod != "" {
		t.Fatalf("empty module path must yield nothing, got %v %q", deps, mod)
	}
}

func TestListCorpusDepsCoverfix_ListingFailureYieldsNothing(t *testing.T) {
	orig := depsRunner
	t.Cleanup(func() { depsRunner = orig })
	depsRunner = func(_ context.Context, _ string, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		if len(args) > 1 && args[1] == "-m" {
			_, _ = io.WriteString(stdout, "example.com/x\n")
			return 0, nil
		}
		return 1, nil
	}
	deps, mod := listCorpusDeps(t.TempDir(), []string{"./acs/regression/a"})
	if deps != nil || mod != "" {
		t.Fatalf("failed listing must yield nothing, got %v %q", deps, mod)
	}
}

func TestCorpus_MissingInputsYieldNil(t *testing.T) {
	if got := corpus(""); got != nil {
		t.Fatalf("empty moduleDir must yield nil, got %v", got)
	}
	if got := corpus(t.TempDir()); got != nil {
		t.Fatalf("a module without a regression directory must yield nil, got %v", got)
	}
}

func TestSelect_AnEmptyDependencyPatternOverlapsEveryScope(t *testing.T) {
	deps := map[string][]string{"./acs/regression/a": {"./..."}}
	selected, wouldSkip := Select([]string{"./acs/regression/a"}, []string{"./internal/x"}, deps)
	if len(selected) != 1 || len(wouldSkip) != 0 {
		t.Fatalf("selected=%v wouldSkip=%v: a root-wide dependency must stay selected", selected, wouldSkip)
	}
}

func TestListCorpusDepsErrors_OmitsErroredPackagesAndTheirImporters(t *testing.T) {
	withResolveDepsCoverfixListing(t, strings.Join([]string{
		"example.com/x/internal/ghost|ERR",
		"example.com/x/acs/regression/a|example.com/x/internal/ghost",
		"example.com/x/acs/regression/b|ERR",
		"example.com/x/acs/regression/b_test|example.com/x/internal/leaf",
		"example.com/x/acs/regression/c|example.com/x/internal/leaf",
		"example.com/x/internal/leaf|",
	}, "\n"))
	patterns := []string{"./acs/regression/a", "./acs/regression/b", "./acs/regression/c"}
	deps, _ := listCorpusDeps(t.TempDir(), patterns)
	if _, ok := deps["./acs/regression/a"]; ok {
		t.Error("a reaches an errored package and must have no dependency entry")
	}
	if _, ok := deps["./acs/regression/b"]; ok {
		t.Error("b has a go list error and must have no dependency entry")
	}
	if !reflect.DeepEqual(deps["./acs/regression/c"], []string{"example.com/x/internal/leaf"}) {
		t.Errorf("c = %v, want its leaf closure", deps["./acs/regression/c"])
	}
}
