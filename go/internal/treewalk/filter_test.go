package treewalk

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestFilterDropsPathsAtOrUnderAnAllowedPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		paths []string
		allow []string
		want  []string
	}{
		{"no allow list keeps every path", []string{"a", filepath.Join("b", "c")}, nil, []string{"a", filepath.Join("b", "c")}},
		{"empty allow list keeps every path", []string{"a"}, []string{}, []string{"a"}},
		{"exact match is dropped", []string{"coverage.txt", "stray"}, []string{"coverage.txt"}, []string{"stray"}},
		{"directory prefix drops the directory and its content", []string{"bin", filepath.Join("bin", "evolve"), filepath.Join("bin", "dist", "x"), "stray"}, []string{"bin"}, []string{"stray"}},
		{"prefix stops at a path component", []string{"binary", filepath.Join("binary", "x"), "bin.old", "bin"}, []string{"bin"}, []string{"binary", filepath.Join("binary", "x"), "bin.old"}},
		{"a file prefix does not allow a longer name", []string{"coverage.txt", "coverage.txt.bak"}, []string{"coverage.txt"}, []string{"coverage.txt.bak"}},
		{"trailing separator is ignored", []string{"bin", filepath.Join("bin", "x"), "other"}, []string{"bin" + string(filepath.Separator)}, []string{"other"}},
		{"leading dot segment is ignored", []string{filepath.Join("bin", "x"), "other"}, []string{"." + string(filepath.Separator) + "bin"}, []string{"other"}},
		{"several allow entries combine", []string{"a", "b", "c"}, []string{"a", "c"}, []string{"b"}},
		{"a nested allow entry does not open its parent", []string{"d", filepath.Join("d", "keep"), filepath.Join("d", "out")}, []string{filepath.Join("d", "out")}, []string{"d", filepath.Join("d", "keep")}},
		{"nothing left yields no path", []string{"a", filepath.Join("a", "b")}, []string{"a"}, nil},
	}
	for _, c := range cases {
		got := Filter(c.paths, c.allow)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: Filter(%v, %v) = %v, want %v", c.name, c.paths, c.allow, got, c.want)
		}
	}
}

func TestFilterKeepsOrderAndLeavesItsInputsUntouched(t *testing.T) {
	t.Parallel()
	paths := []string{"z", filepath.Join("bin", "x"), "a", "m"}
	allow := []string{"bin"}
	pathsBefore, allowBefore := slices.Clone(paths), slices.Clone(allow)
	got := Filter(paths, allow)
	if want := []string{"z", "a", "m"}; !slices.Equal(got, want) {
		t.Errorf("Filter = %v, want %v in input order", got, want)
	}
	if !slices.Equal(paths, pathsBefore) || !slices.Equal(allow, allowBefore) {
		t.Errorf("Filter changed its inputs: paths %v -> %v, allow %v -> %v", pathsBefore, paths, allowBefore, allow)
	}
}

func TestFilterAppliedToADiffHidesOnlyTheAllowedLeaks(t *testing.T) {
	t.Parallel()
	before := Snapshot{Paths: []string{"keep"}}
	after := Snapshot{Paths: []string{"keep", "bin", filepath.Join("bin", "coversplit"), ".evolve", filepath.Join(".evolve", "ledger.ndjson")}}
	got := Filter(Diff(before, after), []string{"bin"})
	if want := []string{".evolve", filepath.Join(".evolve", "ledger.ndjson")}; !slices.Equal(got, want) {
		t.Errorf("Filter(Diff) = %v, want only the leaked %v", got, want)
	}
}
