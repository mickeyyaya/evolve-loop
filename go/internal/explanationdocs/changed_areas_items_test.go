package explanationdocs

import (
	"reflect"
	"strings"
	"testing"
)

func TestChangedAreaFailures_AWrappedExplanationIsReadAcrossItsContinuationLines(t *testing.T) {
	for name, section := range map[string]string{
		"wrapped mid-sentence (cycle 1762)": "" +
			"- `go/internal/phasecoherence/coherence.go` — migrates\n" +
			"  `checkAll` into named steps under the 50-line limit.\n",
		"wrapped right after the separator": "" +
			"- `go/internal/phasecoherence/coherence.go` —\n" +
			"  splits checkAll into named steps under the 50-line limit.\n",
		"a colon, then sub-bullets": "" +
			"- `go/internal/phasecoherence/coherence.go`:\n" +
			"  - splits checkAll into named steps\n" +
			"  - keeps every branch's behavior identical\n",
		"a loose item whose explanation follows a blank line": "" +
			"- `go/internal/phasecoherence/coherence.go`:\n" +
			"\n" +
			"  splits checkAll into named steps under the 50-line limit.\n",
		"CRLF line endings": "" +
			"- `go/internal/phasecoherence/coherence.go` — migrates\r\n" +
			"  `checkAll` into named steps under the 50-line limit.\r\n",
		"an unindented continuation in an item that follows a blank line": "" +
			"- `go/acs/cycle1768/predicates_test.go` — pins the split's unchanged behavior\n" +
			"\n" +
			"- `go/internal/phasecoherence/coherence.go` — migrates\n" +
			"checkAll into named steps under the 50-line limit.\n",
	} {
		if got := changedAreaFailures(section, coverChanged, coverMaterial); len(got) != 0 {
			t.Errorf("%s: failures = %v, want none: a Markdown list item's explanation spans its continuation lines", name, got)
		}
	}
}

func TestChangedAreaFailures_AGroupedBulletExplainsEveryPathItNames(t *testing.T) {
	changed := []string{
		"go/internal/core/composition_carryforward_signal_test.go",
		"go/internal/core/scoped_merge_carryforward_signal_test.go",
		"go/cmd/evolve/cmd_composition_gates_tail_test.go",
	}
	for name, section := range map[string]string{
		"commas across lines (cycle 1772)": "" +
			"- `go/internal/core/composition_carryforward_signal_test.go`,\n" +
			"  `go/internal/core/scoped_merge_carryforward_signal_test.go`,\n" +
			"  `go/cmd/evolve/cmd_composition_gates_tail_test.go` — new tests (TDD-authored this\n" +
			"  cycle) pinning the coded-event and per-gate-tail contract.\n",
		"and across lines (cycle 1735)": "" +
			"- `go/internal/core/composition_carryforward_signal_test.go`, `go/cmd/evolve/cmd_composition_gates_tail_test.go` and\n" +
			"  `go/internal/core/scoped_merge_carryforward_signal_test.go` — the three tests pinning the coded event.\n",
		"a follower written with a leading ./": "" +
			"- `go/internal/core/composition_carryforward_signal_test.go`,\n" +
			"  `./go/cmd/evolve/cmd_composition_gates_tail_test.go`,\n" +
			"  `go/internal/core/scoped_merge_carryforward_signal_test.go` — the three tests pinning the coded event.\n",
		"one line, a serial comma and an ampersand": "" +
			"- `go/internal/core/composition_carryforward_signal_test.go`, `go/cmd/evolve/cmd_composition_gates_tail_test.go`, &" +
			" `go/internal/core/scoped_merge_carryforward_signal_test.go`: the three tests pinning the coded event.\n",
	} {
		if got := changedAreaFailures(section, changed, changed); len(got) != 0 {
			t.Errorf("%s: failures = %v, want none: every path a grouped item names shares its explanation", name, got)
		}
	}
}

func TestChangedAreaFailures_AGroupedItemStillNeedsARealExplanation(t *testing.T) {
	section := "- `go/internal/phasecoherence/coherence.go`, `go/acs/cycle1768/predicates_test.go` — fix\n"

	got := changedAreaFailures(section, coverChanged, coverMaterial)

	want := []string{
		"Explanation Documentation: Changed Areas path go/internal/phasecoherence/coherence.go needs a what/why explanation",
		"Explanation Documentation: Changed Areas path go/acs/cycle1768/predicates_test.go needs a what/why explanation",
		"Explanation Documentation: Changed Areas does not explain material path go/internal/phasecoherence/coherence.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("failures = %v, want %v: a group's explanation is the text after its last path, never the path list itself", got, want)
	}
}

func TestChangedAreaFailures_ABacktickedSpanNamingNoBuildContentIsProse(t *testing.T) {
	for name, section := range map[string]string{
		"an identifier, then prose with a later hyphen": "- `go/internal/phasecoherence/coherence.go`, `checkAll` now splits into named steps under the 50-line limit\n",
		"an identifier glued to a hyphen":               "- `go/internal/phasecoherence/coherence.go`, `helper`-based validation added — improves the loader\n",
		"the file, then its symbol":                     "- `go/internal/phasecoherence/coherence.go`, `checkAll` — splits it into named steps under the limit\n",
		"a path outside the diff":                       "- `go/internal/phasecoherence/coherence.go`, `go/internal/phantom/phantom.go` — splits checkAll into named steps\n",
	} {
		if got := changedAreaFailures(section, coverChanged, coverMaterial); len(got) != 0 {
			t.Errorf("%s: failures = %v, want none: a span joins a group only when it covers a diff path, so anything else stays the first path's explanation", name, got)
		}
	}
}

