package advisor

// catalog_test.go — the SELECT menu (ADR-0103 unit 04 §6 test 32; the core
// on-demand index tests moved verbatim in intent; the ACS-named overflow
// tests stay in core over the facade).

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func overflowCards(n int) []router.PhaseCard {
	cards := make([]router.PhaseCard, n)
	for i := range cards {
		cards[i] = router.PhaseCard{Name: fmt.Sprintf("phase-%02d", i), Role: "evaluate", Optional: true, WhenToUse: fmt.Sprintf("when %d", i)}
	}
	return cards
}

// Test 32 — the enriched slots go to (Optional+metadata, Optional, rest) in
// stable order, overflow names never render individually, the pointer line
// appears only on overflow, the on-demand index only when something
// declined, WriteCatalog is the nil-index alias.
func TestWriteCatalog_EnrichesTwelveByStablePriorityAndPointsAtTheRest(t *testing.T) {
	var b strings.Builder
	WriteCatalogWithOnDemand(&b, richCatalog(), []string{"market-sizing", "okr-draft"})
	out := b.String()
	wantOrder := []string{"- bug-reproduction [", "- plan-review [", "- security-sweep [", "- perf-benchmark [", "- architecture-design [", "- triage [", "- memo [", "- retro [", "- doc-sync [", "- scout [", "- build [", "- audit ["}
	last := -1
	for _, w := range wantOrder {
		i := strings.Index(out, w)
		if i <= last {
			t.Errorf("%q at %d must follow %d (stable partition: metadata, optional, spine):\n%s", w, i, last, out)
		}
		last = i
	}
	if strings.Contains(out, "- ship [") {
		t.Errorf("the 13th card (ship) overflows and is never listed individually:\n%s", out)
	}
	if !strings.Contains(out, ".evolve/phase-inventory.json") || !strings.Contains(out, "\n2 further phase(s) are installed and available ON REQUEST — name one in your plan to use it: market-sizing, okr-draft\n") {
		t.Errorf("the pointer line and the on-demand index:\n%s", out)
	}
	if i := strings.Index(out, "ON REQUEST"); strings.Contains(out[:i], "market-sizing") {
		t.Error("a declined phase must not appear in the SELECT menu")
	}
	var twelve, alias strings.Builder
	WriteCatalogWithOnDemand(&twelve, overflowCards(MaxEnrichedCatalogCards), nil)
	if strings.Contains(twelve.String(), "phase-inventory.json") || strings.Contains(twelve.String(), "ON REQUEST") || strings.Count(twelve.String(), "- phase-") != 12 {
		t.Errorf("exactly the cap: every card enriched, no pointer, no index:\n%s", twelve.String())
	}
	WriteCatalog(&alias, overflowCards(MaxEnrichedCatalogCards))
	if alias.String() != twelve.String() {
		t.Error("WriteCatalog == WriteCatalogWithOnDemand(·, nil)")
	}
	var thirteen strings.Builder
	WriteCatalog(&thirteen, overflowCards(MaxEnrichedCatalogCards+1))
	if !strings.Contains(thirteen.String(), "phase-inventory.json") || strings.Contains(thirteen.String(), "- phase-12 [") {
		t.Errorf("one over the cap: the pointer, the overflow name absent:\n%s", thirteen.String())
	}
	var none strings.Builder
	WriteCatalogWithOnDemand(&none, nil, []string{"market-sizing"})
	if none.Len() != 0 {
		t.Errorf("an empty catalog renders nothing, not even the index: %q", none.String())
	}
	if MaxEnrichedCatalogCards != 12 {
		t.Errorf("MaxEnrichedCatalogCards = %d, want 12", MaxEnrichedCatalogCards)
	}
}

func TestWriteCard_WritesSourceCategoriesAndHintCap(t *testing.T) {
	var b strings.Builder
	writeCard(&b, router.PhaseCard{Name: "security-sweep", Role: "evaluate", WritesSource: true, Categories: []string{"security", "auth"},
		WhenToUse: strings.Repeat("w", 141), AllowedCLIs: []string{"claude-tmux", "codex-tmux"}, ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Default: "deep", Max: "top"}})
	want := "- security-sweep [evaluate, writes-source] (security, auth) — when: " + strings.Repeat("w", 140) + " …[truncated]\n  allowed_clis: claude-tmux, codex-tmux\n  model_tier_envelope: {min: balanced, default: deep, max: top}\n"
	if b.String() != want {
		t.Errorf("full card:\n got %q\nwant %q", b.String(), want)
	}
	b.Reset()
	writeCard(&b, router.PhaseCard{Name: "plan-review", Role: "evaluate", Description: "Reviews the plan."})
	if b.String() != "- plan-review [evaluate] — when: Reviews the plan.\n" {
		t.Errorf("the description is the hint fallback: %q", b.String())
	}
	b.Reset()
	writeCard(&b, router.PhaseCard{Name: "scout", Role: "plan"})
	if b.String() != "- scout [plan]\n" {
		t.Errorf("a metadata-less card degrades to the legacy form: %q", b.String())
	}
}

// The on-demand index names the declined phases compactly (moved from core).
func TestWriteCatalog_OnDemandPhasesAreStillIndexed(t *testing.T) {
	var b strings.Builder
	WriteCatalogWithOnDemand(&b, []router.PhaseCard{{Name: "scout", Optional: true}}, []string{"market-sizing", "okr-draft"})
	out := b.String()
	if !strings.Contains(out, "market-sizing") || !strings.Contains(out, "okr-draft") || !strings.Contains(out, "scout") {
		t.Fatalf("declined phases discoverable by name, the menu still rendered:\n%s", out)
	}
	if idx := out[strings.Index(out, "market-sizing"):]; strings.Count(idx, "\n") > 3 {
		t.Fatalf("the on-demand index must be compact, not a second catalog:\n%s", idx)
	}
}

func TestWriteCatalog_NoIndexLineWhenNothingDeclined(t *testing.T) {
	var b strings.Builder
	WriteCatalogWithOnDemand(&b, []router.PhaseCard{{Name: "scout", Optional: true}}, nil)
	if strings.Contains(b.String(), "ON REQUEST") {
		t.Fatalf("no declined phases means no index line; got:\n%s", b.String())
	}
}
