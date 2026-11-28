package codeindex_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

func TestAmplified_IndependentTruth_SymbolsAreWellFormed(t *testing.T) {
	syms, err := codeindex.IndependentTruth()
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("independent truth is empty")
	}
	names := map[string]bool{}
	for _, s := range syms {
		if s.Name == "" || s.DefFile == "" || s.DefLine <= 0 {
			t.Fatalf("malformed symbol %+v", s)
		}
		if names[s.Name] {
			t.Fatalf("duplicate symbol %s", s.Name)
		}
		names[s.Name] = true
		if len(s.Sites) == 0 {
			t.Fatalf("symbol %s has no sites", s.Name)
		}
		seen := map[codeindex.TruthSite]bool{}
		for _, site := range s.Sites {
			if site.File == "" || site.Func == "" || site.Line <= 0 {
				t.Fatalf("malformed site %+v of %s", site, s.Name)
			}
			if seen[site] {
				t.Fatalf("duplicate site %+v of %s", site, s.Name)
			}
			seen[site] = true
		}
	}
}

func TestAmplified_IndependentTruth_IsDeterministicAndIndependentPerCall(t *testing.T) {
	a, err := codeindex.IndependentTruth()
	if err != nil {
		t.Fatal(err)
	}
	b, err := codeindex.IndependentTruth()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two loads differ")
	}
	if len(a[0].Sites) > 0 {
		a[0].Sites[0].Line = -99
	}
	c, _ := codeindex.IndependentTruth()
	if !reflect.DeepEqual(b, c) {
		t.Fatal("mutating a result leaked into the next load")
	}
}

func TestAmplified_WriteIndependentFixture_WritesGoFilesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := codeindex.WriteIndependentFixture(dir); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		out := map[string]string{}
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b, _ := os.ReadFile(p)
				out[p] = string(b)
			}
			return nil
		})
		return out
	}
	first := snapshot()
	if len(first) == 0 {
		t.Fatal("nothing written")
	}
	hasGo := false
	for p := range first {
		if strings.HasSuffix(p, ".go") {
			hasGo = true
		}
		if strings.HasSuffix(p, ".txt") {
			t.Fatalf("fixture leaked a .txt file: %s", p)
		}
	}
	if !hasGo {
		t.Fatal("no .go file written")
	}
	if err := codeindex.WriteIndependentFixture(dir); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !reflect.DeepEqual(first, snapshot()) {
		t.Fatal("second write changed the tree")
	}
}

func TestAmplified_WriteIndependentFixture_FailsWhenTargetIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := codeindex.WriteIndependentFixture(f); err == nil {
		t.Fatal("want an error when the target is a regular file")
	}
}

func TestAmplified_Table_ShowsEachRowTruthSet(t *testing.T) {
	rep := codeindex.BenchReport{Rows: []codeindex.BenchRow{
		{TruthSet: "alpha-set", Backend: "b1", Query: codeindex.Query("callers")},
		{TruthSet: "beta-set", Backend: "b1", Query: codeindex.Query("callers")},
	}}
	table := rep.Table()
	for _, want := range []string{"alpha-set", "beta-set", "set"} {
		if !strings.Contains(table, want) {
			t.Fatalf("table lacks %q:\n%s", want, table)
		}
	}
}
