package explanationdocs

import (
	"reflect"
	"testing"
)

var coverChanged = []string{
	"go/acs/cycle1768/helpers_test.go",
	"go/acs/cycle1768/predicates_test.go",
	"go/internal/phasecoherence/coherence.go",
}

var coverMaterial = []string{"go/internal/phasecoherence/coherence.go"}

func TestChangedAreaFailures_ACitationCoveringDiffPathsIsNotInvented(t *testing.T) {
	section := "" +
		"- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps under the 50-line limit\n" +
		"- `go/acs/cycle1768/*_test.go` — the cycle's predicates for the shrink and its unchanged behavior\n" +
		"- `go/acs/cycle1768` — the predicate package this cycle adds\n" +
		"- `go/acs/{cycle1768,cycle9}/predicates_test.go` — the predicate file, cited by brace list\n"

	if got := changedAreaFailures(section, coverChanged, coverMaterial); len(got) != 0 {
		t.Errorf("failures = %v, want none: a directory, glob or brace list that covers diff paths names real Build content (cycles 1765 and 1768 each lost a correction round to this)", got)
	}
}

func TestChangedAreaFailures_ACitationCoveringNothingIsStillInvented(t *testing.T) {
	section := "" +
		"- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps under the 50-line limit\n" +
		"- `go/acs/cycle9999/*_test.go` — predicates of a cycle this diff does not touch\n"

	got := changedAreaFailures(section, coverChanged, coverMaterial)

	want := []string{"Explanation Documentation: cited path go/acs/cycle9999/*_test.go is not in the Build diff"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("failures = %v, want %v", got, want)
	}
}

func TestChangedAreaFailures_AMaterialPathNeedsItsOwnEntryEvenUnderACoveringGlob(t *testing.T) {
	section := "- `go/internal/phasecoherence/*.go` — the package's changes, cited as one glob\n"

	got := changedAreaFailures(section, coverChanged, coverMaterial)

	want := []string{"Explanation Documentation: Changed Areas does not explain material path go/internal/phasecoherence/coherence.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("failures = %v, want %v: a pattern is accepted as a citation, never as the per-path explanation a material change owes", got, want)
	}
}
