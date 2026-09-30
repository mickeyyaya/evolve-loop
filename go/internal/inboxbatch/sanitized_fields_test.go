package inboxbatch

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSanitizedFields_NamesEveryFieldTheLoaderWouldRewrite(t *testing.T) {
	item := Item{
		ID:         "clean-id",
		Title:      "a title with a \a bell",
		Files:      []string{"go/ok.go", strings.Repeat("x", maxFieldLen+1)},
		Acceptance: []string{"fine"},
	}

	got := SanitizedFields(item)

	if !slices.Equal(got, []string{"title", "files"}) {
		t.Errorf("SanitizedFields = %v, want [title files]", got)
	}
	if item.Title != "a title with a \a bell" || len(item.Files[1]) != maxFieldLen+1 {
		t.Errorf("SanitizedFields must not rewrite its argument: %+v", item)
	}
	if clean := SanitizedFields(Item{ID: "x", Title: "t", Acceptance: []string{"a"}}); clean != nil {
		t.Errorf("a clean item: SanitizedFields = %v, want nil", clean)
	}
}

func TestSanitizedFields_NamesEachFieldOnceAndNeverRewritesItsArgument(t *testing.T) {
	item := Item{Files: []string{"a\x01", "b\x01"}, Acceptance: []string{"c\x01"}}

	got := SanitizedFields(item)

	if !slices.Equal(got, []string{"files", "acceptance"}) {
		t.Errorf("SanitizedFields = %v, want [files acceptance]", got)
	}
	if item.Files[0] != "a\x01" || item.Acceptance[0] != "c\x01" {
		t.Errorf("SanitizedFields rewrote its argument: %+v", item)
	}
}

func TestIsConsoleRoute_IsTheRuleConsoleRoutedApplies(t *testing.T) {
	for route, want := range map[string]bool{"console": true, " Console-Manual ": true, "console-sandbox-denied": true, "lane": false, "": false} {
		if got := IsConsoleRoute(route); got != want {
			t.Errorf("IsConsoleRoute(%q) = %v, want %v", route, got, want)
		}
		if routed, _ := ConsoleRouted(Item{ID: "x", Route: route}, nil); want && !routed {
			t.Errorf("ConsoleRouted with route %q must route to the console", route)
		}
	}
}

func TestFilenameStampLayout_IsTheStampFiledAtReads(t *testing.T) {
	stamp := time.Date(2026, 9, 30, 9, 30, 0, 0, time.UTC)

	it := Item{Path: stamp.Format(FilenameStampLayout) + "-x.json"}

	if got := it.FiledAt(); !got.Equal(stamp) {
		t.Errorf("FiledAt = %v, want %v", got, stamp)
	}
}
