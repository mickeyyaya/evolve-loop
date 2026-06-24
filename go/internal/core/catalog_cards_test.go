package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestPhaseCardsFromCatalog(t *testing.T) {
	t.Parallel()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "scout"},                     // → plan
		{Name: "build", WritesSource: true}, // → build
		{Name: "audit"},                     // → evaluate
		{Name: "ship"},                      // → control (omitted)
		{Name: "retro"},                     // → control (omitted)
	})
	if err != nil {
		t.Fatal(err)
	}
	cards := phaseCardsFromCatalog(cat)

	byName := map[string]router.PhaseCard{}
	for _, c := range cards {
		byName[c.Name] = c
	}
	if _, ok := byName["ship"]; ok {
		t.Error("control-archetype phase 'ship' must be omitted from the advisor catalog")
	}
	if _, ok := byName["retro"]; ok {
		t.Error("control-archetype phase 'retro' must be omitted")
	}
	if byName["scout"].Role != "plan" {
		t.Errorf("scout role = %q, want plan", byName["scout"].Role)
	}
	if byName["build"].Role != "build" || !byName["build"].WritesSource {
		t.Errorf("build card = %+v, want build/writes-source", byName["build"])
	}
	if byName["audit"].Role != "evaluate" {
		t.Errorf("audit role = %q, want evaluate", byName["audit"].Role)
	}
	if len(cards) != 3 {
		t.Errorf("expected 3 composable cards (control omitted), got %d", len(cards))
	}
}

func TestPhaseCardsFromCatalog_CarriesMetadata(t *testing.T) {
	t.Parallel()
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{
			Name: "reproduce-bug", Optional: true, Role: "evaluate", WritesSource: true,
			Description: "Failing repro test before any patch.",
			WhenToUse:   "bugfix cycles, before tdd/build.",
			Categories:  []string{"bugfix"},
		},
	})
	cards := phaseCardsFromCatalog(cat)
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	c := cards[0]
	if !c.Optional || c.WhenToUse == "" || c.Description == "" || len(c.Categories) != 1 {
		t.Errorf("metadata not projected: %+v", c)
	}
}

func TestWriteCatalog_EnrichedCardLine(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	writeCatalog(&b, []router.PhaseCard{
		{Name: "reproduce-bug", Role: "evaluate", Optional: true, WritesSource: true,
			WhenToUse: "bugfix cycles, before tdd/build", Categories: []string{"bugfix"}},
	})
	out := b.String()
	for _, want := range []string{"reproduce-bug [evaluate, writes-source]", "(bugfix)", "when: bugfix cycles, before tdd/build"} {
		if !strings.Contains(out, want) {
			t.Errorf("enriched card missing %q; got:\n%s", want, out)
		}
	}
}

func TestWriteCatalog_TruncatesLongWhenToUse(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	long := strings.Repeat("x", 400)
	writeCatalog(&b, []router.PhaseCard{
		{Name: "wordy", Role: "evaluate", Optional: true, WhenToUse: long},
	})
	out := b.String()
	if strings.Contains(out, long) {
		t.Error("when_to_use must be truncated in the card line")
	}
	if !strings.Contains(out, "…") {
		t.Errorf("truncation must be marked; got:\n%s", out)
	}
}

func TestWriteCatalog_CapsEnrichedCardsKeepsAllSelectable(t *testing.T) {
	t.Parallel()
	// 14 optional cards + 2 spine cards: enriched rendering must cap, but every
	// name must still appear (a phase absent from the prompt cannot be SELECTed).
	var cards []router.PhaseCard
	cards = append(cards,
		router.PhaseCard{Name: "build", Role: "build"},
		router.PhaseCard{Name: "audit", Role: "evaluate"},
	)
	names := []string{"p-a", "p-b", "p-c", "p-d", "p-e", "p-f", "p-g", "p-h", "p-i", "p-j", "p-k", "p-l", "p-m", "p-n"}
	for _, n := range names {
		cards = append(cards, router.PhaseCard{Name: n, Role: "evaluate", Optional: true, WhenToUse: "use " + n})
	}

	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()

	enriched := strings.Count(out, "— when:")
	if enriched > 12 {
		t.Errorf("at most 12 enriched cards, got %d", enriched)
	}
	for _, c := range cards {
		if !strings.Contains(out, c.Name) {
			t.Errorf("every phase must remain selectable; %q missing:\n%s", c.Name, out)
		}
	}
	// Optional (SELECTable) cards get the enriched slots ahead of spine cards.
	if strings.Contains(out, "build — when:") {
		t.Error("non-optional spine cards must not consume enriched slots")
	}
	if !strings.Contains(out, "also available") {
		t.Errorf("overflow names must be listed under 'also available'; got:\n%s", out)
	}
}

func TestWriteCatalog(t *testing.T) {
	t.Parallel()
	t.Run("empty catalog renders nothing", func(t *testing.T) {
		var b strings.Builder
		writeCatalog(&b, nil)
		if b.Len() != 0 {
			t.Errorf("empty catalog must render nothing, got %q", b.String())
		}
	})
	t.Run("cards render under a SELECT heading with role + writes-source", func(t *testing.T) {
		var b strings.Builder
		writeCatalog(&b, []router.PhaseCard{
			{Name: "scout", Role: "plan"},
			{Name: "build", Role: "build", WritesSource: true},
		})
		out := b.String()
		for _, want := range []string{"prefer these over minting", "scout [plan]", "build [build, writes-source]"} {
			if !strings.Contains(out, want) {
				t.Errorf("catalog output missing %q; got:\n%s", want, out)
			}
		}
	})
}
