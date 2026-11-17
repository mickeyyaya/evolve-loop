package repocontract

import (
	"slices"
	"testing"
)

func TestByNameTests_MergesTheTreeAndDataRowsPerPackage(t *testing.T) {
	t.Parallel()
	tree := []TestSelection{
		{Package: "./internal/a", Tests: []string{"TestA", "TestB"}},
		{Package: "./internal/b", Tests: []string{"TestC"}},
	}
	data := []DataFileRead{
		{Path: ".github/workflows/", TestSelection: TestSelection{Package: "./internal/b", Tests: []string{"TestC", "TestD"}}},
		{Path: WholeRepository, TestSelection: TestSelection{Package: "./internal/c", Tests: []string{"TestE"}}},
		{Path: "docs/", TestSelection: TestSelection{Package: "./internal/a", Tests: []string{"TestZ", "TestA"}}},
	}
	want := []TestSelection{
		{Package: "./internal/a", Tests: []string{"TestA", "TestB", "TestZ"}},
		{Package: "./internal/b", Tests: []string{"TestC", "TestD"}},
		{Package: "./internal/c", Tests: []string{"TestE"}},
	}
	got := mergeSelections(tree, data)
	if len(got) != len(want) {
		t.Fatalf("merged = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Package != want[i].Package || !slices.Equal(got[i].Tests, want[i].Tests) {
			t.Errorf("row %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestByNameTests_CarriesEveryTreeAndDataRow(t *testing.T) {
	t.Parallel()
	byName := map[string][]string{}
	for _, selection := range ByNameTests() {
		if byName[selection.Package] != nil {
			t.Errorf("%s appears twice", selection.Package)
		}
		byName[selection.Package] = selection.Tests
		if !slices.IsSorted(selection.Tests) || len(slices.Compact(slices.Clone(selection.Tests))) != len(selection.Tests) {
			t.Errorf("%s tests are not sorted and unique: %v", selection.Package, selection.Tests)
		}
	}
	for _, selection := range TreeReadingTests() {
		for _, test := range selection.Tests {
			if !slices.Contains(byName[selection.Package], test) {
				t.Errorf("%s.%s is missing from ByNameTests()", selection.Package, test)
			}
		}
	}
	for _, row := range DataFileReads() {
		for _, test := range row.Tests {
			if !slices.Contains(byName[row.Package], test) {
				t.Errorf("%s.%s is missing from ByNameTests()", row.Package, test)
			}
		}
	}
}

func TestDataFileReads_ReturnsACopy(t *testing.T) {
	t.Parallel()
	rows := DataFileReads()
	if len(rows) == 0 {
		t.Fatal("the registry holds no row")
	}
	rows[0].Tests[0] = "Mutated"
	if DataFileReads()[0].Tests[0] == "Mutated" {
		t.Fatal("DataFileReads() shares its slices with the registry")
	}
}
