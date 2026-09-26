package advisor

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// MaxEnrichedCatalogCards bounds how many catalog cards render with full metadata.
const MaxEnrichedCatalogCards = 12

const maxCardHintRunes = 140

// WriteCatalog renders the SELECT menu of pre-defined phases with no on-demand index.
func WriteCatalog(b *strings.Builder, cards []router.PhaseCard) {
	WriteCatalogWithOnDemand(b, cards, nil)
}

// WriteCatalogWithOnDemand renders the SELECT menu, then names the on-demand phases in one line.
// The index hides a declined phase without removing it: a phase absent from the prompt cannot be selected.
func WriteCatalogWithOnDemand(b *strings.Builder, cards []router.PhaseCard, onDemand []string) {
	if len(cards) == 0 {
		return
	}
	b.WriteString("\n## Pre-defined phases you may SELECT (prefer these over minting)\n")
	b.WriteString("Each already exists with a tuned persona + profile. SELECT one by naming it in your plan ")
	b.WriteString("(no \"mint\" block). Only MINT a new phase when none of these fit the work.\n")

	enriched, overflow := cards, []router.PhaseCard(nil)
	if len(cards) > MaxEnrichedCatalogCards {
		ordered := partitionCards(cards)
		enriched, overflow = ordered[:MaxEnrichedCatalogCards], ordered[MaxEnrichedCatalogCards:]
	}
	for _, c := range enriched {
		writeCard(b, c)
	}
	if len(overflow) > 0 {
		b.WriteString("- All remaining selectable phases are in the \"## Phase Catalog — Core Values\" table (this prompt) and .evolve/phase-inventory.json (same SELECT rules; type/domain details there).\n")
	}
	if len(onDemand) > 0 {
		fmt.Fprintf(b, "\n%d further phase(s) are installed and available ON REQUEST — name one in your plan to use it: %s\n",
			len(onDemand), strings.Join(onDemand, ", "))
	}

}

// partitionCards puts spine cards last: they run via the mandatory config however they render.
func partitionCards(cards []router.PhaseCard) []router.PhaseCard {
	var withMeta, opt, rest []router.PhaseCard
	for _, c := range cards {
		switch {
		case c.Optional && (c.WhenToUse != "" || c.Description != "" || len(c.Categories) > 0):
			withMeta = append(withMeta, c)
		case c.Optional:
			opt = append(opt, c)
		default:
			rest = append(rest, c)
		}
	}
	ordered := append(withMeta, opt...)
	return append(ordered, rest...)
}

func writeCard(b *strings.Builder, c router.PhaseCard) {
	ws := ""
	if c.WritesSource {
		ws = ", writes-source"
	}
	fmt.Fprintf(b, "- %s [%s%s]", c.Name, c.Role, ws)
	if len(c.Categories) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(c.Categories, ", "))
	}
	hint := c.WhenToUse
	if hint == "" {
		hint = c.Description
	}
	if hint = textcap.TruncateRunes(hint, maxCardHintRunes); hint != "" {
		fmt.Fprintf(b, " — when: %s", hint)
	}
	b.WriteString("\n")
	// The phase's own guardrails give an advisor proposing {cli,tier} the legal bounds.
	if len(c.AllowedCLIs) > 0 {
		fmt.Fprintf(b, "  allowed_clis: %s\n", strings.Join(c.AllowedCLIs, ", "))
	}
	if env := c.ModelTierEnvelope; env != nil {
		fmt.Fprintf(b, "  model_tier_envelope: {min: %s, default: %s, max: %s}\n", env.Min, env.Default, env.Max)
	}
}
