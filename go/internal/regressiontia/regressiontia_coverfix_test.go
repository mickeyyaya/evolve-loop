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
	if got := resolveDeps("", []string{"./acs/regression/a"}); got != nil {
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
	got := resolveDeps(t.TempDir(), []string{"./acs/regression/derivable", "./acs/regression/hatch", "./acs/regression/plain"})
	want := map[string][]string{
		"./acs/regression/derivable": {"./internal/bar/...", "./internal/foo/..."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveDepsCoverfix_NothingDerivableYieldsNil(t *testing.T) {
	withResolveDepsCoverfixListing(t, "example.com/x/acs/regression/hatch|os\nexample.com/x/acs/regression/plain|fmt\n")
	if got := resolveDeps(t.TempDir(), []string{"./acs/regression/hatch", "./acs/regression/plain"}); got != nil {
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
