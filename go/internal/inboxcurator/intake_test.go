package inboxcurator

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func goodItem() inboxbatch.Item {
	return inboxbatch.Item{ID: "a", Kind: "feature", Origin: inboxbatch.OriginConsole, Files: []string{"go/internal/fleet/pool.go"}}
}

func TestCheckIntake_AdmitsAnItemWithAKnownOriginKindAndADeclaredSurface(t *testing.T) {
	if err := CheckIntake(goodItem(), []string{"bug", "feature"}); err != nil {
		t.Fatalf("CheckIntake = %v, want nil", err)
	}
}

func TestCheckIntake_RefusesAnUnknownOriginKindOrSurface(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*inboxbatch.Item)
		why  string
	}{
		"no origin":          {func(it *inboxbatch.Item) { it.Origin = "" }, "origin"},
		"an unknown origin":  {func(it *inboxbatch.Item) { it.Origin = "lane" }, "origin"},
		"an unknown kind":    {func(it *inboxbatch.Item) { it.Kind = "loop-reliability" }, `kind "loop-reliability"`},
		"no files":           {func(it *inboxbatch.Item) { it.Files = nil }, "files"},
		"no path in files":   {func(it *inboxbatch.Item) { it.Files = []string{"the pool"} }, "files"},
		"an absolute path":   {func(it *inboxbatch.Item) { it.Files = append(it.Files, "/Users/x/go/a.go") }, "/Users/x/go/a.go"},
		"a parent reference": {func(it *inboxbatch.Item) { it.Files = []string{"go/../../etc/passwd"} }, "go/../../etc/passwd"},
	} {
		it := goodItem()
		tc.edit(&it)
		err := CheckIntake(it, []string{"bug", "feature"})
		if !errors.Is(err, ErrIntake) || !strings.Contains(err.Error(), tc.why) {
			t.Errorf("%s: err = %v, want ErrIntake naming %q", name, err, tc.why)
		}
	}
}

func TestUnderivedConsoleRoute_FlagsAConsoleRouteThatNoFactExplains(t *testing.T) {
	gate := func(p string) bool { return p == "go/internal/guards/ship.go" }
	for name, tc := range map[string]struct {
		edit func(*inboxbatch.Item)
		want bool
	}{
		"no route":                           {func(*inboxbatch.Item) {}, false},
		"a console route with no reason":     {func(it *inboxbatch.Item) { it.Route = "console-manual" }, true},
		"a console route on a gate file":     {func(it *inboxbatch.Item) { it.Route = "console"; it.Files = []string{"go/internal/guards/ship.go"} }, false},
		"a console route on a pipeline kind": {func(it *inboxbatch.Item) { it.Route = "console"; it.Kind = "pipeline-repair" }, false},
	} {
		it := goodItem()
		tc.edit(&it)
		if got := UnderivedConsoleRoute(it, gate); got != tc.want {
			t.Errorf("%s: UnderivedConsoleRoute = %v, want %v", name, got, tc.want)
		}
	}
}

func admitFixture() (inboxbatch.Item, []inboxbatch.Item) {
	files := []string{"go/internal/fleet/pool.go"}
	candidate := inboxbatch.Item{ID: "new", Kind: "feature", Origin: inboxbatch.OriginConsole, Files: files, Title: "a free lane takes the next task"}
	open := []inboxbatch.Item{
		{ID: "dup", Files: files, Title: "a free lane takes the next task"},
		{ID: "near", Files: files, Title: "a free lane metric"},
	}
	return candidate, open
}

func TestAdmit_RefusesAnOverlapUnlessAllowedWithAReason(t *testing.T) {
	candidate, open := admitFixture()
	rules := Rules{Kinds: []string{"feature"}, Thresholds: Thresholds{Refuse: 0.6, Warn: 0.2}}

	_, err := Admit(candidate, open, rules, Override{})

	var overlap *OverlapError
	if !errors.As(err, &overlap) || !errors.Is(err, ErrIntake) || overlapIDs(overlap.Refused) == nil || overlapIDs(overlap.Refused)[0] != "dup" {
		t.Fatalf("Admit = %v, want an OverlapError naming dup", err)
	}
	got, err := Admit(candidate, open, rules, Override{Reason: "two queues", AllowOverlap: true})
	if err != nil || got.Record == nil || got.Record.Reason != "two queues" || len(got.Record.Overlaps) != 1 || got.Record.Overlaps[0] != "dup" {
		t.Fatalf("Admit with the override = %+v, %v; want a record of the allowed overlap", got, err)
	}
	if ids := overlapIDs(got.Warned); len(ids) != 1 || ids[0] != "near" {
		t.Errorf("Warned = %v, want [near]", ids)
	}
}

func TestAdmit_AnUnexplainedConsoleRouteNeedsAReason(t *testing.T) {
	candidate, _ := admitFixture()
	candidate.Route = "console-manual"
	rules := Rules{Kinds: []string{"feature"}, Thresholds: Thresholds{Refuse: 0.6, Warn: 0.2}}

	if _, err := Admit(candidate, nil, rules, Override{}); !errors.Is(err, ErrUnexplainedRoute) || !errors.Is(err, ErrIntake) {
		t.Fatalf("Admit = %v, want ErrUnexplainedRoute", err)
	}
	got, err := Admit(candidate, nil, rules, Override{Reason: "an operator decision"})
	if err != nil || got.Record == nil || !got.Record.ConsoleRoute {
		t.Fatalf("Admit with a reason = %+v, %v; want a record of the console route", got, err)
	}
}

func TestAdmit_AClearItemHasNoRecordAndABadOneIsRefused(t *testing.T) {
	candidate, _ := admitFixture()
	rules := Rules{Kinds: []string{"feature"}, Thresholds: Thresholds{Refuse: 0.6, Warn: 0.2}}

	var got Admission
	got, err := Admit(candidate, nil, rules, Override{Reason: "unused"})
	if err != nil || got.Record != nil {
		t.Errorf("Admit = %+v, %v; want no record when nothing is overridden", got, err)
	}
	candidate.Kind = "wish"
	if _, err := Admit(candidate, nil, rules, Override{}); !errors.Is(err, ErrIntake) {
		t.Errorf("Admit of an unknown kind = %v, want ErrIntake", err)
	}
}

func TestOverlapError_NamesTheOpenItemsAndTheThreshold(t *testing.T) {
	err := &OverlapError{Refused: []Overlap{{ID: "a"}, {ID: "b"}}, Threshold: 0.5}

	if msg := err.Error(); !strings.Contains(msg, "a, b") || !strings.Contains(msg, "0.50") || !strings.Contains(msg, ErrIntake.Error()) {
		t.Errorf("Error() = %q, want the ids, the threshold and the intake refusal", msg)
	}
	if ids := err.IDs(); !slices.Equal(ids, []string{"a", "b"}) || err.Unwrap() != ErrIntake {
		t.Errorf("IDs() = %v, Unwrap() = %v; want [a b] and ErrIntake", ids, err.Unwrap())
	}
}
