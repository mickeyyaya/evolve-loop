package advisor

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func routingContext(in router.RouteInput) string {
	var b strings.Builder
	WriteRoutingContext(&b, in)
	return b.String()
}

func TestWriteRoutingContext_RendersTheLaneItemTheCycleIsPinnedTo(t *testing.T) {
	got := routingContext(router.RouteInput{
		GoalText: "wave goal about pipeline health",
		LaneItems: []router.LaneItem{{
			ID: "netflix-margin-device-experience", Kind: "strategy", DeliverableKind: "document",
			Acceptance: []string{"at least two options", "routing plan justifies tdd"},
		}},
	})
	for _, want := range []string{
		"## Lane scope",
		"netflix-margin-device-experience",
		"kind: strategy",
		"deliverable_kind: document",
		"1. at least two options",
		"2. routing plan justifies tdd",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("routing context lacks %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "## Goal") > strings.Index(got, "## Lane scope") {
		t.Errorf("the lane scope must follow the goal so the stable goal prefix stays first:\n%s", got)
	}
}

func TestWriteRoutingContext_AnUnresolvedLaneItemSaysWhy(t *testing.T) {
	got := routingContext(router.RouteInput{LaneItems: []router.LaneItem{{ID: "ghost-item", Unresolved: "inbox record not resolved"}}})
	if !strings.Contains(got, "ghost-item") || !strings.Contains(got, "inbox record not resolved") {
		t.Errorf("an unresolved lane item must render its id and the reason:\n%s", got)
	}
}

func TestWriteRoutingContext_ALaneSectionLongerThanTheLaneCapIsCapped(t *testing.T) {
	got := routingContext(router.RouteInput{LaneItems: []router.LaneItem{{
		ID: "oversized-item", Kind: "strategy", DeliverableKind: "document",
		Acceptance: []string{strings.Repeat("a", maxLaneScopeRunes)},
	}}})
	start, end := strings.Index(got, "## Lane scope"), strings.Index(got, "\n\n## Objective signals")
	if start < 0 || end < start {
		t.Fatalf("routing context lacks a lane section followed by the objective signals:\n%s", got)
	}
	const marker = " …[truncated]"
	section := got[start:end]
	if !strings.HasSuffix(section, marker) || utf8.RuneCountInString(section) != maxLaneScopeRunes+utf8.RuneCountInString(marker) {
		t.Errorf("the lane section must be capped at %d runes plus the truncation marker; got %d runes ending %q",
			maxLaneScopeRunes, utf8.RuneCountInString(section), section[len(section)-min(len(section), 40):])
	}
}

func TestWriteRoutingContext_NoLaneItemsRendersNoLaneSection(t *testing.T) {
	if got := routingContext(router.RouteInput{GoalText: "goal"}); strings.Contains(got, "Lane scope") {
		t.Errorf("a run without a lane pin must render no lane section:\n%s", got)
	}
}

func TestWriteRubricLines_ProjectsADeclaredSkipWhen(t *testing.T) {
	var b strings.Builder
	writeRubricLines(&b, config.RoutingConfig{Triggers: map[string]config.RoutingBlock{
		"fault-localization": {SkipWhen: []config.Condition{
			{Field: "cycle_size", Op: "eq", Value: "trivial"},
			{Field: "deliverable_kind", Op: "eq", Value: "document"},
		}},
	}})
	if want := "- cycle_size == trivial OR deliverable_kind == document → skip fault-localization"; !strings.Contains(b.String(), want) {
		t.Errorf("rubric lacks %q:\n%s", want, b.String())
	}
}