func TestChangedAreaFailures_AnItemEndsAtTheNextBulletOrAtUnindentedTextAfterABlankLine(t *testing.T) {
	want := []string{
		"Explanation Documentation: Changed Areas path go/internal/phasecoherence/coherence.go needs a what/why explanation",
		"Explanation Documentation: Changed Areas does not explain material path go/internal/phasecoherence/coherence.go",
	}
	for name, section := range map[string]string{
		"the next bullet": "" +
			"- `go/internal/phasecoherence/coherence.go` — fix\n" +
			"- Note: the rest of the package is untouched by this change.\n",
		"unindented text after a blank line": "" +
			"- `go/internal/phasecoherence/coherence.go` — fix\n" +
			"\n" +
			"A later paragraph, long enough to pass the floor on its own.\n",
		"a nested item's next sibling": "" +
			"- Core changes:\n" +
			"  - `go/internal/phasecoherence/coherence.go` — fix\n" +
			"  - also touches the helper, long enough to pass the floor\n",
	} {
		if got := changedAreaFailures(section, coverChanged, coverMaterial); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: failures = %v, want %v: a short explanation never borrows the next block's text", name, got, want)
		}
	}
}

func TestChangedAreaFailures_ProseOutsideAnyItemIsNotAnEntry(t *testing.T) {
	section := "" +
		"`go/internal/phantom/phantom.go` is untouched; it is named here only for contrast.\n" +
		"- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps\n"

	if got := changedAreaFailures(section, coverChanged, coverMaterial); len(got) != 0 {
		t.Errorf("failures = %v, want none: only a bullet beginning with a backticked path starts an entry", got)
	}
}

func TestChangedAreaFailures_AnUnclosedBacktickNamesNoPath(t *testing.T) {
	section := "- `go/internal/phasecoherence/coherence.go — splits checkAll into named steps\n"

	got := changedAreaFailures(section, coverChanged, coverMaterial)

	want := []string{"Explanation Documentation: Changed Areas does not explain material path go/internal/phasecoherence/coherence.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("failures = %v, want %v", got, want)
	}
}

func TestLeadingCodeSpan_ReadsOnlyASpanThatOpensTheString(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]struct {
		span, rest string
		ok         bool
	}{
		"`go/a.go` — why":       {"go/a.go", " — why", true},
		"— migrates `checkAll`": {"", "— migrates `checkAll`", false},
		"`go/a.go — why":        {"", "`go/a.go — why", false},
	} {
		span, rest, ok := leadingCodeSpan(input)
		if span != want.span || rest != want.rest || ok != want.ok {
			t.Errorf("leadingCodeSpan(%q) = (%q, %q, %v), want (%q, %q, %v)", input, span, rest, ok, want.span, want.rest, want.ok)
		}
	}
}

func TestChangedAreaFailures_NestedPathBulletsStayTheirOwnEntries(t *testing.T) {
	for name, section := range map[string]string{
		"under a heading bullet": "" +
			"- Core changes:\n" +
			"  - `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps\n",
		"under a path bullet": "" +
			"- `go/internal/phasecoherence/coherence.go` — splits checkAll into named steps\n" +
			"  - `go/acs/cycle1768/predicates_test.go` — pins the split's unchanged behavior\n",
	} {
		if got := changedAreaFailures(section, coverChanged, coverMaterial); len(got) != 0 {
			t.Errorf("%s: failures = %v, want none", name, got)
		}
	}
}

func TestValidateDocument_WrappedAndGroupedChangedAreasPassTheWholeDocument(t *testing.T) {
	base := strings.Repeat("a", 40)
	body := "# Build Explanation — Cycle 42\n\n" +
		"## Build Binding\n- Cycle: 42\n- Base SHA: " + base + "\n\n" +
		"## Summary\nSplits checkAll into named steps under the size limit.\n\n" +
		"## Rationale\nNamed steps keep each function under the limit without changing behavior.\n\n" +
		"## Changed Areas\n" +
		"- `go/internal/phasecoherence/coherence.go` — migrates\n" +
		"  `checkAll` into named steps under the 50-line limit.\n" +
		"- `go/acs/cycle1768/helpers_test.go`,\n" +
		"  `go/acs/cycle1768/predicates_test.go` — the cycle's predicates for the shrink.\n\n" +
		"## Design Decisions\nEach step keeps the original order of checks.\n\n" +
		"## Verification\nThe package tests and the cycle predicates pass unchanged.\n\n" +
		"## Compatibility\nNo exported behavior changes.\n\n" +
		"## Limitations\nOnly the one function is split.\n"

	if got := validateDocument(body, 42, base, coverChanged, coverMaterial); len(got) != 0 {
		t.Errorf("failures = %v, want none: wrapped and grouped bullets reach the item reader through the real section extraction", got)
	}
}
