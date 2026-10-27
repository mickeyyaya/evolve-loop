package fixability_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	laneItem      = "retro-minting-files-through-inbox-add-lane"
	acFiveConsole = "retro-minting-files-through-inbox-add-lane-ac-5-console"
)

func lifecycleFence(p string) bool {
	return strings.HasPrefix(p, "go/internal/guards/")
}

func TestCompanions_AConsoleItemThatExtendsTheLaneIDIsACompanion(t *testing.T) {
	got := fixability.Companions(laneItem, []fixability.Item{{ID: acFiveConsole}, {ID: "unrelated-console"}})
	if !reflect.DeepEqual(got, []string{acFiveConsole}) {
		t.Errorf("companions = %v, want only %s", got, acFiveConsole)
	}
}

func TestCompanions_TheSharedStemWithoutLaneIsACompanion(t *testing.T) {
	if got := fixability.Companions(laneItem, []fixability.Item{{ID: unifyItem}}); !reflect.DeepEqual(got, []string{unifyItem}) {
		t.Errorf("companions = %v, want %s", got, unifyItem)
	}
}

func TestCompanions_AConsoleItemThatNamesTheLaneItemInDepsOrConnectsToIsACompanion(t *testing.T) {
	open := []fixability.Item{{ID: "fix-a", Deps: []string{laneItem}}, {ID: "fix-b", ConnectsTo: []string{laneItem}}, {ID: "fix-c", ConnectsTo: []string{laneItem + "-two"}}}
	if got := fixability.Companions(laneItem, open); !reflect.DeepEqual(got, []string{"fix-a", "fix-b"}) {
		t.Errorf("companions = %v, want fix-a and fix-b", got)
	}
}

func TestCompanions_AnItemWithoutTheLaneSuffixHasNoStemRuleAndIsNotItsOwnCompanion(t *testing.T) {
	if got := fixability.Companions("retro-minting", []fixability.Item{{ID: "retro-minting"}, {ID: "retro-console"}}); len(got) != 0 {
		t.Errorf("companions = %v, want none", got)
	}
	if got := fixability.Companions(laneItem, []fixability.Item{{ID: laneItem}}); len(got) != 0 {
		t.Errorf("the lane item is its own companion: %v", got)
	}
	if fixability.ReasonConsoleCompanionPending != "console_companion_pending" {
		t.Errorf("reason = %q", fixability.ReasonConsoleCompanionPending)
	}
}

func TestOpenConsole_KeepsTheRoutedAndTheProtectedItemsAndNotANameAlone(t *testing.T) {
	items := []inboxbatch.Item{
		{ID: "a", Route: "console-manual"},
		{ID: "b", Files: []string{"go/internal/guards/x.go"}},
		{ID: acFiveConsole, Route: "lane", ConnectsTo: []string{"n"}, Deps: []string{"d"}, Files: []string{"go/internal/guards/floor_test.go"}},
		{ID: "named-console", Route: "lane"},
		{ID: laneItem, Route: "lane", Files: []string{"go/internal/faillearn/inbox.go"}},
	}

	got := fixability.OpenConsole(items, lifecycleFence)

	want := []fixability.Item{{ID: "a"}, {ID: "b"}, {ID: acFiveConsole, ConnectsTo: []string{"n"}, Deps: []string{"d"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OpenConsole = %+v, want %+v", got, want)
	}
}
