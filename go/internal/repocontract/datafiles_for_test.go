package repocontract

import (
	"slices"
	"testing"
)

func testNames(selections []TestSelection) []string {
	var names []string
	for _, selection := range selections {
		for _, test := range selection.Tests {
			names = append(names, selection.Package+"."+test)
		}
	}
	slices.Sort(names)
	return names
}

func TestByNameTestsFor_ANonGoPathSelectsTheCoarseSet(t *testing.T) {
	t.Parallel()
	want := testNames(ByNameTests())
	for _, changed := range [][]string{
		{".github/workflows/required.yml"},
		{"go/internal/core/x.go", "go/Makefile"},
		{"go/internal/core/x.go", ".evolve/policy.json"},
	} {
		if got := testNames(ByNameTestsFor(changed)); !slices.Equal(got, want) {
			t.Errorf("ByNameTestsFor(%q) names %d tests, want the coarse %d", changed, len(got), len(want))
		}
	}
	if !slices.Contains(testNames(ByNameTestsFor([]string{".github/workflows/required.yml"})), "./internal/ciparity.TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies") {
		t.Error("a workflow change must select the ciparity workflow test")
	}
}

func TestByNameTestsFor_ADocsOnlyChangeSelectsTheCoarseSet(t *testing.T) {
	t.Parallel()
	want := testNames(ByNameTests())
	if got := testNames(ByNameTestsFor([]string{"docs/architecture/signal-codes.md"})); !slices.Equal(got, want) {
		t.Errorf("a docs change names %d tests, want the coarse %d; docs/ is not left out (D20)", len(got), len(want))
	}
}

func TestByNameTestsFor_AGoOnlyChangeSelectsTheAlwaysSet(t *testing.T) {
	t.Parallel()
	coarse := testNames(ByNameTests())
	for _, changed := range [][]string{{"go/internal/core/x.go"}, nil, {}} {
		got := testNames(ByNameTestsFor(changed))
		if len(got) >= len(coarse) {
			t.Errorf("ByNameTestsFor(%q) names %d tests, want fewer than the coarse %d", changed, len(got), len(coarse))
		}
		if slices.Contains(got, "./internal/ciparity.TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies") {
			t.Errorf("ByNameTestsFor(%q) must leave out the workflow data row", changed)
		}
		for _, selection := range TreeReadingTests() {
			for _, test := range selection.Tests {
				if !slices.Contains(got, selection.Package+"."+test) {
					t.Fatalf("the always set holds the tree-reading test %s.%s", selection.Package, test)
				}
			}
		}
		for _, row := range DataFileReads() {
			if row.Path != WholeRepository {
				continue
			}
			if !slices.Contains(got, row.Package+"."+row.Tests[0]) {
				t.Fatalf("the always set holds the WholeRepository row %s.%s", row.Package, row.Tests[0])
			}
		}
	}
}

func TestByNameTestsFor_TheAlwaysSetHoldsTheDataRowsUnderGo(t *testing.T) {
	t.Parallel()
	got := testNames(ByNameTestsFor([]string{"go/internal/core/x.go"}))
	for _, row := range DataFileReads() {
		if len(row.Path) >= 3 && row.Path[:3] == "go/" && !slices.Contains(got, row.Package+"."+row.Tests[0]) {
			t.Errorf("the always set holds the row %q %s.%s", row.Path, row.Package, row.Tests[0])
		}
	}
}

func TestUnknownChange_SelectsTheCoarseSet(t *testing.T) {
	t.Parallel()
	if got, want := testNames(ByNameTestsFor(UnknownChange())), testNames(ByNameTests()); !slices.Equal(got, want) {
		t.Errorf("UnknownChange selects %d tests, want the coarse %d", len(got), len(want))
	}
}
