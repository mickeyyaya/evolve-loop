package fixability

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const laneItem = "retro-minting-files-through-inbox-add-lane"

func TestCompanions_AConsoleItemThatExtendsTheLaneIDIsACompanion(t *testing.T) {
	t.Parallel()
	got := Companions(laneItem, []Item{{ID: acFiveConsole}, {ID: "unrelated-console"}})
	if !slices.Equal(got, []string{acFiveConsole}) {
		t.Errorf("companions = %v, want only %s", got, acFiveConsole)
	}
}

func TestCompanions_TheSharedStemWithoutLaneIsACompanion(t *testing.T) {
	t.Parallel()
	if got := Companions(laneItem, []Item{{ID: mergeConsole}}); !slices.Equal(got, []string{mergeConsole}) {
		t.Errorf("companions = %v, want %s", got, mergeConsole)
	}
}

func TestCompanions_AConsoleItemThatNamesTheLaneItemInDepsOrConnectsToIsACompanion(t *testing.T) {
	t.Parallel()
	open := []Item{{ID: "fix-a", Deps: []string{laneItem}}, {ID: "fix-b", ConnectsTo: []string{laneItem}}, {ID: "fix-c", ConnectsTo: []string{laneItem + "-two"}}}
	if got := Companions(laneItem, open); !slices.Equal(got, []string{"fix-a", "fix-b"}) {
		t.Errorf("companions = %v, want fix-a and fix-b", got)
	}
}

func TestCompanions_AnItemWithoutTheLaneSuffixHasNoStemRuleAndIsNotItsOwnCompanion(t *testing.T) {
	t.Parallel()
	if got := Companions("retro-minting", []Item{{ID: "retro-minting"}, {ID: "retro-console"}}); len(got) != 0 {
		t.Errorf("companions = %v, want none", got)
	}
}

func TestOpenConsole_KeepsTheRoutedAndTheProtectedItemsAndNotANameAlone(t *testing.T) {
	t.Parallel()
	items := []inboxbatch.Item{
		{ID: "a", Route: "console-manual"},
		{ID: "b", Files: []string{"go/internal/guards/x.go"}},
		{ID: acFiveConsole, Route: "lane", ConnectsTo: []string{"n"}, Deps: []string{"d"}, Files: []string{"go/internal/guards/floor_test.go"}},
		{ID: "named-console", Route: "lane"},
		{ID: laneItem, Route: "lane", Files: []string{"go/internal/faillearn/inbox.go"}},
	}

	got := OpenConsole(items, testFence)

	want := []Item{{ID: "a"}, {ID: "b"}, {ID: acFiveConsole, ConnectsTo: []string{"n"}, Deps: []string{"d"}}}
	if len(got) != 3 || got[0].ID != want[0].ID || got[1].ID != want[1].ID || got[2].ID != want[2].ID || !slices.Equal(got[2].ConnectsTo, want[2].ConnectsTo) || !slices.Equal(got[2].Deps, want[2].Deps) {
		t.Errorf("OpenConsole = %+v, want %+v", got, want)
	}
}
