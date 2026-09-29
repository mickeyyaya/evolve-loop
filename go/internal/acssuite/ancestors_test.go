package acssuite

import (
	"reflect"
	"testing"
)

func TestAncestorCyclePackages_NamesOnlyOtherCyclesOwnPackages(t *testing.T) {
	t.Parallel()
	added := []string{
		"go/acs/cycle1761/predicates_test.go",
		"go/acs/cycle1761/helpers_test.go",
		"go/acs/cycle1759/predicates_test.go",
		"go/acs/cycle1764/predicates_test.go",
		"go/acs/regression/cycle300/predicates_test.go",
		"go/acs/redteam/probe_test.go",
		"go/acs/cycle1770x/predicates_test.go",
		"go/internal/gc/gc.go",
		"docs/acs/cycle1761/notes.md",
	}

	got := AncestorCyclePackages(added, 1764)

	want := []string{"go/acs/cycle1759", "go/acs/cycle1761"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AncestorCyclePackages = %v, want %v: another cycle's own package, deduplicated and sorted; never this cycle's, regression, redteam or a look-alike", got, want)
	}
}

func TestAncestorCyclePackages_NoneWhenOnlyThisCyclesPackageWasAdded(t *testing.T) {
	t.Parallel()
	if got := AncestorCyclePackages([]string{"go/acs/cycle1764/predicates_test.go"}, 1764); len(got) != 0 {
		t.Errorf("AncestorCyclePackages = %v, want none", got)
	}
}
