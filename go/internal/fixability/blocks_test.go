package fixability_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
)

const (
	unifyItem     = "retro-minting-files-through-inbox-add-console"
	unifyDefect   = "M2: go/internal/inboxfile/inboxfile.go duplicates the filing rules; unification queued as " + unifyItem
	backstopShip  = "repo-contract importer backstop RED: failing: github.com/mickeyyaya/evolve-loop/go/internal/inboxfile.TestLockName, github.com/mickeyyaya/evolve-loop/go/internal/core.TestFiled"
	otherShipGate = "repo-contract importer backstop RED: failing: github.com/mickeyyaya/evolve-loop/go/internal/inboxfilex.TestOther"
)

func blockerIDs(d fixability.Decision) []string {
	out := []string{}
	for _, f := range d.Blockers {
		out = append(out, f.ID)
	}
	return out
}

func TestClassify_AGateFailureThatNamesTheCitedPathBlocks(t *testing.T) {
	d := fixability.Classify(fixability.Input{
		Defects:          []string{unifyDefect},
		GateFailures:     []string{backstopShip},
		OpenConsoleItems: []string{unifyItem},
	})
	if !d.Stop || !reflect.DeepEqual(blockerIDs(d), []string{"M2"}) {
		t.Fatalf("a ship gate that names go/internal/inboxfile did not block M2: %+v", d)
	}
}

func TestClassify_AGateFailureMustNameTheWholePathSegment(t *testing.T) {
	for name, defect := range map[string]string{
		"a longer package":   unifyDefect,
		"a parent directory": "M2: go/internal/inbox duplicates the filing rules; unification queued as " + unifyItem,
	} {
		d := fixability.Classify(fixability.Input{
			Defects:          []string{defect},
			GateFailures:     []string{otherShipGate},
			OpenConsoleItems: []string{unifyItem},
		})
		if d.Stop {
			t.Errorf("%s: a gate on go/internal/inboxfilex blocked the finding: %+v", name, d)
		}
	}
}

func TestClassify_AGateFailureThatNamesATestOfTheFindingBlocks(t *testing.T) {
	defect := "M4: TestLockName fails in the importer backstop; fix is console item " + unifyItem
	for gate, blocks := range map[string]bool{
		backstopShip: true,
		"repo-contract importer backstop RED: failing: example.com/x.TestLockNameTwo": false,
	} {
		d := fixability.Classify(fixability.Input{Defects: []string{defect}, GateFailures: []string{gate}, OpenConsoleItems: []string{unifyItem}})
		if d.Stop != blocks {
			t.Errorf("gate %q: stop = %t, want %t", gate, d.Stop, blocks)
		}
	}
}

func TestClassify_ARedTestNeedsTheFailVerbAfterTheTests(t *testing.T) {
	for text, blocks := range map[string]bool{
		"M1: floor tests fail (3) with this diff; fix is " + unifyItem:       true,
		"M1: floor tests (3) fail with this diff; fix is " + unifyItem:       true,
		"M1: the predicate failed; see " + unifyItem:                         true,
		"M2: add a regression test that fails before the fix; " + unifyItem:  false,
		"M2: the tests no longer fail after " + unifyItem:                    false,
		"M2: the tests pass and the predicates hold; queued as " + unifyItem: false,
	} {
		d := fixability.Classify(fixability.Input{Defects: []string{text}, OpenConsoleItems: []string{unifyItem}})
		if d.Stop != blocks {
			t.Errorf("%q: stop = %t, want %t", text, d.Stop, blocks)
		}
	}
}

func TestClassify_AConsoleItemIDMustBeWhole(t *testing.T) {
	for text, named := range map[string]bool{
		"M2: see " + unifyItem + "-two for the fix":      false,
		"M2: see " + unifyItem + "2 for the fix":         false,
		"M2: see " + unifyItem + "x for the fix":         false,
		"M2: see pre" + unifyItem + " for the fix":       false,
		"M2: see 9" + unifyItem + " for the fix":         false,
		"M2: see pre-" + unifyItem + " for the fix":      false,
		"M2: see " + unifyItem + "-two and " + unifyItem: true,
		"M2: queued as " + unifyItem + ".":               true,
		unifyItem:                                        true,
	} {
		d := fixability.Classify(fixability.Input{Defects: []string{text}, OpenConsoleItems: []string{unifyItem}})
		if got := len(d.Handoff) == 1; got != named {
			t.Errorf("%q: hand-off = %t, want %t (%+v)", text, got, named, d)
		}
	}
}

func TestClassify_AGatePathNeedsABoundaryOnBothSides(t *testing.T) {
	dirDefect := "M2: go/internal/inboxfile duplicates the filing rules; unification queued as " + unifyItem
	for gate, blocks := range map[string]bool{
		"failing: go/internal/inboxfile.TestX":       true,
		"failing: example.com/go/internal/inboxfile": true,
		"failing: xgo/internal/inboxfile":            false,
		"failing: -go/internal/inboxfile":            false,
		"failing: .go/internal/inboxfile":            false,
		"failing: _go/internal/inboxfile":            false,
		"failing: go/internal/inboxfile-two":         false,
		"failing: go/internal/inboxfile/sub":         false,
		"failing: go/internal/inboxfile_x":           false,
	} {
		d := fixability.Classify(fixability.Input{Defects: []string{dirDefect}, GateFailures: []string{gate}, OpenConsoleItems: []string{unifyItem}})
		if d.Stop != blocks {
			t.Errorf("gate %q: stop = %t, want %t", gate, d.Stop, blocks)
		}
	}
}

func TestClassify_AGateThatNamesTheCitedFileItselfBlocks(t *testing.T) {
	d := fixability.Classify(fixability.Input{
		Defects:          []string{unifyDefect},
		GateFailures:     []string{"gofmt: go/internal/inboxfile/inboxfile.go differs"},
		OpenConsoleItems: []string{unifyItem},
	})
	if !d.Stop {
		t.Fatalf("a gate that names the cited file did not block: %+v", d)
	}
}

func TestClassify_AFileAtTheRootHasNoPackageForTheGateRule(t *testing.T) {
	d := fixability.Classify(fixability.Input{
		Defects:          []string{"M2: ./main.go repeats the rules; queued as " + unifyItem},
		GateFailures:     []string{"vet . failed"},
		OpenConsoleItems: []string{unifyItem},
	})
	if d.Stop {
		t.Fatalf("the root directory of a file at the root matched a gate: %+v", d)
	}
}
