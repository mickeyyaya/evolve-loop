package explanationdocs

import (
	"reflect"
	"strings"
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
		"- `go/acs/{cycle9,cycle1768}/predicates_test.go` — the predicate file, cited by brace list\n" +
		"- `go/acs/cycle1768/...` — the predicate package, cited in Go's package-tree form\n" +
		"- `go/internal/**` — the internal tree, cited recursively\n"

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

func TestChangedAreaFailures_AMalformedBraceListCoversNothing(t *testing.T) {
	material := "- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps under the 50-line limit\n"
	for name, citation := range map[string]string{
		"an empty alternative never collapses to a bare prefix":           "go/{fake,phantom,}",
		"a nested brace list, even one whose mis-parse names a diff path": "go/acs/{x,{cycle1768,y}}/predicates_test.go",
		"an unclosed brace": "go/acs/{cycle1768/predicates_test.go",
	} {
		section := material + "- `" + citation + "` — a citation whose brace list is malformed\n"

		got := changedAreaFailures(section, coverChanged, coverMaterial)

		want := []string{"Explanation Documentation: cited path " + normalize(citation) + " is not in the Build diff"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: failures = %v, want %v", name, got, want)
		}
	}
}

func TestChangedAreaFailures_AMultiGroupBraceListWithinTheBoundCovers(t *testing.T) {
	section := "" +
		"- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps under the 50-line limit\n" +
		"- `go/{x,acs}/{cycle9,cycle1768}/{helpers,predicates}_test.go` — both predicate files, cited by three brace lists\n"

	if got := changedAreaFailures(section, coverChanged, coverMaterial); len(got) != 0 {
		t.Errorf("failures = %v, want none", got)
	}
}

func TestExpandBraces_StopsPastTheBoundInsteadOfGrowingExponentially(t *testing.T) {
	t.Parallel()
	if got := expandBraces(strings.Repeat("{a,b}", 7)); got != nil {
		t.Errorf("expandBraces made %d patterns from 7 two-way groups; past the bound it must give up (a builder-authored line must not cost 2^N)", len(got))
	}
	if got := expandBraces(strings.Repeat("{a,b}", 6)); len(got) != 64 {
		t.Errorf("expandBraces made %d patterns from 6 two-way groups, want the 64 the bound allows", len(got))
	}
}

func TestChangedAreaFailures_AStringPrefixThatIsNotADirectoryCoversNothing(t *testing.T) {
	section := "" +
		"- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps under the 50-line limit\n" +
		"- `go/acs/cycle176` — a made-up package that is only a string prefix of cycle1768\n"

	got := changedAreaFailures(section, coverChanged, coverMaterial)

	want := []string{"Explanation Documentation: cited path go/acs/cycle176 is not in the Build diff"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("failures = %v, want %v", got, want)
	}
}

func TestChangedAreaFailures_ALiteralPathWithBracesIsCitedExactly(t *testing.T) {
	literal := "templates/{{cookiecutter.slug}}/app.py"
	section := "- `" + literal + "` — the template's entry point, a real path whose name holds braces\n"

	if got := changedAreaFailures(section, []string{literal}, []string{literal}); len(got) != 0 {
		t.Errorf("failures = %v, want none: a path that exists with braces in its name must stay citable exactly, or its material entry is impossible to write", got)
	}
}
