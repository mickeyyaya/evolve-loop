package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// makeOverflowCards builds n cards that force the overflow path when n > maxEnrichedCatalogCards.
// All cards are Optional with metadata so they sort into the withMeta bucket.
func makeOverflowCards(n int) []router.PhaseCard {
	cards := make([]router.PhaseCard, n)
	for i := range cards {
		cards[i] = router.PhaseCard{
			Name:        fmt.Sprintf("phase-%02d", i),
			Role:        "evaluate",
			Optional:    true,
			Description: "a phase",
			WhenToUse:   "cycle work",
		}
	}
	return cards
}

func TestWriteCatalog_OverflowNotEnumerated(t *testing.T) {
	t.Parallel()
	cards := makeOverflowCards(maxEnrichedCatalogCards + 5)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()
	if strings.Contains(out, "- also available (") {
		idx := strings.Index(out, "- also available (")
		end := strings.Index(out[idx:], "\n")
		if end < 0 {
			end = len(out[idx:])
		}
		t.Errorf("RED: writeCatalog emitted a bare overflow enumeration.\n"+
			"Line: %q\n"+
			"Builder must replace '- also available (<names>)' with a one-line pointer to phase-inventory.json.", out[idx:idx+end])
	}
}

func TestWriteCatalog_PointerLinePresent(t *testing.T) {
	t.Parallel()
	cards := makeOverflowCards(maxEnrichedCatalogCards + 5)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()
	for i := maxEnrichedCatalogCards; i < len(cards); i++ {
		name := cards[i].Name
		if strings.Contains(out, name) {
			t.Errorf("RED: overflow card %q individually named in output.\n"+
				"Builder must replace the overflow name list with a one-line pointer.\n"+
				"Pointer preserves selectability without enumerating every overflow name.", name)
		}
	}
	if !strings.Contains(out, "phase-inventory.json") {
		t.Errorf("RED (secondary): pointer line missing reference to phase-inventory.json.\n"+
			"Builder must include a lookup reference (phase-inventory.json) so router can SELECT overflow phases.\n"+
			"Current output: %q", truncate(out, 400))
	}
}

func TestWriteCatalog_PointerLineRequired_Negative(t *testing.T) {
	t.Parallel()
	// Minimal overflow: exactly 1 card beyond the cap.
	cards := makeOverflowCards(maxEnrichedCatalogCards + 1)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()

	overflowName := cards[maxEnrichedCatalogCards].Name

	if strings.Contains(out, overflowName) {
		t.Errorf("Negative: with 1 overflow card, %q is individually listed in output.\n"+
			"Even minimal overflow must use a pointer, not an enumeration.", overflowName)
	}

	// A builder who removes the enumeration but adds no pointer would produce
	// output no longer than the no-overflow case; catch that here.
	noOverflowCards := makeOverflowCards(maxEnrichedCatalogCards)
	var noB strings.Builder
	writeCatalog(&noB, noOverflowCards)
	if len(out) <= len(noB.String()) {
		t.Errorf("Negative (anti-gaming): output with 1 overflow card (%d bytes) is not longer "+
			"than output with no overflow (%d bytes).\n"+
			"Builder must add a pointer line — do not just remove the enumeration silently.",
			len(out), len(noB.String()))
	}
}

func TestWriteCatalog_NoOverflow_NoPointer(t *testing.T) {
	t.Parallel()
	// Exactly at the cap — none overflow.
	cards := makeOverflowCards(maxEnrichedCatalogCards)
	var b strings.Builder
	writeCatalog(&b, cards)
	out := b.String()
	if strings.Contains(out, "also available") {
		t.Errorf("Edge: writeCatalog emitted overflow text when %d cards fit within cap %d.\n"+
			"No overflow → no enumeration and no pointer.", len(cards), maxEnrichedCatalogCards)
	}
	if strings.Contains(out, "phase-inventory.json") {
		t.Errorf("Edge: writeCatalog emitted a pointer when no overflow exists (%d cards ≤ cap %d).\n"+
			"Pointer must only appear when len(cards) > maxEnrichedCatalogCards.", len(cards), maxEnrichedCatalogCards)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
