package gotypes_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

func truthOf(t *testing.T) map[string]codeindex.BenchSymbol {
	t.Helper()
	pool, err := gotypes.Truth(fixtureIndex)
	if err != nil {
		t.Fatalf("Truth: %v", err)
	}
	bySymbol := map[string]codeindex.BenchSymbol{}
	for _, sym := range pool {
		bySymbol[sym.Name] = sym
	}
	return bySymbol
}

func TestTruth_AMissingIndexIsAbsent(t *testing.T) {
	t.Parallel()

	_, err := gotypes.Truth(filepath.Join(t.TempDir(), "none.gob"))

	if !errors.Is(err, codeindex.ErrAbsent) {
		t.Errorf("err = %v, want ErrAbsent", err)
	}
}

func TestTruth_ListsEachModuleFunctionThatHasACallerWithItsSites(t *testing.T) {
	t.Parallel()

	pool := truthOf(t)

	drive := pool["b.Drive"]
	if want := []codeindex.TruthSite{{File: "go/c/c.go", Line: 9, Func: "c.Main"}}; !slices.Equal(drive.Sites, want) {
		t.Errorf("b.Drive sites = %+v, want %+v", drive.Sites, want)
	}
	if drive.DefFile != "go/b/b.go" || drive.DefLine != 7 {
		t.Errorf("b.Drive is defined at %s:%d, want go/b/b.go:7", drive.DefFile, drive.DefLine)
	}
	if _, listed := pool["a.Engine.Reset"]; listed {
		t.Errorf("a function with no caller must not be a candidate")
	}
	if _, listed := pool["error.Error"]; listed {
		t.Errorf("a function outside the module must not be a candidate")
	}
}

func TestTruth_OnlyAUniqueStaticNonTestSymbolCanBeGrepChecked(t *testing.T) {
	t.Parallel()
	pool := truthOf(t)

	if len(pool["b.Drive"].GrepFiles) == 0 {
		t.Fatalf("b.Drive has one definition and only static calls, so it is checkable")
	}
	for _, file := range pool["b.Drive"].GrepFiles {
		if strings.HasSuffix(file, "_test.go") {
			t.Errorf("grep files hold the test file %s", file)
		}
	}
	for name, why := range map[string]string{
		"a.Digest":     "two functions share the bare name",
		"a.Runner.Run": "an interface method",
	} {
		if sym, listed := pool[name]; listed && len(sym.GrepFiles) != 0 {
			t.Errorf("%s must not be grep-checked: %s", name, why)
		}
	}
}
